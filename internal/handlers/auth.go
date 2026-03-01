package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetLogin — страница входа
func GetLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Вход",
	})
}

// PostLogin — обработка формы входа
func PostLogin(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
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

	c.SetCookie("token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/inspections")
}

// GetRegister — страница регистрации
func GetRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "Регистрация",
	})
}

// PostRegister — обработка формы регистрации
func PostRegister(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	fullName := strings.TrimSpace(c.PostForm("full_name"))
	initials := strings.TrimSpace(c.PostForm("initials"))

	if email == "" || password == "" || fullName == "" || initials == "" {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Заполните все поля",
		})
		return
	}

	if len(password) < 6 {
		c.HTML(http.StatusBadRequest, "register.html", gin.H{
			"title": "Регистрация",
			"error": "Пароль должен содержать минимум 6 символов",
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

	c.Redirect(http.StatusFound, "/login?registered=1")
}

// PostLogout — выход из системы
func PostLogout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}
