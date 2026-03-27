package handlers

import (
	"crypto/rand"
	"fmt"
	"inspection-app/internal/auth"
	"inspection-app/internal/mailer"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GetForgotPassword — форма ввода email для сброса пароля
func GetForgotPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot_password.html", gin.H{
		"title": "Восстановление пароля",
		"sent":  c.Query("sent") == "1",
	})
}

// PostForgotPassword — генерирует код и отправляет письмо
func PostForgotPassword(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))

	// Инкрементируем на каждый запрос (не только при найденном email),
	// чтобы не давать злоумышленнику 3 "бесплатных" проверки несуществующих адресов.
	security.ForgotPasswordLimiter.Increment(c.ClientIP())

	// Всегда показываем одинаковый ответ — чтобы не раскрывать, есть ли email в базе
	showSent := func() {
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"title": "Восстановление пароля",
			"sent":  true,
			"email": email,
		})
	}

	var user models.User
	if err := storage.DB.Where("email = ?", email).First(&user).Error; err != nil {
		showSent()
		return
	}

	// 6-значный код на crypto/rand (криптографически безопасный)
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		showSent()
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expiry := time.Now().Add(15 * time.Minute)

	storage.DB.Model(&user).Updates(map[string]interface{}{
		"reset_token":  code,
		"reset_expiry": expiry,
	})

	body := fmt.Sprintf(
		"Код для сброса пароля в системе «Акты осмотра»:\n\n    %s\n\nКод действителен 15 минут.\n\nЕсли вы не запрашивали сброс — проигнорируйте это письмо.",
		code,
	)
	mailer.Send(email, "Сброс пароля — Акты осмотра", body)

	security.Log(security.EventForgotPassword, c.ClientIP(), "email="+email)
	showSent()
}

// GetResetPassword — форма ввода кода и нового пароля
func GetResetPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"title": "Новый пароль",
		"email": c.Query("email"),
	})
}

// PostResetPassword — проверяет код, обновляет пароль
func PostResetPassword(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	code := c.PostForm("code")
	password := c.PostForm("password")
	confirm := c.PostForm("confirm")

	renderErr := func(msg string) {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"title": "Новый пароль",
			"email": email,
			"error": msg,
		})
	}

	if password != confirm {
		renderErr("Пароли не совпадают")
		return
	}
	if err := security.ValidatePassword(password); err != nil {
		renderErr(err.Error())
		return
	}

	var user models.User
	if err := storage.DB.Where("email = ?", email).First(&user).Error; err != nil {
		renderErr("Пользователь не найден")
		return
	}

	if user.ResetToken == "" || user.ResetToken != code {
		renderErr("Неверный код")
		return
	}
	if user.ResetExpiry == nil || time.Now().After(*user.ResetExpiry) {
		renderErr("Код истёк. Запросите новый.")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		renderErr("Ошибка сервера")
		return
	}

	storage.DB.Model(&user).Updates(map[string]interface{}{
		"password_hash": hash,
		"reset_token":   "",
		"reset_expiry":  nil,
	})

	security.Log(security.EventPasswordReset, c.ClientIP(), "email="+email)
	c.Redirect(http.StatusFound, "/login?reset=1")
}
