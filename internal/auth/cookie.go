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

// SetAuthCookie устанавливает JWT-токен в httpOnly cookie.
// В production: Secure=true, SameSite=Lax (CSRF-защита).
func SetAuthCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, 86400, "/", "", IsProduction(), true)
}

// ClearAuthCookie удаляет cookie авторизации.
func ClearAuthCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", IsProduction(), true)
}
