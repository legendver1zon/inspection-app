package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSON-варианты регистрации и сброса пароля для React-фронтенда.
// Логика общая с HTML-обработчиками (registerUser, requestPasswordReset, resetPassword).

func APIRegister(c *gin.Context) {
	var req struct {
		Email           string `json:"email"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
		FullName        string `json:"full_name"`
		NoPatronymic    bool   `json:"no_patronymic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заполните все поля"})
		return
	}
	status, msg := registerUser(registerInput{
		Email:        req.Email,
		Password:     req.Password,
		Confirm:      req.ConfirmPassword,
		FullName:     req.FullName,
		NoPatronymic: req.NoPatronymic,
	}, c.ClientIP())
	if msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func APIForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите email"})
		return
	}
	requestPasswordReset(req.Email, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func APIResetPassword(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
		Confirm  string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Заполните все поля"})
		return
	}
	status, msg := resetPassword(req.Email, req.Code, req.Password, req.Confirm, c.ClientIP())
	if msg != "" {
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
