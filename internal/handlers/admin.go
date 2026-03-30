package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"net/http"
	"strconv"
	"strings"

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

// GetAdminEditUser — страница редактирования пользователя
func GetAdminEditUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := storage.DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}
	currentUserID := c.GetUint("userID")
	var currentUser models.User
	storage.DB.First(&currentUser, currentUserID)
	c.HTML(http.StatusOK, "edit_user.html", gin.H{
		"title":       "Редактирование пользователя",
		"editUser":    user,
		"user":        currentUser,
		"isAdmin":     true,
		"currentUser": currentUser,
	})
}

// PostAdminEditUser — сохранение изменений пользователя
func PostAdminEditUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := storage.DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	currentUserID := c.GetUint("userID")
	var currentUser models.User
	storage.DB.First(&currentUser, currentUserID)

	fullName := strings.TrimSpace(c.PostForm("full_name"))
	email := strings.ToLower(strings.TrimSpace(c.PostForm("email")))
	role := c.PostForm("role")
	newPassword := c.PostForm("new_password")

	renderErr := func(errMsg string) {
		c.HTML(http.StatusBadRequest, "edit_user.html", gin.H{
			"title":       "Редактирование пользователя",
			"editUser":    user,
			"user":        currentUser,
			"isAdmin":     true,
			"currentUser": currentUser,
			"error":       errMsg,
		})
	}

	if fullName == "" || email == "" {
		renderErr("ФИО и email обязательны")
		return
	}
	if len(strings.Fields(fullName)) < 2 {
		renderErr("Введите полное ФИО (минимум Фамилия и Имя)")
		return
	}
	if role != "admin" && role != "inspector" {
		renderErr("Неверная роль")
		return
	}

	updates := map[string]interface{}{
		"full_name": fullName,
		"initials":  buildInitials(fullName),
		"email":     email,
		"role":      role,
	}

	if newPassword != "" {
		if err := security.ValidatePassword(newPassword); err != nil {
			renderErr(err.Error())
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			renderErr("Ошибка сервера")
			return
		}
		updates["password_hash"] = hash
	}

	storage.DB.Model(&user).Updates(updates)
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

	// Нельзя удалить последнего администратора
	var target models.User
	if err := storage.DB.First(&target, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}
	if target.Role == models.RoleAdmin {
		var adminCount int64
		storage.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Нельзя удалить единственного администратора"})
			return
		}
	}

	storage.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusFound, "/admin/users")
}
