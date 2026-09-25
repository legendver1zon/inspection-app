package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"inspection-app/internal/textutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// dummyHash — заранее вычисленный bcrypt hash для constant-time проверки
// при несуществующем пользователе (anti timing attack).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

// GetLogin — страница входа
func GetLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":      "Вход",
		"registered": c.Query("registered"),
		"reset":      c.Query("reset"),
	})
}

// PostLogin — обработка формы входа
func PostLogin(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")

	if email == "" || password == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "Вход",
			"error": "Заполните все поля",
		})
		return
	}

	var user models.User
	result := storage.DB.Where("email = ?", email).First(&user)
	// Constant-time: всегда выполняем bcrypt даже если пользователь не найден (anti timing attack)
	hashToCheck := string(dummyHash)
	if result.Error == nil {
		hashToCheck = user.PasswordHash
	}
	passwordValid := auth.CheckPassword(password, hashToCheck)
	if result.Error != nil || !passwordValid {
		security.LoginLimiter.Increment(c.ClientIP())
		security.Log(security.EventLoginFailed, c.ClientIP(), "email="+email)
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Вход",
			"error": "Неверный email или пароль",
		})
		return
	}

	token, err := auth.GenerateToken(user.ID, string(user.Role))
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"title": "Вход",
			"error": "Ошибка сервера",
		})
		return
	}

	security.LoginLimiter.Reset(c.ClientIP())
	security.Log(security.EventLoginSuccess, c.ClientIP(), "email="+email)
	auth.SetAuthCookie(c, token)
	c.Redirect(http.StatusFound, "/inspections")
}

// GetRegister — страница регистрации
func GetRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "Регистрация",
	})
}

type registerInput struct {
	Email, Password, Confirm, FullName string
	NoPatronymic                       bool
}

// registerUser проверяет данные и создаёт пользователя.
// Возвращает HTTP-статус и текст ошибки (пустой при успехе).
func registerUser(in registerInput, ip string) (int, string) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	if email == "" || in.Password == "" || fullName == "" {
		return http.StatusBadRequest, "Заполните все поля"
	}

	minWords := 3
	if in.NoPatronymic {
		minWords = 2
	}
	if len(strings.Fields(fullName)) < minWords {
		if in.NoPatronymic {
			return http.StatusBadRequest, "Введите Фамилию и Имя"
		}
		return http.StatusBadRequest, "Введите полное ФИО (Фамилия, Имя и Отчество)"
	}
	if in.Password != in.Confirm {
		return http.StatusBadRequest, "Пароли не совпадают"
	}
	if err := security.ValidatePassword(in.Password); err != nil {
		return http.StatusBadRequest, err.Error()
	}

	var existing models.User
	if storage.DB.Where("email = ?", email).First(&existing).Error == nil {
		return http.StatusBadRequest, "Пользователь с таким email уже существует"
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return http.StatusInternalServerError, "Ошибка сервера"
	}

	// Первый пользователь становится администратором
	role := models.RoleInspector
	var count int64
	storage.DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		role = models.RoleAdmin
	}

	user := models.User{
		Email:        email,
		PasswordHash: hash,
		FullName:     fullName,
		Initials:     textutil.Initials(fullName),
		Role:         role,
	}
	if err := storage.DB.Create(&user).Error; err != nil {
		return http.StatusInternalServerError, "Ошибка создания пользователя"
	}

	security.RegisterLimiter.Increment(ip)
	security.Log(security.EventRegister, ip, "email="+email)
	return http.StatusOK, ""
}

// PostRegister — обработка формы регистрации
func PostRegister(c *gin.Context) {
	status, msg := registerUser(registerInput{
		Email:        c.PostForm("email"),
		Password:     c.PostForm("password"),
		Confirm:      c.PostForm("confirm_password"),
		FullName:     c.PostForm("full_name"),
		NoPatronymic: c.PostForm("no_patronymic") == "1",
	}, c.ClientIP())
	if msg != "" {
		c.HTML(status, "register.html", gin.H{
			"title": "Регистрация",
			"error": msg,
		})
		return
	}
	c.Redirect(http.StatusFound, "/login?registered=1")
}

// PostLogout — выход из системы
func PostLogout(c *gin.Context) {
	auth.ClearAuthCookie(c)
	c.Redirect(http.StatusFound, "/login")
}
