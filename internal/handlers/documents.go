package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/pdf"
	"inspection-app/internal/queue"
	"inspection-app/internal/storage"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// uploadQueue — глобальная очередь задач загрузки фото.
// Устанавливается из main.go; nil означает синхронный fallback.
var uploadQueue *queue.RedisQueue

// SetUploadQueue регистрирует Redis-очередь для асинхронной загрузки фото.
func SetUploadQueue(q *queue.RedisQueue) {
	uploadQueue = q
}

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
		// Создаём/переименовываем папку на Яндекс Диске (нужна для QR-кода в PDF).
		// Загрузка фото происходит сразу при добавлении, здесь не запускаем.
		if _, err := EnsureInspectionFolder(inspection.ID); err != nil {
			log.Printf("PostGenerateDocument EnsureFolder: %v", err)
		}
		// Перечитываем осмотр, чтобы получить актуальный PhotoFolderURL
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

	// Санитизация ActNumber для безопасного использования в HTTP-заголовке
	safeActNumber := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '/' || r < 32 {
			return '_'
		}
		return r
	}, doc.Inspection.ActNumber)
	c.Header("Content-Disposition", fmt.Sprintf(
		`attachment; filename="act_%s.%s"`, safeActNumber, doc.Format,
	))
	c.File(absPath)
}
