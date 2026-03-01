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
	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	}
	query.Find(&inspections)

	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "list.html", gin.H{
		"title":       "Осмотры",
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

	var count int64
	storage.DB.Model(&models.Inspection{}).Count(&count)

	c.HTML(http.StatusOK, "new.html", gin.H{
		"title":     "Новый осмотр",
		"user":      user,
		"actNumber": strconv.FormatInt(count+1, 10),
		"today":     time.Now().Format("02.01.2006"),
		"isAdmin":   c.GetString("userRole") == "admin",
	})
}

// PostInspection — создание нового осмотра
func PostInspection(c *gin.Context) {
	userID := c.GetUint("userID")

	roomsCount, _ := strconv.Atoi(c.PostForm("rooms_count"))
	floor, _ := strconv.Atoi(c.PostForm("floor"))
	totalArea, _ := strconv.ParseFloat(c.PostForm("total_area"), 64)
	tempOut, _ := strconv.ParseFloat(c.PostForm("temp_outside"), 64)
	tempIn, _ := strconv.ParseFloat(c.PostForm("temp_inside"), 64)
	humidity, _ := strconv.ParseFloat(c.PostForm("humidity"), 64)

	inspection := models.Inspection{
		ActNumber:        c.PostForm("act_number"),
		UserID:           userID,
		Date:             time.Now(),
		InspectionTime:   c.PostForm("inspection_time"),
		Address:          c.PostForm("address"),
		RoomsCount:       roomsCount,
		Floor:            floor,
		TotalArea:        totalArea,
		TempOutside:      tempOut,
		TempInside:       tempIn,
		Humidity:         humidity,
		OwnerName:        c.PostForm("owner_name"),
		DeveloperRepName: c.PostForm("developer_rep_name"),
		Status:           "draft",
	}

	if err := storage.DB.Create(&inspection).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания"})
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
		"title":      "Акт №" + inspection.ActNumber,
		"inspection": inspection,
		"documents":  documents,
		"user":       user,
		"isAdmin":    c.GetString("userRole") == "admin",
	})
}

// GetEditInspection — форма редактирования (дефекты по помещениям)
func GetEditInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	var rooms []models.InspectionRoom
	storage.DB.Preload("Defects.DefectTemplate").
		Where("inspection_id = ?", inspection.ID).
		Order("room_number").
		Find(&rooms)

	roomMap := make(map[int]*models.InspectionRoom)
	for i := range rooms {
		roomMap[rooms[i].RoomNumber] = &rooms[i]
	}

	activeRooms := len(rooms)
	if activeRooms < 3 {
		activeRooms = 3
	}

	templates := loadTemplatesBySection()

	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "edit.html", gin.H{
		"title":              "Редактировать акт №" + inspection.ActNumber,
		"inspection":         inspection,
		"user":               user,
		"isAdmin":            c.GetString("userRole") == "admin",
		"roomNums":           makeRange(1, 10),
		"wallNums":           []int{1, 2, 3, 4},
		"roomMap":            roomMap,
		"activeRooms":        activeRooms,
		"templates_window":   templates["window"],
		"templates_ceiling":  templates["ceiling"],
		"templates_wall":     templates["wall"],
		"templates_floor":    templates["floor"],
		"templates_door":     templates["door"],
		"templates_plumbing": templates["plumbing"],
	})
}

// PostEditInspection — сохранение дефектов по помещениям
func PostEditInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	c.Request.ParseMultipartForm(32 << 20)

	activeRooms, _ := strconv.Atoi(c.PostForm("active_rooms"))
	if activeRooms < 1 {
		activeRooms = 3
	}

	// Удаляем старые данные
	var oldRooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", inspection.ID).Find(&oldRooms)
	for _, r := range oldRooms {
		storage.DB.Where("room_id = ?", r.ID).Delete(&models.RoomDefect{})
	}
	storage.DB.Where("inspection_id = ?", inspection.ID).Delete(&models.InspectionRoom{})

	var allTemplates []models.DefectTemplate
	storage.DB.Order("section, order_index").Find(&allTemplates)

	simpleSections := []string{"window", "ceiling", "floor", "door", "plumbing"}

	for i := 1; i <= activeRooms; i++ {
		iStr := strconv.Itoa(i)

		length, _ := strconv.ParseFloat(c.PostForm("room_length_"+iStr), 64)
		width, _ := strconv.ParseFloat(c.PostForm("room_width_"+iStr), 64)
		height, _ := strconv.ParseFloat(c.PostForm("room_height_"+iStr), 64)
		w1h, _ := strconv.ParseFloat(c.PostForm("room_w1h_"+iStr), 64)
		w1w, _ := strconv.ParseFloat(c.PostForm("room_w1w_"+iStr), 64)
		w2h, _ := strconv.ParseFloat(c.PostForm("room_w2h_"+iStr), 64)
		w2w, _ := strconv.ParseFloat(c.PostForm("room_w2w_"+iStr), 64)
		dh, _ := strconv.ParseFloat(c.PostForm("room_dh_"+iStr), 64)
		dw, _ := strconv.ParseFloat(c.PostForm("room_dw_"+iStr), 64)

		room := models.InspectionRoom{
			InspectionID:  inspection.ID,
			RoomNumber:    i,
			RoomName:      c.PostForm("room_name_" + iStr),
			Length:        length,
			Width:         width,
			Height:        height,
			Window1Height: w1h,
			Window1Width:  w1w,
			Window2Height: w2h,
			Window2Width:  w2w,
			DoorHeight:    dh,
			DoorWidth:     dw,
			WindowType:    c.PostForm("room_window_type_" + iStr),
			WallType:      c.PostForm("room_wall_type_" + iStr),
		}

		if err := storage.DB.Create(&room).Error; err != nil {
			continue
		}

		// Простые секции (одно значение на дефект)
		for _, tmpl := range allTemplates {
			if !containsStr(simpleSections, tmpl.Section) {
				continue
			}
			key := "defect_" + strconv.FormatUint(uint64(tmpl.ID), 10) + "_" + iStr
			if val := c.PostForm(key); val != "" {
				storage.DB.Create(&models.RoomDefect{
					RoomID:           room.ID,
					DefectTemplateID: tmpl.ID,
					Section:          tmpl.Section,
					Value:            val,
				})
			}
		}

		// Стены — 4 значения на дефект
		for _, tmpl := range allTemplates {
			if tmpl.Section != "wall" {
				continue
			}
			for w := 1; w <= 4; w++ {
				key := "defect_" + strconv.FormatUint(uint64(tmpl.ID), 10) + "_" + iStr + "_wall" + strconv.Itoa(w)
				if val := c.PostForm(key); val != "" {
					storage.DB.Create(&models.RoomDefect{
						RoomID:           room.ID,
						DefectTemplateID: tmpl.ID,
						Section:          "wall",
						Value:            val,
						WallNumber:       w,
					})
				}
			}
		}

		// Прочее для каждой секции
		for _, sec := range append(simpleSections, "wall") {
			if notes := c.PostForm("notes_" + sec + "_" + iStr); notes != "" {
				storage.DB.Create(&models.RoomDefect{
					RoomID:  room.ID,
					Section: sec,
					Notes:   notes,
				})
			}
		}
	}

	status := c.PostForm("status")
	if status == "" {
		status = "draft"
	}
	storage.DB.Model(inspection).Update("status", status)

	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10))
}

// PostUploadPlan — загрузка фото плана
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
	if err := c.SaveUploadedFile(file, "web/static/uploads/"+filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
		return
	}

	storage.DB.Model(inspection).Update("plan_image", "/static/uploads/"+filename)
	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10)+"/edit")
}

func loadInspection(c *gin.Context) (*models.Inspection, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return nil, false
	}

	var inspection models.Inspection
	if err := storage.DB.Preload("User").Preload("Rooms.Defects.DefectTemplate").
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

func loadTemplatesBySection() map[string][]models.DefectTemplate {
	var all []models.DefectTemplate
	storage.DB.Order("section, order_index").Find(&all)
	result := make(map[string][]models.DefectTemplate)
	for _, t := range all {
		result[t.Section] = append(result[t.Section], t)
	}
	return result
}

func makeRange(min, max int) []int {
	r := make([]int, max-min+1)
	for i := range r {
		r[i] = min + i
	}
	return r
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
