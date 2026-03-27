package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const pageSize = 20

// GetInspections — список осмотров
func GetInspections(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	tab := c.DefaultQuery("tab", "draft")
	if tab != "draft" && tab != "completed" {
		tab = "draft"
	}

	// Параметры поиска
	ownerFilter := strings.TrimSpace(c.Query("owner"))
	inspectorFilter := strings.TrimSpace(c.Query("inspector"))
	addressFilter := strings.TrimSpace(c.Query("address"))
	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to"))

	// Счётчики по статусам
	var draftCount, completedCount int64
	draftQ := storage.DB.Model(&models.Inspection{}).Where("status = ?", "draft")
	completedQ := storage.DB.Model(&models.Inspection{}).Where("status = ?", "completed")
	listQ := storage.DB.Model(&models.Inspection{}).Preload("User").Where("status = ?", tab).Order("created_at desc")

	if role != "admin" {
		draftQ = draftQ.Where("user_id = ?", userID)
		completedQ = completedQ.Where("user_id = ?", userID)
		listQ = listQ.Where("user_id = ?", userID)
	}

	// Фильтр по фамилии собственника
	if ownerFilter != "" {
		like := "%" + ownerFilter + "%"
		draftQ = draftQ.Where("owner_name LIKE ?", like)
		completedQ = completedQ.Where("owner_name LIKE ?", like)
		listQ = listQ.Where("owner_name LIKE ?", like)
	}

	// Фильтр по фамилии инспектора (подзапрос по таблице users)
	if inspectorFilter != "" {
		sub := storage.DB.Table("users").Select("id").Where("full_name LIKE ?", "%"+inspectorFilter+"%")
		draftQ = draftQ.Where("user_id IN (?)", sub)
		completedQ = completedQ.Where("user_id IN (?)", sub)
		listQ = listQ.Where("user_id IN (?)", sub)
	}

	// Фильтр по адресу
	if addressFilter != "" {
		like := "%" + addressFilter + "%"
		draftQ = draftQ.Where("address LIKE ?", like)
		completedQ = completedQ.Where("address LIKE ?", like)
		listQ = listQ.Where("address LIKE ?", like)
	}

	// Фильтр по дате
	if dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			draftQ = draftQ.Where("date >= ?", t)
			completedQ = completedQ.Where("date >= ?", t)
			listQ = listQ.Where("date >= ?", t)
		}
	}
	if dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			end := t.Add(24*time.Hour - time.Nanosecond)
			draftQ = draftQ.Where("date <= ?", end)
			completedQ = completedQ.Where("date <= ?", end)
			listQ = listQ.Where("date <= ?", end)
		}
	}

	draftQ.Count(&draftCount)
	completedQ.Count(&completedCount)

	// Пагинация — клонируем запрос чтобы Count не затронул Preload в listQ
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	var totalCount int64
	listQ.Session(&gorm.Session{}).Count(&totalCount)
	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	offset := (page - 1) * pageSize

	var inspections []models.Inspection
	listQ.Limit(pageSize).Offset(offset).Find(&inspections)

	var user models.User
	storage.DB.First(&user, userID)

	hasFilters := ownerFilter != "" || inspectorFilter != "" || addressFilter != "" || dateFrom != "" || dateTo != ""

	// Базовый URL для ссылок пагинации (все текущие фильтры без page)
	qp := url.Values{}
	qp.Set("tab", tab)
	if ownerFilter != "" {
		qp.Set("owner", ownerFilter)
	}
	if inspectorFilter != "" {
		qp.Set("inspector", inspectorFilter)
	}
	if addressFilter != "" {
		qp.Set("address", addressFilter)
	}
	if dateFrom != "" {
		qp.Set("date_from", dateFrom)
	}
	if dateTo != "" {
		qp.Set("date_to", dateTo)
	}
	pageBase := "/inspections?" + qp.Encode()

	c.HTML(http.StatusOK, "list.html", gin.H{
		"title":           "Осмотры",
		"inspections":     inspections,
		"user":            user,
		"isAdmin":         role == "admin",
		"tab":             tab,
		"draftCount":      draftCount,
		"completedCount":  completedCount,
		"filterOwner":     ownerFilter,
		"filterInspector": inspectorFilter,
		"filterAddress":   addressFilter,
		"filterDateFrom":  dateFrom,
		"filterDateTo":    dateTo,
		"hasFilters":      hasFilters,
		"page":            page,
		"totalPages":      totalPages,
		"totalCount":      totalCount,
		"pageBase":        pageBase,
		"prevPage":        page - 1,
		"nextPage":        page + 1,
	})
}

// GetNewInspection — сразу создаёт пустой осмотр и редиректит на редактирование
func GetNewInspection(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	if allowed, msg := security.CheckInspectionLimit(userID, role); !allowed {
		security.Log(security.EventInspectionBlocked, c.ClientIP(), "userID="+strconv.Itoa(int(userID)))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	var count int64
	storage.DB.Model(&models.Inspection{}).Count(&count)

	inspection := models.Inspection{
		ActNumber: strconv.FormatInt(count+1, 10),
		UserID:    userID,
		Date:      time.Now(),
		Status:    "draft",
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
		w3h, _ := strconv.ParseFloat(c.PostForm("room_w3h_"+iStr), 64)
		w3w, _ := strconv.ParseFloat(c.PostForm("room_w3w_"+iStr), 64)
		w4h, _ := strconv.ParseFloat(c.PostForm("room_w4h_"+iStr), 64)
		w4w, _ := strconv.ParseFloat(c.PostForm("room_w4w_"+iStr), 64)
		w5h, _ := strconv.ParseFloat(c.PostForm("room_w5h_"+iStr), 64)
		w5w, _ := strconv.ParseFloat(c.PostForm("room_w5w_"+iStr), 64)
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
			Window3Height: w3h,
			Window3Width:  w3w,
			Window4Height: w4h,
			Window4Width:  w4w,
			Window5Height: w5h,
			Window5Width:  w5w,
			DoorHeight:    dh,
			DoorWidth:     dw,
			WindowType:    c.PostForm("room_window_type_" + iStr),
			WallType:      buildWallType(c, iStr),
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
				tid := tmpl.ID
				storage.DB.Create(&models.RoomDefect{
					RoomID:           room.ID,
					DefectTemplateID: &tid,
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
					tid := tmpl.ID
					storage.DB.Create(&models.RoomDefect{
						RoomID:           room.ID,
						DefectTemplateID: &tid,
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

	// Обновляем поля шапки акта
	roomsCount, _ := strconv.Atoi(c.PostForm("rooms_count"))
	floor, _ := strconv.Atoi(c.PostForm("floor"))
	totalArea, _ := strconv.ParseFloat(c.PostForm("total_area"), 64)
	tempOut, _ := strconv.ParseFloat(c.PostForm("temp_outside"), 64)
	tempIn, _ := strconv.ParseFloat(c.PostForm("temp_inside"), 64)
	humidity, _ := strconv.ParseFloat(c.PostForm("humidity"), 64)

	storage.DB.Model(inspection).Updates(map[string]interface{}{
		"status":             status,
		"act_number":         c.PostForm("act_number"),
		"inspection_time":    c.PostForm("inspection_time"),
		"address":            c.PostForm("address"),
		"rooms_count":        roomsCount,
		"floor":              floor,
		"total_area":         totalArea,
		"temp_outside":       tempOut,
		"temp_inside":        tempIn,
		"humidity":           humidity,
		"owner_name":         c.PostForm("owner_name"),
		"developer_rep_name": c.PostForm("developer_rep_name"),
	})

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

	if err := security.ValidateImage(file, security.MaxPlanSize); err != nil {
		security.Log(security.EventFileRejected, c.ClientIP(), "plan: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "План: " + err.Error()})
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

// GetUploadStatus — GET /inspections/:id/upload-status
// Возвращает JSON со статусом загрузки фото в облако.
func GetUploadStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}

	var inspection models.Inspection
	if err := storage.DB.First(&inspection, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return
	}

	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	storage.DB.Model(&models.Photo{}).
		Select("photos.upload_status as status, COUNT(*) as count").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ?", id).
		Group("photos.upload_status").
		Scan(&rows)

	counts := map[string]int64{"pending": 0, "uploading": 0, "done": 0, "failed": 0}
	var total int64
	for _, r := range rows {
		counts[r.Status] = r.Count
		total += r.Count
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"pending":   counts["pending"],
		"uploading": counts["uploading"],
		"done":      counts["done"],
		"failed":    counts["failed"],
		"all_done":  counts["pending"] == 0 && counts["uploading"] == 0 && counts["failed"] == 0,
	})
}

// PostDeleteInspection — удаление акта осмотра (только admin)
func PostDeleteInspection(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}

	var inspection models.Inspection
	if err := storage.DB.First(&inspection, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return
	}

	// Удаляем дефекты всех помещений
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", id).Find(&rooms)
	for _, r := range rooms {
		storage.DB.Where("room_id = ?", r.ID).Delete(&models.RoomDefect{})
	}
	storage.DB.Where("inspection_id = ?", id).Delete(&models.InspectionRoom{})

	// Удаляем файлы документов и записи
	var docs []models.Document
	storage.DB.Where("inspection_id = ?", id).Find(&docs)
	for _, doc := range docs {
		if doc.FilePath != "" {
			os.Remove(doc.FilePath)
		}
	}
	storage.DB.Where("inspection_id = ?", id).Delete(&models.Document{})

	// Удаляем сам осмотр
	storage.DB.Delete(&inspection)

	c.Redirect(http.StatusFound, "/inspections")
}

func loadInspection(c *gin.Context) (*models.Inspection, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return nil, false
	}

	var inspection models.Inspection
	if err := storage.DB.Preload("User").Preload("Rooms.Defects.DefectTemplate").Preload("Rooms.Defects.Photos").
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

func buildWallType(c *gin.Context, iStr string) string {
	var types []string
	if c.PostForm("room_wall_type_paint_"+iStr) != "" {
		types = append(types, "paint")
	}
	if c.PostForm("room_wall_type_tile_"+iStr) != "" {
		types = append(types, "tile")
	}
	if c.PostForm("room_wall_type_gkl_"+iStr) != "" {
		types = append(types, "gkl")
	}
	return strings.Join(types, ",")
}
