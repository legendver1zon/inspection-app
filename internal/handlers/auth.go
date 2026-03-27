package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

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
	if result.Error != nil || !auth.CheckPassword(password, user.PasswordHash) {
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
	c.SetCookie("token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/inspections")
}

// GetRegister — страница регистрации
func GetRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "Регистрация",
	})
}

// buildInitials генерирует «Фамилия И.О.» из полного ФИО
func buildInitials(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return fullName
	}
	result := parts[0]
	for i := 1; i < len(parts) && i <= 2; i++ {
		runes := []rune(parts[i])
		if len(runes) > 0 {
			result += " " + string(runes[0]) + "."
		}
	}
	return result
}

// PostRegister — обработка формы регистрации
func PostRegister(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	fullName := strings.TrimSpace(c.PostForm("full_name"))
	noPatronymic := c.PostForm("no_patronymic") == "1"
	initials := buildInitials(fullName)

	if email == "" || password == "" || fullName == "" {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Заполните все поля",
		})
		return
	}

	minWords := 3
	if noPatronymic {
		minWords = 2
	}
	if len(strings.Fields(fullName)) < minWords {
		errMsg := "Введите полное ФИО (Фамилия, Имя и Отчество)"
		if noPatronymic {
			errMsg = "Введите Фамилию и Имя"
		}
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": errMsg,
		})
		return
	}

	if password != confirmPassword {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Пароли не совпадают",
		})
		return
	}

	if err := security.ValidatePassword(password); err != nil {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": err.Error(),
		})
		return
	}

	// Проверяем, не занят ли email
	var existing models.User
	if storage.DB.Where("email = ?", email).First(&existing).Error == nil {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Пользователь с таким email уже существует",
		})
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Ошибка сервера",
		})
		return
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
		Initials:     initials,
		Role:         role,
	}

	if err := storage.DB.Create(&user).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Ошибка создания пользователя",
		})
		return
	}

	security.RegisterLimiter.Increment(c.ClientIP())
	security.Log(security.EventRegister, c.ClientIP(), "email="+email)
	c.Redirect(http.StatusFound, "/login?registered=1")
}

// PostLogout — выход из системы
func PostLogout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
