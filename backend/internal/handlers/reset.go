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

// requestPasswordReset — генерирует код и отправляет письмо.
// Ответ наружу всегда одинаковый, чтобы не раскрывать, есть ли email в базе.
func requestPasswordReset(email, ip string) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Инкрементируем на каждый запрос (не только при найденном email),
	// чтобы не давать злоумышленнику 3 "бесплатных" проверки несуществующих адресов.
	security.ForgotPasswordLimiter.Increment(ip)

	var user models.User
	if err := storage.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return
	}

	// 6-значный код на crypto/rand (криптографически безопасный)
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
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

	security.Log(security.EventForgotPassword, ip, "email="+email)
}

// PostForgotPassword — форма запроса кода
func PostForgotPassword(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	requestPasswordReset(email, c.ClientIP())
	c.HTML(http.StatusOK, "forgot_password.html", gin.H{
		"title": "Восстановление пароля",
		"sent":  true,
		"email": email,
	})
}

// GetResetPassword — форма ввода кода и нового пароля
func GetResetPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"title": "Новый пароль",
		"email": c.Query("email"),
	})
}

// resetPassword проверяет код и меняет пароль.
// Возвращает HTTP-статус и текст ошибки (пустой при успехе).
func resetPassword(email, code, password, confirm, ip string) (int, string) {
	email = strings.ToLower(strings.TrimSpace(email))

	if password != confirm {
		return http.StatusBadRequest, "Пароли не совпадают"
	}
	if err := security.ValidatePassword(password); err != nil {
		return http.StatusBadRequest, err.Error()
	}

	var user models.User
	if err := storage.DB.Where("email = ?", email).First(&user).Error; err != nil {
		security.ResetPasswordLimiter.Increment(ip)
		return http.StatusBadRequest, "Пользователь не найден"
	}

	// Лимит и на аккаунт (не только на IP): распределённый перебор кода
	// с многих IP упирается в счётчик по email
	if allowed, _ := security.ResetPasswordLimiter.Check("email:" + email); !allowed {
		security.Log(security.EventPasswordResetBlocked, ip, "email="+email)
		return http.StatusTooManyRequests, "Слишком много попыток для этого аккаунта. Запросите новый код позже."
	}

	if user.ResetToken == "" || user.ResetToken != code {
		security.ResetPasswordLimiter.Increment(ip)
		security.ResetPasswordLimiter.Increment("email:" + email)
		return http.StatusBadRequest, "Неверный код"
	}
	if user.ResetExpiry == nil || time.Now().After(*user.ResetExpiry) {
		return http.StatusBadRequest, "Код истёк. Запросите новый."
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return http.StatusInternalServerError, "Ошибка сервера"
	}

	storage.DB.Model(&user).Updates(map[string]interface{}{
		"password_hash": hash,
		"reset_token":   "",
		"reset_expiry":  nil,
	})

	security.ResetPasswordLimiter.Reset(ip)
	security.ResetPasswordLimiter.Reset("email:" + email)
	security.Log(security.EventPasswordReset, ip, "email="+email)
	return http.StatusOK, ""
}

// PostResetPassword — проверяет код, обновляет пароль
func PostResetPassword(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	_, msg := resetPassword(email, c.PostForm("code"), c.PostForm("password"), c.PostForm("confirm"), c.ClientIP())
	if msg != "" {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"title": "Новый пароль",
			"email": email,
			"error": msg,
		})
		return
	}
	c.Redirect(http.StatusFound, "/login?reset=1")
}
