package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// PostGenerateDocument — генерация документа (заглушка, реализуем позже)
func PostGenerateDocument(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	format := c.PostForm("format")
	if format == "" {
		format = "pdf"
	}

	userID := c.GetUint("userID")

	// TODO: реализовать генерацию PDF/DOCX в следующем этапе
	doc := models.Document{
		InspectionID: inspection.ID,
		Format:       format,
		FilePath:     "", // будет заполнено после генерации
		GeneratedBy:  userID,
	}
	storage.DB.Create(&doc)

	c.Redirect(http.StatusFound, "/inspections/"+c.Param("id"))
}

// GetDownloadDocument — скачивание документа
func GetDownloadDocument(c *gin.Context) {
	id := c.Param("id")

	var doc models.Document
	if err := storage.DB.Preload("Inspection").First(&doc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Документ не найден"})
		return
	}

	// Проверка доступа
	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && doc.Inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	if doc.FilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл ещё не сгенерирован"})
		return
	}

	absPath, _ := filepath.Abs(doc.FilePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не найден на диске"})
		return
	}

	c.File(absPath)
}
