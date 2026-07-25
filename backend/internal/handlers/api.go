package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"

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
