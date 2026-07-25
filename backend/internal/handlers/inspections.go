package handlers

import (
	"errors"
	"fmt"
	"inspection-app/internal/logger"
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

// errActNumberTaken — sentinel-ошибка, которую транзакция возвращает при
// обнаружении гонки по уникальному индексу act_number. Handler превращает её
// в человеко-читаемый редирект, чтобы не потерять данные инспектора.
var errActNumberTaken = errors.New("act_number already taken (race)")

// redirectWithError — редирект на страницу редактирования с сообщением об ошибке.
// Используется, когда валидация до транзакции не прошла — данные в БД не трогаем.
func redirectWithError(c *gin.Context, inspectionID uint, msg string) {
	editURL := "/inspections/" + strconv.FormatUint(uint64(inspectionID), 10) +
		"/edit?error=" + url.QueryEscape(msg)
	c.Redirect(http.StatusFound, editURL)
}

// isActNumberConflict возвращает true, если ошибка GORM — это нарушение
// уникального индекса по act_number (PostgreSQL SQLSTATE 23505).
// Строковая проверка выбрана, чтобы не тянуть pgx/pgconn как прямую зависимость.
func isActNumberConflict(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "23505") && strings.Contains(s, "act_number")
}

const pageSize = 20

// GetDashboard — страница статистики
func GetDashboard(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	user := CurrentUser(c)

	base := storage.DB.Model(&models.Inspection{})
	if role != "admin" {
		base = base.Where("user_id = ?", userID)
	}

	var draftCount, completedCount, totalCount int64
	base.Session(&gorm.Session{}).Count(&totalCount)
	base.Session(&gorm.Session{}).Where("status = ?", "draft").Count(&draftCount)
	base.Session(&gorm.Session{}).Where("status = ?", "completed").Count(&completedCount)

	// Создано сегодня
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	base.Session(&gorm.Session{}).Where("created_at >= ?", today).Count(&todayCount)

	// Создано за последние 7 дней
	weekAgo := today.AddDate(0, 0, -7)
	var weekCount int64
	base.Session(&gorm.Session{}).Where("created_at >= ?", weekAgo).Count(&weekCount)

	// Фото: pending/failed
	var photoPending, photoFailed int64
	storage.DB.Model(&models.Photo{}).Where("upload_status = ?", "pending").Count(&photoPending)
	storage.DB.Model(&models.Photo{}).Where("upload_status = ?", "failed").Count(&photoFailed)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":          "Статистика",
		"user":           user,
		"isAdmin":        role == "admin",
		"totalCount":     totalCount,
		"draftCount":     draftCount,
		"completedCount": completedCount,
		"todayCount":     todayCount,
		"weekCount":      weekCount,
		"photoPending":   photoPending,
		"photoFailed":    photoFailed,
	})
}

// GetInspections — список осмотров: черновики карточками, завершённые таблицей.
// Обе группы рендерятся на одной странице; tab задаёт активную вкладку на мобильном.
func GetInspections(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	tab := c.DefaultQuery("tab", "draft")
	if tab != "draft" && tab != "completed" {
		tab = "draft"
	}

	// Параметры поиска: q — совмещённый (номер/адрес/собственник),
	// остальные — расширенные фильтры
	q := strings.TrimSpace(c.Query("q"))
	actFilter := strings.TrimSpace(c.Query("act_number"))
	ownerFilter := strings.TrimSpace(c.Query("owner"))
	inspectorFilter := strings.TrimSpace(c.Query("inspector"))
	addressFilter := strings.TrimSpace(c.Query("address"))
	dateFrom := strings.TrimSpace(c.Query("date_from"))
	dateTo := strings.TrimSpace(c.Query("date_to"))

	// buildQ — базовый запрос с ролью и всеми фильтрами
	buildQ := func() *gorm.DB {
		db := storage.DB.Model(&models.Inspection{})
		if role != "admin" {
			db = db.Where("user_id = ?", userID)
		}
		if q != "" {
			like := "%" + escapeLike(q) + "%"
			db = db.Where("act_number LIKE ? OR address LIKE ? OR owner_name LIKE ?", like, like, like)
		}
		if actFilter != "" {
			db = db.Where("act_number LIKE ?", "%"+escapeLike(actFilter)+"%")
		}
		if ownerFilter != "" {
			db = db.Where("owner_name LIKE ?", "%"+escapeLike(ownerFilter)+"%")
		}
		if inspectorFilter != "" {
			sub := storage.DB.Table("users").Select("id").Where("full_name LIKE ?", "%"+escapeLike(inspectorFilter)+"%")
			db = db.Where("user_id IN (?)", sub)
		}
		if addressFilter != "" {
			db = db.Where("address LIKE ?", "%"+escapeLike(addressFilter)+"%")
		}
		if dateFrom != "" {
			if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
				db = db.Where("date >= ?", t)
			}
		}
		if dateTo != "" {
			if t, err := time.Parse("2006-01-02", dateTo); err == nil {
				db = db.Where("date <= ?", t.Add(24*time.Hour-time.Nanosecond))
			}
		}
		return db
	}

	var draftCount, completedCount int64
	buildQ().Where("status = ?", "draft").Count(&draftCount)
	buildQ().Where("status = ?", "completed").Count(&completedCount)

	// Черновики — карточками, без пагинации (ограничение — защитный предел)
	var drafts []models.Inspection
	buildQ().Preload("User").Where("status = ?", "draft").
		Order("created_at desc").Limit(60).Find(&drafts)

	// Завершённые — таблицей с пагинацией, новые сверху
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	totalPages := int((completedCount + int64(pageSize) - 1) / int64(pageSize))
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	var completed []models.Inspection
	buildQ().Preload("User").Where("status = ?", "completed").
		Order("date desc, id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&completed)

	user := CurrentUser(c)
	hasFilters := actFilter != "" || ownerFilter != "" || inspectorFilter != "" || addressFilter != "" || dateFrom != "" || dateTo != ""

	// Базовый URL для ссылок пагинации (все текущие фильтры без page)
	qp := url.Values{}
	qp.Set("tab", tab)
	for k, v := range map[string]string{
		"q": q, "act_number": actFilter, "owner": ownerFilter,
		"inspector": inspectorFilter, "address": addressFilter,
		"date_from": dateFrom, "date_to": dateTo,
	} {
		if v != "" {
			qp.Set(k, v)
		}
	}
	pageBase := "/inspections?" + qp.Encode()

	c.HTML(http.StatusOK, "list.html", gin.H{
		"title":           "Осмотры",
		"draftCards":      buildActCards(drafts),
		"doneCards":       buildActCards(completed),
		"user":            user,
		"isAdmin":         role == "admin",
		"tab":             tab,
		"draftCount":      draftCount,
		"completedCount":  completedCount,
		"filterQ":         q,
		"filterActNumber": actFilter,
		"filterOwner":     ownerFilter,
		"filterInspector": inspectorFilter,
		"filterAddress":   addressFilter,
		"filterDateFrom":  dateFrom,
		"filterDateTo":    dateTo,
		"hasFilters":      hasFilters,
		"hasAnyFilter":    hasFilters || q != "",
		"page":            page,
		"totalPages":      totalPages,
		"pageBase":        pageBase,
		"prevPage":        page - 1,
		"nextPage":        page + 1,
	})
}

// GetNewInspection — сразу создаёт пустой осмотр и редиректит на редактирование.
// Номер акта формируется из ID записи (гарантированно уникален, без race condition).
func GetNewInspection(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	if allowed, msg := security.CheckInspectionLimit(userID, role); !allowed {
		security.Log(security.EventInspectionBlocked, c.ClientIP(), "userID="+strconv.Itoa(int(userID)))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	inspection := models.Inspection{
		ActNumber: "-", // placeholder, обновляется ниже внутри транзакции
		UserID:    userID,
		Date:      time.Now(),
		Status:    "draft",
	}

	err := storage.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&inspection).Error; err != nil {
			return err
		}
		actNumber := strconv.FormatUint(uint64(inspection.ID), 10) + "-" + time.Now().Format("020106")
		return tx.Model(&inspection).Update("act_number", actNumber).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания"})
		return
	}

	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10)+"/edit")
}

// ArchivedDefect — удалённый дефект с фото для блока архива в view.html.
type ArchivedDefect struct {
	Defect   models.RoomDefect
	RoomName string
}

// GetInspection — просмотр осмотра
func GetInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	var documents []models.Document
	storage.DB.Where("inspection_id = ?", inspection.ID).Find(&documents)

	// Удалённые дефекты с фото — для блока архива
	var deletedDefects []models.RoomDefect
	storage.DB.Unscoped().
		Preload("Photos").
		Preload("DefectTemplate").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND room_defects.deleted_at IS NOT NULL", inspection.ID).
		Find(&deletedDefects)

	// Получаем названия помещений для удалённых дефектов
	roomIDSet := map[uint]struct{}{}
	for _, d := range deletedDefects {
		roomIDSet[d.RoomID] = struct{}{}
	}
	roomIDs := make([]uint, 0, len(roomIDSet))
	for id := range roomIDSet {
		roomIDs = append(roomIDs, id)
	}
	var deletedRooms []models.InspectionRoom
	if len(roomIDs) > 0 {
		storage.DB.Unscoped().Where("id IN ?", roomIDs).Find(&deletedRooms)
	}
	roomNameMap := map[uint]string{}
	for _, r := range deletedRooms {
		roomNameMap[r.ID] = r.RoomName
	}

	// Фильтруем: только дефекты у которых есть хотя бы одно фото
	var archived []ArchivedDefect
	for _, d := range deletedDefects {
		if len(d.Photos) > 0 {
			archived = append(archived, ArchivedDefect{
				Defect:   d,
				RoomName: roomNameMap[d.RoomID],
			})
		}
	}

	// Soft retry: если есть failed фото с оставшимися попытками — перезапускаем загрузку
	go TriggerRetryForInspection(inspection.ID)

	user := CurrentUser(c)

	c.HTML(http.StatusOK, "view.html", gin.H{
		"title":          "Акт №" + inspection.ActNumber,
		"inspection":     inspection,
		"documents":      documents,
		"user":           user,
		"isAdmin":        c.GetString("userRole") == "admin",
		"archivedDefects": archived,
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

	user := CurrentUser(c)

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
		"error":              c.Query("error"),
	})
}

// PostEditInspection — сохранение дефектов по помещениям
func PostEditInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		logger.Ctx(c.Request.Context()).Error("ParseMultipartForm failed",
			"inspection_id", inspection.ID,
			"error", err,
			"content_length", c.Request.ContentLength,
		)
		editURL := "/inspections/" + strconv.FormatUint(uint64(inspection.ID), 10) + "/edit?error=" +
			url.QueryEscape("Ошибка при получении данных формы. Попробуйте сохранить ещё раз.")
		c.Redirect(http.StatusFound, editURL)
		return
	}

	activeRooms, _ := strconv.Atoi(c.PostForm("active_rooms"))
	if activeRooms < 1 {
		activeRooms = 3
	}

	// Защита от пустых данных: если ВСЕ ключевые поля формы пустые,
	// значит данные не были переданы (обрыв соединения, ошибка браузера и т.д.)
	// В этом случае НЕ удаляем старые данные.
	address := c.PostForm("address")
	ownerName := c.PostForm("owner_name")
	room1Name := c.PostForm("room_name_1")
	if address == "" && ownerName == "" && room1Name == "" && c.PostForm("active_rooms") == "" {
		logger.Ctx(c.Request.Context()).Error("empty form data detected — aborting save to protect existing data",
			"inspection_id", inspection.ID,
			"active_rooms_raw", c.PostForm("active_rooms"),
			"content_length", c.Request.ContentLength,
		)
		editURL := "/inspections/" + strconv.FormatUint(uint64(inspection.ID), 10) + "/edit?error=" +
			url.QueryEscape("Данные формы не были получены сервером. Ваши предыдущие данные сохранены. Попробуйте ещё раз.")
		c.Redirect(http.StatusFound, editURL)
		return
	}

	// Диагностика: логируем ключевые поля формы для отладки проблем с потерей данных
	logger.Ctx(c.Request.Context()).Info("PostEditInspection form received",
		"inspection_id", inspection.ID,
		"content_length", c.Request.ContentLength,
		"active_rooms", activeRooms,
		"address", address,
		"owner_name", ownerName,
		"room_1_name", room1Name,
		"form_keys_count", len(c.Request.PostForm),
	)

	// Валидация номера акта ДО транзакции: предотвращаем потерю данных
	// из-за конфликта по уникальному индексу act_number.
	submittedActNumber := strings.TrimSpace(c.PostForm("act_number"))
	if submittedActNumber == "" {
		redirectWithError(c, inspection.ID, "Номер акта не может быть пустым")
		return
	}
	if submittedActNumber != inspection.ActNumber {
		// Валидируем только ИЗМЕНЁННЫЙ номер: legacy-номера, введённые до появления
		// валидации, не должны блокировать сохранение остальных полей акта
		if err := security.ValidateActNumber(submittedActNumber); err != nil {
			redirectWithError(c, inspection.ID, "Недопустимый "+err.Error())
			return
		}
		var conflict models.Inspection
		err := storage.DB.Where("act_number = ? AND id != ?", submittedActNumber, inspection.ID).First(&conflict).Error
		if err == nil {
			redirectWithError(c, inspection.ID,
				fmt.Sprintf("Номер %q уже используется в осмотре #%d", submittedActNumber, conflict.ID))
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// Проверка уникальности упала по нетипичной причине — не блокируем,
			// defensive catch внутри транзакции перехватит 23505 в крайнем случае.
			logger.Ctx(c.Request.Context()).Error("act_number uniqueness check failed",
				"inspection_id", inspection.ID, "error", err)
		}
	}

	// Собираем данные шапки акта ДО транзакции (парсинг формы не требует БД)
	roomsCount, _ := strconv.Atoi(c.PostForm("rooms_count"))
	floor, _ := strconv.Atoi(c.PostForm("floor"))
	totalArea, _ := strconv.ParseFloat(c.PostForm("total_area"), 64)
	tempOut, _ := strconv.ParseFloat(c.PostForm("temp_outside"), 64)
	tempIn, _ := strconv.ParseFloat(c.PostForm("temp_inside"), 64)
	humidity, _ := strconv.ParseFloat(c.PostForm("humidity"), 64)

	updates := map[string]interface{}{
		"act_number":         submittedActNumber,
		"inspection_time":    c.PostForm("inspection_time"),
		"address":            address,
		"rooms_count":        roomsCount,
		"floor":              floor,
		"total_area":         totalArea,
		"temp_outside":       tempOut,
		"temp_inside":        tempIn,
		"humidity":           humidity,
		"owner_name":         ownerName,
		"developer_rep_name": c.PostForm("developer_rep_name"),
		"electricity":        c.PostForm("electricity"),
		"ventilation":        c.PostForm("ventilation"),
		"general_notes":      c.PostForm("general_notes"),
	}
	if dateStr := c.PostForm("inspection_date"); dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			updates["date"] = d
		}
	}

	// Всё сохранение — в одной транзакции (атомарность: или всё, или ничего).
	// Порядок операций важен: UPDATE шапки с act_number идёт ПЕРВЫМ. Если
	// случится гонка по уникальному индексу (23505), транзакция откатится
	// ДО удаления комнат — данные инспектора останутся целыми.
	txErr := storage.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Обновляем шапку акта (включая act_number). Ошибка уникальности
		//    ловится здесь и превращается в sentinel для понятного редиректа.
		if err := tx.Model(inspection).Updates(updates).Error; err != nil {
			if isActNumberConflict(err) {
				return errActNumberTaken
			}
			return err
		}

		// 2a. Запоминаем старые дефекты по смысловому ключу — чтобы после
		// пересоздания перепривязать их фото к новым дефектам, а не отправлять
		// в архив при каждом сохранении.
		type oldDefectRow struct {
			ID               uint
			RoomNumber       int
			Section          string
			DefectTemplateID *uint
			WallNumber       int
		}
		var oldDefects []oldDefectRow
		tx.Table("room_defects").
			Select("room_defects.id, inspection_rooms.room_number, room_defects.section, room_defects.defect_template_id, room_defects.wall_number").
			Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
			Where("inspection_rooms.inspection_id = ? AND room_defects.deleted_at IS NULL AND inspection_rooms.deleted_at IS NULL", inspection.ID).
			Scan(&oldDefects)

		defKey := func(roomNumber int, section string, tmplID *uint, wall int) string {
			t := "-"
			if tmplID != nil {
				t = strconv.FormatUint(uint64(*tmplID), 10)
			}
			return strconv.Itoa(roomNumber) + "|" + section + "|" + t + "|" + strconv.Itoa(wall)
		}
		oldByKey := make(map[string][]uint, len(oldDefects))
		for _, d := range oldDefects {
			k := defKey(d.RoomNumber, d.Section, d.DefectTemplateID, d.WallNumber)
			oldByKey[k] = append(oldByKey[k], d.ID)
		}
		relinkPhotos := func(newDefectID uint, roomNumber int, section string, tmplID *uint, wall int) {
			k := defKey(roomNumber, section, tmplID, wall)
			if ids := oldByKey[k]; len(ids) > 0 {
				tx.Model(&models.Photo{}).Where("defect_id IN ?", ids).Update("defect_id", newDefectID)
				delete(oldByKey, k)
			}
		}

		// 2. Удаляем старые комнаты и дефекты (P12: subquery вместо N+1 цикла)
		roomIDs := tx.Model(&models.InspectionRoom{}).Select("id").Where("inspection_id = ?", inspection.ID)
		tx.Where("room_id IN (?)", roomIDs).Delete(&models.RoomDefect{})
		tx.Where("inspection_id = ?", inspection.ID).Delete(&models.InspectionRoom{})

		var allTemplates []models.DefectTemplate
		tx.Order("section, order_index").Find(&allTemplates)

		simpleSections := []string{"window", "ceiling", "floor", "door", "plumbing"}

		for i := 1; i <= activeRooms; i++ {
			iStr := strconv.Itoa(i)

			room := parseRoom(c, iStr, inspection.ID, i)
			if err := tx.Create(&room).Error; err != nil {
				logger.Ctx(c.Request.Context()).Error("room create failed", "room", i, "inspection_id", inspection.ID, "error", err)
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
					nd := models.RoomDefect{
						RoomID:           room.ID,
						DefectTemplateID: &tid,
						Section:          tmpl.Section,
						Value:            val,
					}
					if err := tx.Create(&nd).Error; err == nil {
						relinkPhotos(nd.ID, i, tmpl.Section, &tid, 0)
					}
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
						nd := models.RoomDefect{
							RoomID:           room.ID,
							DefectTemplateID: &tid,
							Section:          "wall",
							Value:            val,
							WallNumber:       w,
						}
						if err := tx.Create(&nd).Error; err == nil {
							relinkPhotos(nd.ID, i, "wall", &tid, w)
						}
					}
				}
			}

			// Прочее для каждой секции
			for _, sec := range append(simpleSections, "wall") {
				if notes := c.PostForm("notes_" + sec + "_" + iStr); notes != "" {
					nd := models.RoomDefect{
						RoomID:  room.ID,
						Section: sec,
						Notes:   notes,
					}
					if err := tx.Create(&nd).Error; err == nil {
						relinkPhotos(nd.ID, i, sec, nil, 0)
					}
				}
			}
		}

		return nil
	})

	if txErr != nil {
		// Конфликт по act_number из-за гонки: показываем баннер и не теряем данные
		// (транзакция откатилась до удаления комнат).
		if errors.Is(txErr, errActNumberTaken) {
			logger.Ctx(c.Request.Context()).Warn("act_number conflict during save (race)",
				"inspection_id", inspection.ID, "submitted", submittedActNumber)
			redirectWithError(c, inspection.ID,
				fmt.Sprintf("Номер %q уже используется. Выберите другой.", submittedActNumber))
			return
		}
		logger.Ctx(c.Request.Context()).Error("edit inspection transaction failed", "inspection_id", inspection.ID, "error", txErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
		return
	}

	// Диагностика: считаем что фактически сохранилось
	var savedRoomCount int64
	var savedDefectCount int64
	storage.DB.Model(&models.InspectionRoom{}).Where("inspection_id = ?", inspection.ID).Count(&savedRoomCount)
	savedRoomIDs := storage.DB.Model(&models.InspectionRoom{}).Select("id").Where("inspection_id = ?", inspection.ID)
	storage.DB.Model(&models.RoomDefect{}).Where("room_id IN (?)", savedRoomIDs).Count(&savedDefectCount)
	logger.Ctx(c.Request.Context()).Info("PostEditInspection saved",
		"inspection_id", inspection.ID,
		"rooms_saved", savedRoomCount,
		"defects_saved", savedDefectCount,
	)

	// Создаём/переименовываем папку на Яндекс Диске в фоне — не блокируем ответ.
	if cloudStore != nil {
		go func(id uint) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("EnsureInspectionFolder panic", "inspection_id", id, "error", r)
				}
			}()
			if _, err := EnsureInspectionFolder(id); err != nil {
				logger.Error("EnsureInspectionFolder failed", "inspection_id", id, "error", err)
			}
		}(inspection.ID)
	}

	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10))
}

// GetCheckActNumber — AJAX-проверка уникальности номера акта.
// GET /api/inspections/:id/check-act-number?value=X
// Отвечает {"taken": bool, "other_id": N}. Auth такой же, как на редактирование:
// инспектор-владелец или админ.
func GetCheckActNumber(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	value := strings.TrimSpace(c.Query("value"))
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value не указан"})
		return
	}
	// Своё же значение — не считается занятым (и не валидируется: legacy-номер
	// может не соответствовать текущим правилам, но менять его не обязаны)
	if value == inspection.ActNumber {
		c.JSON(http.StatusOK, gin.H{"taken": false})
		return
	}
	if err := security.ValidateActNumber(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var conflict models.Inspection
	err := storage.DB.Where("act_number = ? AND id != ?", value, inspection.ID).First(&conflict).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"taken": false})
		return
	}
	if err != nil {
		logger.Ctx(c.Request.Context()).Error("check-act-number query failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка проверки"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"taken": true, "other_id": conflict.ID})
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

	c.JSON(http.StatusOK, BuildUploadStatusMap(uint(id)))
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

	// Собираем пути файлов до транзакции (удалим после успешного коммита)
	var docs []models.Document
	storage.DB.Where("inspection_id = ?", id).Find(&docs)
	var filePaths []string
	for _, doc := range docs {
		if doc.FilePath != "" {
			filePaths = append(filePaths, doc.FilePath)
		}
	}

	err = storage.DB.Transaction(func(tx *gorm.DB) error {
		// Удаляем фото дефектов
		defectIDs := tx.Model(&models.RoomDefect{}).Select("id").
			Where("room_id IN (?)", tx.Model(&models.InspectionRoom{}).Select("id").Where("inspection_id = ?", id))
		if err := tx.Where("defect_id IN (?)", defectIDs).Delete(&models.Photo{}).Error; err != nil {
			return err
		}
		// Удаляем дефекты всех помещений (subquery вместо N+1 цикла)
		roomIDs := tx.Model(&models.InspectionRoom{}).Select("id").Where("inspection_id = ?", id)
		if err := tx.Where("room_id IN (?)", roomIDs).Delete(&models.RoomDefect{}).Error; err != nil {
			return err
		}
		if err := tx.Where("inspection_id = ?", id).Delete(&models.InspectionRoom{}).Error; err != nil {
			return err
		}
		// Удаляем документы
		if err := tx.Where("inspection_id = ?", id).Delete(&models.Document{}).Error; err != nil {
			return err
		}
		// Удаляем сам осмотр
		return tx.Delete(&inspection).Error
	})
	if err != nil {
		logger.Ctx(c.Request.Context()).Error("ошибка удаления осмотра", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления"})
		return
	}

	// Файлы удаляем после успешной транзакции
	for _, fp := range filePaths {
		if err := os.Remove(fp); err != nil {
			logger.Ctx(c.Request.Context()).Warn("не удалось удалить файл", "path", fp, "error", err)
		}
	}

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

// escapeLike экранирует спецсимволы LIKE (%, _) в пользовательском вводе.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// parseRoom парсит данные одного помещения из формы.
func parseRoom(c *gin.Context, iStr string, inspectionID uint, roomNumber int) models.InspectionRoom {
	pf := func(name string) float64 {
		v, _ := strconv.ParseFloat(c.PostForm(name+iStr), 64)
		return v
	}
	return models.InspectionRoom{
		InspectionID:  inspectionID,
		RoomNumber:    roomNumber,
		RoomName:      c.PostForm("room_name_" + iStr),
		Length:        pf("room_length_"),
		Width:         pf("room_width_"),
		Height:        pf("room_height_"),
		Window1Height: pf("room_w1h_"),
		Window1Width:  pf("room_w1w_"),
		Window2Height: pf("room_w2h_"),
		Window2Width:  pf("room_w2w_"),
		Window3Height: pf("room_w3h_"),
		Window3Width:  pf("room_w3w_"),
		Window4Height: pf("room_w4h_"),
		Window4Width:  pf("room_w4w_"),
		Window5Height: pf("room_w5h_"),
		Window5Width:  pf("room_w5w_"),
		DoorHeight:    pf("room_dh_"),
		DoorWidth:     pf("room_dw_"),
		WindowType:    c.PostForm("room_window_type_" + iStr),
		WallType:      buildWallType(c, iStr),
	}
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

// PostUpdateStatus — обновляет только статус осмотра (draft / completed)
func PostUpdateStatus(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}
	status := c.PostForm("status")
	if status != "draft" && status != "completed" {
		status = "draft"
	}
	storage.DB.Model(inspection).Update("status", status)
	c.Redirect(http.StatusFound, "/inspections/"+strconv.FormatUint(uint64(inspection.ID), 10))
}
