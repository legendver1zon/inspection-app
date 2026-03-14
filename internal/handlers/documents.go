package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/pdf"
	"inspection-app/internal/storage"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// PostGenerateDocument — генерация PDF документа
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
	outputDir := "web/static/documents"

	var filePath string
	var genErr error

	if format == "pdf" {
		SyncInspectionPhotos(inspection.ID)
		// Перечитываем осмотр, чтобы получить актуальный PhotoFolderURL после синхронизации
		storage.DB.Preload("User").Preload("Rooms.Defects.DefectTemplate").Preload("Rooms.Defects.Photos").
			First(&inspection, inspection.ID)
		filePath, genErr = pdf.Generate(inspection, outputDir)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Формат не поддерживается: " + format})
		return
	}

	if genErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Ошибка генерации PDF: %v", genErr),
		})
		return
	}

	// Сохраняем абсолютный путь, чтобы скачивание работало независимо от CWD
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		absFilePath = filePath
	}

	doc := models.Document{
		InspectionID: inspection.ID,
		Format:       format,
		FilePath:     absFilePath,
		GeneratedBy:  userID,
	}
	storage.DB.Create(&doc)

	c.Redirect(http.StatusFound, "/inspections/"+c.Param("id"))
}

// PostDeleteDocument — удаление PDF документа
func PostDeleteDocument(c *gin.Context) {
	id := c.Param("id")

	var doc models.Document
	if err := storage.DB.Preload("Inspection").First(&doc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Документ не найден"})
		return
	}

	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && doc.Inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	if doc.FilePath != "" {
		os.Remove(doc.FilePath)
	}
	storage.DB.Delete(&doc)

	c.Redirect(http.StatusFound, fmt.Sprintf("/inspections/%d", doc.InspectionID))
}

// GetDownloadDocument — скачивание документа
func GetDownloadDocument(c *gin.Context) {
	id := c.Param("id")

	var doc models.Document
	if err := storage.DB.Preload("Inspection").First(&doc, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Документ не найден"})
		return
	}

	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && doc.Inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	if doc.FilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не сгенерирован"})
		return
	}

	absPath, _ := filepath.Abs(doc.FilePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		// Файл потерян — удаляем устаревшую запись и редиректим обратно
		storage.DB.Delete(&doc)
		c.Redirect(http.StatusFound, fmt.Sprintf("/inspections/%d", doc.InspectionID))
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(
		`attachment; filename="act_%s.%s"`, doc.Inspection.ActNumber, doc.Format,
	))
	c.File(absPath)
}
