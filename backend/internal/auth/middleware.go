package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// WantsJSON — запрос пришёл из fetch/XHR, а не переходом по ссылке или формой:
// такому клиенту редирект на /login бесполезен, ему нужен 401 с текстом.
func WantsJSON(c *gin.Context) bool {
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		return true
	}
	mode := c.GetHeader("Sec-Fetch-Mode")
	return mode != "" && mode != "navigate"
}

func Unauthorized(c *gin.Context) {
	if WantsJSON(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Сессия истекла, войдите заново"})
		return
	}
	c.Redirect(http.StatusFound, "/login")
	c.Abort()
}

// RenewIfNeeded — продлевает сессию: перевыпускает cookie, если токен старше суток.
func RenewIfNeeded(c *gin.Context, claims *Claims) {
	if !ShouldRenew(claims) {
		return
	}
	token, err := GenerateToken(claims.UserID, claims.Role)
	if err != nil {
		return
	}
	SetAuthCookie(c, token)
}

// RequireAuth — middleware, проверяет JWT из cookie
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("token")
		if err != nil || tokenStr == "" {
			Unauthorized(c)
			return
		}

		claims, err := ParseToken(tokenStr)
		if err != nil {
			ClearAuthCookie(c)
			Unauthorized(c)
			return
		}

		RenewIfNeeded(c, claims)
		c.Set("userID", claims.UserID)
		c.Set("userRole", claims.Role)
		c.Next()
	}
}

// RequireAdmin — middleware, только для администраторов
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("userRole")
		if !exists || role.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
			c.Abort()
			return
		}
		c.Next()
	}
}
