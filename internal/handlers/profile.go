package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetProfile — страница профиля
func GetProfile(c *gin.Context) {
	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":   "Профиль",
		"user":    user,
		"isAdmin": c.GetString("userRole") == "admin",
	})
}

// PostProfile — обновление профиля
func PostProfile(c *gin.Context) {
	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	fullName := strings.TrimSpace(c.PostForm("full_name"))
	initials := strings.TrimSpace(c.PostForm("initials"))
	newPassword := c.PostForm("new_password")

	if fullName == "" || initials == "" {
		c.HTML(http.StatusBadRequest, "profile.html", gin.H{
			"title":   "Профиль",
			"user":    user,
			"error":   "ФИО и инициалы обязательны",
			"isAdmin": c.GetString("userRole") == "admin",
		})
		return
	}

	updates := map[string]interface{}{
		"full_name": fullName,
		"initials":  initials,
	}

	if newPassword != "" {
		if len(newPassword) < 6 {
			c.HTML(http.StatusBadRequest, "profile.html", gin.H{
				"title":   "Профиль",
				"user":    user,
				"error":   "Пароль минимум 6 символов",
				"isAdmin": c.GetString("userRole") == "admin",
			})
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err == nil {
			updates["password_hash"] = hash
		}
	}

	storage.DB.Model(&user).Updates(updates)

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":   "Профиль",
		"user":    user,
		"success": "Профиль обновлён",
		"isAdmin": c.GetString("userRole") == "admin",
	})
}
