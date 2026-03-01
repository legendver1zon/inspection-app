package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GetInspections — список осмотров
func GetInspections(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	var inspections []models.Inspection
	query := storage.DB.Preload("User").Order("created_at desc")

	// Инспектор видит только свои осмотры
	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	}

	query.Find(&inspections)

	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "list.html", gin.H{
		"title":       "Мои осмотры",
		"inspections": inspections,
		"user":        user,
		"isAdmin":     role == "admin",
	})
}

// GetNewInspection — форма нового осмотра
func GetNewInspection(c *gin.Context) {
	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	// Считаем следующий номер акта
	var count int64
	storage.DB.Model(&models.Inspection{}).Count(&count)
	actNumber := strconv.FormatInt(count+1, 10)

	c.HTML(http.StatusOK, "new.html", gin.H{
		"title":     "Новый осмотр",
		"user":      user,
		"actNumber": actNumber,
		"today":     time.Now().Format("02.01.2006"),
	})
}

// PostInspection — создание нового осмотра
func PostInspection(c *gin.Context) {
	userID := c.GetUint("userID")

	actNumber := c.PostForm("act_number")
	address := c.PostForm("address")
	inspTime := c.PostForm("inspection_time")
	ownerName := c.PostForm("owner_name")
	devRepName := c.PostForm("developer_rep_name")

	roomsCount, _ := strconv.Atoi(c.PostForm("rooms_count"))
	floor, _ := strconv.Atoi(c.PostForm("floor"))
	totalArea, _ := strconv.ParseFloat(c.PostForm("total_area"), 64)
	tempOut, _ := strconv.ParseFloat(c.PostForm("temp_outside"), 64)
	tempIn, _ := strconv.ParseFloat(c.PostForm("temp_inside"), 64)
	humidity, _ := strconv.ParseFloat(c.PostForm("humidity"), 64)

	inspection := models.Inspection{
		ActNumber:        actNumber,
		UserID:           userID,
		Date:             time.Now(),
		InspectionTime:   inspTime,
		Address:          address,
		RoomsCount:       roomsCount,
		Floor:            floor,
		TotalArea:        totalArea,
		TempOutside:      tempOut,
		TempInside:       tempIn,
		Humidity:         humidity,
		OwnerName:        ownerName,
		DeveloperRepName: devRepName,
		Status:           "draft",
	}

	if err := storage.DB.Create(&inspection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания осмотра"})
		return
	}

	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10)+"/edit")
}

// GetInspection — просмотр осмотра
func GetInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	var documents []models.Document
	storage.DB.Where("inspection_id = ?", inspection.ID).Find(&documents)

	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "view.html", gin.H{
		"title":       "Акт №" + inspection.ActNumber,
		"inspection":  inspection,
		"documents":   documents,
		"user":        user,
		"isAdmin":     c.GetString("userRole") == "admin",
	})
}

// GetEditInspection — форма редактирования осмотра (дефекты)
func GetEditInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	// Загружаем все шаблоны дефектов
	var templates []models.DefectTemplate
	storage.DB.Order("section, order_index").Find(&templates)

	// Загружаем уже заполненные дефекты
	var filled []models.InspectionDefect
	storage.DB.Preload("DefectTemplate").
		Where("inspection_id = ?", inspection.ID).
		Find(&filled)

	// Загружаем комнаты
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", inspection.ID).
		Order("room_number").Find(&rooms)

	// Если комнат нет — создаём пустые 10 строк
	if len(rooms) == 0 {
		for i := 1; i <= 10; i++ {
			rooms = append(rooms, models.InspectionRoom{
				InspectionID: inspection.ID,
				RoomNumber:   i,
			})
		}
	}

	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "edit.html", gin.H{
		"title":      "Редактировать акт №" + inspection.ActNumber,
		"inspection": inspection,
		"templates":  templates,
		"filled":     filled,
		"rooms":      rooms,
		"user":       user,
		"isAdmin":    c.GetString("userRole") == "admin",
	})
}

// PostEditInspection — сохранение дефектов осмотра
func PostEditInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	// Удаляем старые дефекты и перезаписываем
	storage.DB.Where("inspection_id = ?", inspection.ID).Delete(&models.InspectionDefect{})
	storage.DB.Where("inspection_id = ?", inspection.ID).Delete(&models.InspectionRoom{})

	// Сохраняем дефекты
	form, _ := c.MultipartForm()
	if form != nil {
		values := form.Value

		var templates []models.DefectTemplate
		storage.DB.Find(&templates)

		for _, tmpl := range templates {
			key := "defect_" + strconv.FormatUint(uint64(tmpl.ID), 10)
			if tmpl.Section == "wall_paint" {
				// Для стен — 4 поля
				for wall := 1; wall <= 4; wall++ {
					wallKey := key + "_wall" + strconv.Itoa(wall)
					if vals, ok := values[wallKey]; ok && vals[0] != "" {
						defect := models.InspectionDefect{
							InspectionID:     inspection.ID,
							DefectTemplateID: tmpl.ID,
							Value:            vals[0],
							WallNumber:       wall,
						}
						storage.DB.Create(&defect)
					}
				}
			} else {
				if vals, ok := values[key]; ok && vals[0] != "" {
					defect := models.InspectionDefect{
						InspectionID:     inspection.ID,
						DefectTemplateID: tmpl.ID,
						Value:            vals[0],
					}
					storage.DB.Create(&defect)
				}
			}
		}

		// Сохраняем "Прочее" по секциям
		for _, section := range []string{"window", "ceiling", "wall_paint"} {
			notesKey := "notes_" + section
			if vals, ok := values[notesKey]; ok && vals[0] != "" {
				defect := models.InspectionDefect{
					InspectionID:     inspection.ID,
					DefectTemplateID: 0,
					Notes:            vals[0],
					Value:            section + "_notes",
				}
				storage.DB.Create(&defect)
			}
		}

		// Сохраняем замеры помещений
		for i := 1; i <= 10; i++ {
			iStr := strconv.Itoa(i)
			roomName := ""
			if vals, ok := values["room_name_"+iStr]; ok {
				roomName = vals[0]
			}
			length, _ := strconv.ParseFloat(getFormValue(values, "room_length_"+iStr), 64)
			width, _ := strconv.ParseFloat(getFormValue(values, "room_width_"+iStr), 64)
			height, _ := strconv.ParseFloat(getFormValue(values, "room_height_"+iStr), 64)
			w1h, _ := strconv.ParseFloat(getFormValue(values, "window1_height_"+iStr), 64)
			w1w, _ := strconv.ParseFloat(getFormValue(values, "window1_width_"+iStr), 64)
			w2h, _ := strconv.ParseFloat(getFormValue(values, "window2_height_"+iStr), 64)
			w2w, _ := strconv.ParseFloat(getFormValue(values, "window2_width_"+iStr), 64)
			dh, _ := strconv.ParseFloat(getFormValue(values, "door_height_"+iStr), 64)
			dw, _ := strconv.ParseFloat(getFormValue(values, "door_width_"+iStr), 64)

			// Сохраняем только непустые строки
			if roomName != "" || length != 0 || width != 0 || height != 0 {
				room := models.InspectionRoom{
					InspectionID:  inspection.ID,
					RoomNumber:    i,
					RoomName:      roomName,
					Length:        length,
					Width:         width,
					Height:        height,
					Window1Height: w1h,
					Window1Width:  w1w,
					Window2Height: w2h,
					Window2Width:  w2w,
					DoorHeight:    dh,
					DoorWidth:     dw,
				}
				storage.DB.Create(&room)
			}
		}
	}

	// Обновляем статус
	status := c.PostForm("status")
	if status == "" {
		status = "draft"
	}
	storage.DB.Model(&inspection).Update("status", status)

	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10))
}

// PostUploadPlan — загрузка фото плана помещений
func PostUploadPlan(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	file, err := c.FormFile("plan_image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не найден"})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := "plan_" + strconv.FormatUint(uint64(inspection.ID), 10) + ext
	savePath := "web/static/uploads/" + filename

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения файла"})
		return
	}

	storage.DB.Model(&inspection).Update("plan_image", "/static/uploads/"+filename)
	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10)+"/edit")
}

// loadInspection — вспомогательная функция загрузки осмотра с проверкой доступа
func loadInspection(c *gin.Context) (*models.Inspection, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return nil, false
	}

	var inspection models.Inspection
	if err := storage.DB.Preload("User").Preload("Rooms").
		Preload("Defects.DefectTemplate").
		First(&inspection, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return nil, false
	}

	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return nil, false
	}

	return &inspection, true
}

func getFormValue(values map[string][]string, key string) string {
	if vals, ok := values[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}
