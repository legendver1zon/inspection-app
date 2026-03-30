package auth

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// IsProduction возвращает true если приложение запущено в production-режиме.
func IsProduction() bool {
	return os.Getenv("GIN_MODE") == "release"
}

// isSecureCookie возвращает true только если явно задан COOKIE_SECURE=true.
// Secure=true требует HTTPS. По умолчанию false — безопасно для HTTP-сайтов.
func isSecureCookie() bool {
	return os.Getenv("COOKIE_SECURE") == "true"
}

// SetAuthCookie устанавливает JWT-токен в httpOnly cookie.
// SameSite=Lax всегда (CSRF-защита). Secure — только при HTTPS (COOKIE_SECURE=true).
func SetAuthCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, 86400, "/", "", isSecureCookie(), true)
}

// ClearAuthCookie удаляет cookie авторизации.
func ClearAuthCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", isSecureCookie(), true)
}
