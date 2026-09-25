package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"inspection-app/internal/textutil"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// JSON-API для React-фронтенда. Авторизация — тот же httpOnly-cookie JWT,
// что и у HTML-страниц: фронт и API живут на одном домене.

type apiUser struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	Initials  string `json:"initials"`
	Role      string `json:"role"`
	AvatarURL string `json:"avatar_url"`
}

func toAPIUser(u models.User) apiUser {
	return apiUser{
		ID: u.ID, Email: u.Email, FullName: u.FullName,
		Initials: u.Initials, Role: string(u.Role), AvatarURL: u.AvatarURL,
	}
}

type apiActCard struct {
	ID         uint    `json:"id"`
	ActNumber  string  `json:"act_number"`
	Address    string  `json:"address"`
	OwnerName  string  `json:"owner_name"`
	Date       string  `json:"date"`
	Status     string  `json:"status"`
	Inspector  string  `json:"inspector"`
	TotalArea  float64 `json:"total_area"`
	Rooms      int64   `json:"rooms"`
	Filled     int64   `json:"filled"`
	Percent    int     `json:"percent"`
	Defects    int64   `json:"defects"`
	Photos     int64   `json:"photos"`
	CloudState string  `json:"cloud_state"`
	CloudN     int64   `json:"cloud_n"`
}

func toAPIActCard(c actCard) apiActCard {
	return apiActCard{
		ID: c.ID, ActNumber: c.ActNumber, Address: c.Address, OwnerName: c.OwnerName,
		Date: c.HumanDate, Status: c.Status, Inspector: c.User.Initials,
		TotalArea: c.TotalArea, Rooms: c.TotalRooms, Filled: c.FilledRooms,
		Percent: c.Percent, Defects: c.Defects, Photos: c.Photos,
		CloudState: c.CloudState, CloudN: c.CloudN,
	}
}

// APIAuth — авторизация для /api/*: при неудаче отвечает 401 JSON,
// а не редиректом на /login, как HTML-middleware.
func APIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		unauth := func() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Требуется вход"})
		}
		tok, err := c.Cookie("token")
		if err != nil {
			unauth()
			return
		}
		claims, err := auth.ParseToken(tok)
		if err != nil {
			unauth()
			return
		}
		var u models.User
		if storage.DB.First(&u, claims.UserID).Error != nil {
			unauth()
			return
		}
		auth.RenewIfNeeded(c, claims)
		c.Set("userID", claims.UserID)
		c.Set("userRole", claims.Role)
		c.Set("currentUser", u)
		c.Next()
	}
}

// APILogin — POST /api/login {email, password}
func APILogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заполните email и пароль"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заполните email и пароль"})
		return
	}

	if allowed, retryAfter := security.LoginLimiter.Check(c.ClientIP()); !allowed {
		security.Log(security.EventLoginBlocked, c.ClientIP(), "")
		mins := int(retryAfter.Minutes()) + 1
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Слишком много неудачных попыток входа. Попробуйте через " + strconv.Itoa(mins) + " мин.",
		})
		return
	}

	var user models.User
	err := storage.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		// Анти-enumeration: тратим столько же времени, сколько на реальную проверку
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		security.LoginLimiter.Increment(c.ClientIP())
		security.Log(security.EventLoginFailed, c.ClientIP(), "email="+email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный email или пароль"})
		return
	}
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		security.LoginLimiter.Increment(c.ClientIP())
		security.Log(security.EventLoginFailed, c.ClientIP(), "email="+email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный email или пароль"})
		return
	}

	token, err := auth.GenerateToken(user.ID, string(user.Role))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
		return
	}
	auth.SetAuthCookie(c, token)
	security.LoginLimiter.Reset(c.ClientIP())
	security.Log(security.EventLoginSuccess, c.ClientIP(), "email="+email)
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(user)})
}

// APILogout — POST /api/logout
func APILogout(c *gin.Context) {
	auth.ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// APIMe — GET /api/me
func APIMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(CurrentUser(c))})
}

// APIListInspections — GET /api/inspections?q=&page=
func APIListInspections(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	q := strings.TrimSpace(c.Query("q"))

	buildQ := func() *gorm.DB {
		db := storage.DB.Model(&models.Inspection{})
		if role != "admin" {
			db = db.Where("user_id = ?", userID)
		}
		if q != "" {
			like := "%" + escapeLike(q) + "%"
			db = db.Where("act_number LIKE ? OR address LIKE ? OR owner_name LIKE ?", like, like, like)
		}
		return db
	}

	var draftCount, completedCount int64
	buildQ().Where("status = ?", "draft").Count(&draftCount)
	buildQ().Where("status = ?", "completed").Count(&completedCount)

	var drafts []models.Inspection
	buildQ().Preload("User").Where("status = ?", "draft").
		Order("created_at desc").Limit(60).Find(&drafts)

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

	draftCards := buildActCards(drafts)
	doneCards := buildActCards(completed)

	resp := gin.H{
		"drafts":          make([]apiActCard, 0, len(draftCards)),
		"completed":       make([]apiActCard, 0, len(doneCards)),
		"draft_count":     draftCount,
		"completed_count": completedCount,
		"page":            page,
		"total_pages":     totalPages,
	}
	ds := make([]apiActCard, len(draftCards))
	for i, card := range draftCards {
		ds[i] = toAPIActCard(card)
	}
	cs := make([]apiActCard, len(doneCards))
	for i, card := range doneCards {
		cs[i] = toAPIActCard(card)
	}
	resp["drafts"] = ds
	resp["completed"] = cs

	// Кэшировать список не нужно: данные меняются после каждого сохранения
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, resp)
}

// ===== Детали акта для React-страницы просмотра =====

var apiSectionNames = map[string]string{
	"window": "Окна и откосы", "ceiling": "Потолок", "wall": "Стены",
	"floor": "Пол", "door": "Двери", "plumbing": "Сантехника",
}

type apiPhoto struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

type apiDefect struct {
	ID          uint       `json:"id"`
	Section     string     `json:"section"`
	SectionName string     `json:"section_name"`
	Name        string     `json:"name"`
	Value       string     `json:"value"`
	WallNumber  int        `json:"wall_number"`
	Notes       string     `json:"notes"`
	Photos      []apiPhoto `json:"photos"`
}

type apiRoom struct {
	ID      uint        `json:"id"`
	Number  int         `json:"number"`
	Name    string      `json:"name"`
	Defects []apiDefect `json:"defects"`
}

type apiArchivedDefect struct {
	RoomName string     `json:"room_name"`
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	Photos   []apiPhoto `json:"photos"`
}

type apiDocument struct {
	ID      uint   `json:"id"`
	Format  string `json:"format"`
	Created string `json:"created"`
}

func toAPIDefect(d models.RoomDefect) apiDefect {
	name := d.DefectTemplate.Name
	if d.DefectTemplateID == nil || name == "" {
		name = "Прочее"
	}
	photos := make([]apiPhoto, len(d.Photos))
	for i, p := range d.Photos {
		photos[i] = apiPhoto{ID: p.ID, Status: p.UploadStatus}
	}
	return apiDefect{
		ID: d.ID, Section: d.Section, SectionName: apiSectionNames[d.Section],
		Name: name, Value: d.Value, WallNumber: d.WallNumber, Notes: d.Notes,
		Photos: photos,
	}
}

// APIGetInspection — GET /api/inspections/:id
func APIGetInspection(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	rooms := make([]apiRoom, 0, len(inspection.Rooms))
	for _, r := range inspection.Rooms {
		room := apiRoom{ID: r.ID, Number: r.RoomNumber, Name: r.RoomName, Defects: make([]apiDefect, 0, len(r.Defects))}
		for _, d := range r.Defects {
			room.Defects = append(room.Defects, toAPIDefect(d))
		}
		rooms = append(rooms, room)
	}

	// Архив: мягко удалённые дефекты с фото (показываются, но не идут в PDF)
	var deletedDefects []models.RoomDefect
	storage.DB.Unscoped().
		Preload("Photos").
		Preload("DefectTemplate").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND room_defects.deleted_at IS NOT NULL", inspection.ID).
		Find(&deletedDefects)

	archived := make([]apiArchivedDefect, 0)
	if len(deletedDefects) > 0 {
		roomIDs := make([]uint, 0, len(deletedDefects))
		for _, d := range deletedDefects {
			roomIDs = append(roomIDs, d.RoomID)
		}
		var archRooms []models.InspectionRoom
		storage.DB.Unscoped().Where("id IN ?", roomIDs).Find(&archRooms)
		roomNames := make(map[uint]string, len(archRooms))
		for _, r := range archRooms {
			roomNames[r.ID] = r.RoomName
		}
		for _, d := range deletedDefects {
			if len(d.Photos) == 0 {
				continue
			}
			ad := toAPIDefect(d)
			archived = append(archived, apiArchivedDefect{
				RoomName: roomNames[d.RoomID], Name: ad.Name, Value: ad.Value, Photos: ad.Photos,
			})
		}
	}

	var docs []models.Document
	storage.DB.Where("inspection_id = ?", inspection.ID).Order("created_at desc").Find(&docs)
	documents := make([]apiDocument, len(docs))
	for i, d := range docs {
		documents[i] = apiDocument{ID: d.ID, Format: d.Format, Created: humanDate(d.CreatedAt)}
	}

	planImage := ""
	if inspection.PlanImage != "" {
		planImage = inspection.PlanImage
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"inspection": gin.H{
			"id":                 inspection.ID,
			"act_number":         inspection.ActNumber,
			"status":             inspection.Status,
			"date":               humanDate(inspection.Date),
			"time":               inspection.InspectionTime,
			"address":            inspection.Address,
			"owner_name":         inspection.OwnerName,
			"developer_rep_name": inspection.DeveloperRepName,
			"inspector":          inspection.User.Initials,
			"rooms_count":        inspection.RoomsCount,
			"floor":              inspection.Floor,
			"total_area":         inspection.TotalArea,
			"temp_outside":       inspection.TempOutside,
			"temp_inside":        inspection.TempInside,
			"humidity":           inspection.Humidity,
			"electricity":        inspection.Electricity,
			"ventilation":        inspection.Ventilation,
			"general_notes":      inspection.GeneralNotes,
			"plan_image":         planImage,
			"photo_folder_url":   inspection.PhotoFolderURL,
			"rooms":              rooms,
			"archived":           archived,
			"documents":          documents,
		},
	})
}

// ===== Данные формы редактирования =====

// APIGetEditData — GET /api/inspections/:id/edit-data.
// Отдаёт сырые значения полей акта, замеры помещений с дефектами
// и справочник шаблонов. Сохранение идёт в старый POST /inspections/:id/edit —
// React-форма собирает те же поля, что и HTML-форма.
func APIGetEditData(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	var templates []models.DefectTemplate
	storage.DB.Order("section, order_index").Find(&templates)
	tpls := make([]gin.H, len(templates))
	for i, t := range templates {
		tpls[i] = gin.H{
			"id": t.ID, "section": t.Section, "name": t.Name,
			"threshold": t.Threshold, "unit": t.Unit,
		}
	}

	rooms := make([]gin.H, 0, len(inspection.Rooms))
	for _, r := range inspection.Rooms {
		defects := make([]gin.H, 0, len(r.Defects))
		for _, d := range r.Defects {
			photos := make([]apiPhoto, len(d.Photos))
			for pi, p := range d.Photos {
				photos[pi] = apiPhoto{ID: p.ID, Status: p.UploadStatus}
			}
			defects = append(defects, gin.H{
				"id": d.ID, "template_id": d.DefectTemplateID, "section": d.Section,
				"value": d.Value, "wall_number": d.WallNumber, "notes": d.Notes,
				"photos": photos,
			})
		}
		wallTypes := []string{}
		if r.WallType != "" {
			wallTypes = strings.Split(r.WallType, ",")
		}
		rooms = append(rooms, gin.H{
			"number": r.RoomNumber, "name": r.RoomName,
			"length": r.Length, "width": r.Width, "height": r.Height,
			"w1h": r.Window1Height, "w1w": r.Window1Width,
			"w2h": r.Window2Height, "w2w": r.Window2Width,
			"w3h": r.Window3Height, "w3w": r.Window3Width,
			"w4h": r.Window4Height, "w4w": r.Window4Width,
			"w5h": r.Window5Height, "w5w": r.Window5Width,
			"dh": r.DoorHeight, "dw": r.DoorWidth,
			"window_type": r.WindowType, "wall_types": wallTypes,
			"defects": defects,
		})
	}
	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i]["number"].(int) < rooms[j]["number"].(int)
	})

	date := ""
	if !inspection.Date.IsZero() {
		date = inspection.Date.Format("2006-01-02")
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"act": gin.H{
			"id":                 inspection.ID,
			"act_number":         inspection.ActNumber,
			"status":             inspection.Status,
			"date":               date,
			"time":               inspection.InspectionTime,
			"address":            inspection.Address,
			"owner_name":         inspection.OwnerName,
			"developer_rep_name": inspection.DeveloperRepName,
			"rooms_count":        inspection.RoomsCount,
			"floor":              inspection.Floor,
			"total_area":         inspection.TotalArea,
			"temp_outside":       inspection.TempOutside,
			"temp_inside":        inspection.TempInside,
			"humidity":           inspection.Humidity,
			"electricity":        inspection.Electricity,
			"ventilation":        inspection.Ventilation,
			"general_notes":      inspection.GeneralNotes,
			"plan_image":         inspection.PlanImage,
		},
		"rooms":     rooms,
		"templates": tpls,
	})
}

// ===== Дашборд, профиль, админка =====

// APIAdminOnly — доступ только администраторам (после APIAuth)
func APIAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("userRole") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Только для администраторов"})
			return
		}
		c.Next()
	}
}

// APIDashboard — GET /api/dashboard
func APIDashboard(c *gin.Context) {
	userID := c.GetUint("userID")
	role := c.GetString("userRole")

	base := func() *gorm.DB {
		db := storage.DB.Model(&models.Inspection{})
		if role != "admin" {
			db = db.Where("user_id = ?", userID)
		}
		return db
	}

	var total, draft, completed, today, week int64
	base().Count(&total)
	base().Where("status = ?", "draft").Count(&draft)
	base().Where("status = ?", "completed").Count(&completed)
	dayStart := time.Now().Truncate(24 * time.Hour)
	base().Where("created_at >= ?", dayStart).Count(&today)
	base().Where("created_at >= ?", dayStart.AddDate(0, 0, -7)).Count(&week)

	var photoPending, photoFailed int64
	storage.DB.Model(&models.Photo{}).Where("upload_status IN ?", []string{"pending", "uploading"}).Count(&photoPending)
	storage.DB.Model(&models.Photo{}).Where("upload_status = ?", "failed").Count(&photoFailed)

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"total": total, "draft": draft, "completed": completed,
		"today": today, "week": week,
		"photo_pending": photoPending, "photo_failed": photoFailed,
	})
}

// APIUpdateProfile — POST /api/profile
func APIUpdateProfile(c *gin.Context) {
	var req struct {
		FullName        string `json:"full_name"`
		Initials        string `json:"initials"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		Confirm         string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	user := CurrentUser(c)
	fullName := strings.TrimSpace(req.FullName)
	initials := strings.TrimSpace(req.Initials)
	if fullName == "" || initials == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ФИО и инициалы обязательны"})
		return
	}

	updates := map[string]interface{}{"full_name": fullName, "initials": initials}
	if req.NewPassword != "" {
		if req.CurrentPassword == "" || !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный текущий пароль"})
			return
		}
		if req.NewPassword != req.Confirm {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Пароли не совпадают"})
			return
		}
		if err := security.ValidatePassword(req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
			return
		}
		updates["password_hash"] = hash
		security.Log(security.EventPasswordChange, c.ClientIP(), "userID="+strconv.Itoa(int(user.ID)))
	}

	if err := storage.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
		return
	}
	var fresh models.User
	storage.DB.First(&fresh, user.ID)
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(fresh)})
}

type apiAdminUser struct {
	apiUser
	Created string `json:"created"`
	Acts    int64  `json:"acts"`
}

// APIListUsers — GET /api/users (admin)
func APIListUsers(c *gin.Context) {
	var users []models.User
	storage.DB.Order("created_at desc").Find(&users)

	type idCount struct {
		UserID uint
		C      int64
	}
	var rows []idCount
	storage.DB.Model(&models.Inspection{}).
		Select("user_id, count(*) as c").Group("user_id").Scan(&rows)
	acts := make(map[uint]int64, len(rows))
	for _, r := range rows {
		acts[r.UserID] = r.C
	}

	out := make([]apiAdminUser, len(users))
	for i, u := range users {
		out[i] = apiAdminUser{apiUser: toAPIUser(u), Created: humanDate(u.CreatedAt), Acts: acts[u.ID]}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"users": out})
}

// APIUpdateUser — POST /api/users/:id (admin)
func APIUpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}
	var target models.User
	if err := storage.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	var req struct {
		FullName    string `json:"full_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	fullName := strings.TrimSpace(req.FullName)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if fullName == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ФИО и email обязательны"})
		return
	}
	if len(strings.Fields(fullName)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Введите полное ФИО (минимум Фамилия и Имя)"})
		return
	}
	if req.Role != "admin" && req.Role != "inspector" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверная роль"})
		return
	}
	if target.ID == c.GetUint("userID") && req.Role != string(target.Role) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя изменить свою роль"})
		return
	}

	updates := map[string]interface{}{
		"full_name": fullName,
		"initials":  textutil.Initials(fullName),
		"email":     email,
		"role":      req.Role,
	}
	if req.NewPassword != "" {
		if err := security.ValidatePassword(req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
			return
		}
		updates["password_hash"] = hash
	}

	if err := storage.DB.Model(&target).Updates(updates).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось сохранить (email может быть занят)"})
		return
	}
	var fresh models.User
	storage.DB.First(&fresh, target.ID)
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(fresh)})
}

// APIDeleteUser — POST /api/users/:id/delete (admin)
func APIDeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID"})
		return
	}
	if uint(id) == c.GetUint("userID") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя удалить свой аккаунт"})
		return
	}
	var target models.User
	if err := storage.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	if target.Role == models.RoleAdmin {
		var admins int64
		storage.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&admins)
		if admins <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя удалить единственного администратора"})
			return
		}
	}
	storage.DB.Delete(&models.User{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
