package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

// CurrentUser возвращает пользователя текущего запроса, загруженного
// auth-middleware (main.go кладёт его в контекст как "currentUser").
// Fallback на запрос к БД — для вызовов вне защищённой группы (например, в тестах).
func CurrentUser(c *gin.Context) models.User {
	if v, ok := c.Get("currentUser"); ok {
		if u, ok := v.(models.User); ok {
			return u
		}
	}
	var u models.User
	storage.DB.First(&u, c.GetUint("userID"))
	return u
}
