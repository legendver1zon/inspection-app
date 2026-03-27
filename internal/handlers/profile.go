package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"net/http"
	"path/filepath"
	"strconv"
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
	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmNewPassword := c.PostForm("confirm_new_password")

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
		if currentPassword == "" || !auth.CheckPassword(currentPassword, user.PasswordHash) {
			c.HTML(http.StatusBadRequest, "profile.html", gin.H{
				"title":   "Профиль",
				"user":    user,
				"error":   "Неверный текущий пароль",
				"isAdmin": c.GetString("userRole") == "admin",
			})
			return
		}
		if newPassword != confirmNewPassword {
			c.HTML(http.StatusBadRequest, "profile.html", gin.H{
				"title":   "Профиль",
				"user":    user,
				"error":   "Пароли не совпадают",
				"isAdmin": c.GetString("userRole") == "admin",
			})
			return
		}
		if err := security.ValidatePassword(newPassword); err != nil {
			c.HTML(http.StatusBadRequest, "profile.html", gin.H{
				"title":   "Профиль",
				"user":    user,
				"error":   err.Error(),
				"isAdmin": c.GetString("userRole") == "admin",
			})
			return
		}
		hash, err := auth.HashPassword(newPassword)
		if err == nil {
			updates["password_hash"] = hash
			security.Log(security.EventPasswordChange, c.ClientIP(), "userID="+strconv.Itoa(int(userID)))
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

// PostUploadAvatar — загрузка аватара пользователя
func PostUploadAvatar(c *gin.Context) {
	userID := c.GetUint("userID")
	var user models.User
	storage.DB.First(&user, userID)

	file, err := c.FormFile("avatar")
	if err != nil {
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	if err := security.ValidateImage(file, security.MaxAvatarSize); err != nil {
		security.Log(security.EventFileRejected, c.ClientIP(), "avatar: "+err.Error())
		c.HTML(http.StatusBadRequest, "profile.html", gin.H{
			"title":   "Профиль",
			"user":    user,
			"error":   "Аватар: " + err.Error(),
			"isAdmin": c.GetString("userRole") == "admin",
		})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := "avatar_" + strconv.FormatUint(uint64(userID), 10) + ext
	if err := c.SaveUploadedFile(file, "web/static/uploads/"+filename); err != nil {
		c.Redirect(http.StatusFound, "/profile")
		return
	}

	storage.DB.Model(&user).Update("avatar_url", "/static/uploads/"+filename)
	c.Redirect(http.StatusFound, "/profile")
}
