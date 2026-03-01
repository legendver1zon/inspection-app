package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetAdminUsers — список пользователей (только admin)
func GetAdminUsers(c *gin.Context) {
	var users []models.User
	storage.DB.Order("created_at desc").Find(&users)

	userID := c.GetUint("userID")
	var currentUser models.User
	storage.DB.First(&currentUser, userID)

	c.HTML(http.StatusOK, "users.html", gin.H{
		"title":       "Управление пользователями",
		"users":       users,
		"user":        currentUser,
		"isAdmin":     true,
		"currentUser": currentUser,
	})
}

// PostAdminChangeRole — изменение роли пользователя
func PostAdminChangeRole(c *gin.Context) {
	id := c.Param("id")
	role := c.PostForm("role")

	if role != "admin" && role != "inspector" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверная роль"})
		return
	}

	storage.DB.Model(&models.User{}).Where("id = ?", id).Update("role", role)
	c.Redirect(http.StatusFound, "/admin/users")
}

// DeleteAdminUser — удаление пользователя
func DeleteAdminUser(c *gin.Context) {
	id := c.Param("id")
	currentUserID := c.GetUint("userID")

	// Нельзя удалить себя
	if id == strconv.FormatUint(uint64(currentUserID), 10) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя удалить свой аккаунт"})
		return
	}

	storage.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusFound, "/admin/users")
}
