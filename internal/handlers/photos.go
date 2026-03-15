package handlers

import (
	"fmt"
	"inspection-app/internal/cloudstorage"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// cloudStore — глобальный экземпляр облачного хранилища.
var cloudStore cloudstorage.FileStorage

// SetCloudStorage инициализирует облачное хранилище для обработчиков фото.
func SetCloudStorage(s cloudstorage.FileStorage) {
	cloudStore = s
}

// PostUploadPhoto обрабатывает POST /defects/:id/photos
// Сохраняет фото локально; синхронизация с облаком происходит при генерации PDF.
func PostUploadPhoto(c *gin.Context) {
	defectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID дефекта"})
		return
	}

	var defect models.RoomDefect
	if err := storage.DB.First(&defect, defectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Дефект не найден"})
		return
	}

	var room models.InspectionRoom
	if err := storage.DB.First(&room, defect.RoomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Помещение не найдено"})
		return
	}

	var inspection models.Inspection
	if err := storage.DB.First(&inspection, room.InspectionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return
	}

	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не найден в запросе (поле: photo)"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Допустимые форматы: jpg, jpeg, png, webp"})
		return
	}

	// Определяем локальный путь
	var photoCount int64
	storage.DB.Model(&models.Photo{}).Where("defect_id = ?", defectID).Count(&photoCount)
	fileName := fmt.Sprintf("photo_%d%s", photoCount+1, ext)

	localDir := filepath.Join("web", "static", "uploads", "photos",
		strconv.Itoa(int(inspection.ID)), strconv.Itoa(defectID))
	if err := os.MkdirAll(localDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания директории: " + err.Error()})
		return
	}

	localFile := filepath.Join(localDir, fileName)
	dst, err := os.Create(localFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания файла: " + err.Error()})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка записи файла: " + err.Error()})
		return
	}

	absPath, _ := filepath.Abs(localFile)
	staticURL := "/static/uploads/photos/" +
		strconv.Itoa(int(inspection.ID)) + "/" + strconv.Itoa(defectID) + "/" + fileName

	photo := models.Photo{
		DefectID: uint(defectID),
		FileURL:  staticURL,
		FilePath: absPath,
		FileName: fileName,
	}
	if err := storage.DB.Create(&photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       photo.ID,
		"url":      photo.FileURL,
		"filename": photo.FileName,
	})
}

// DeletePhoto обрабатывает POST /photos/:id/delete
func DeletePhoto(c *gin.Context) {
	photoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID фото"})
		return
	}

	var photo models.Photo
	if err := storage.DB.First(&photo, photoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Фото не найдено"})
		return
	}

	// Проверяем права через дефект → помещение → осмотр
	var defect models.RoomDefect
	storage.DB.First(&defect, photo.DefectID)
	var room models.InspectionRoom
	storage.DB.First(&room, defect.RoomID)
	var inspection models.Inspection
	storage.DB.First(&inspection, room.InspectionID)

	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	// Удаляем локальный файл
	if photo.FilePath != "" {
		os.Remove(photo.FilePath)
	}

	storage.DB.Delete(&photo)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// sectionFolderName возвращает читаемое название папки для секции дефекта.
func sectionFolderName(section string, wallNumber int) string {
	switch section {
	case "window":
		return "Окна"
	case "ceiling":
		return "Потолок"
	case "wall":
		if wallNumber > 0 {
			return fmt.Sprintf("Стены/Стена_%d", wallNumber)
		}
		return "Стены"
	case "floor":
		return "Пол"
	case "door":
		return "Двери"
	case "plumbing":
		return "Сантехника"
	default:
		return section
	}
}

// SyncInspectionPhotos загружает все локальные фото осмотра на Яндекс Диск.
// Вызывается перед генерацией PDF.
func SyncInspectionPhotos(inspectionID uint) {
	if cloudStore == nil {
		return
	}

	// Получаем все фото осмотра с локальным путём
	var photos []models.Photo
	storage.DB.
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.file_path != ''", inspectionID).
		Find(&photos)

	if len(photos) == 0 {
		return
	}

	// Собираем данные дефектов для формирования путей
	type defectInfo struct {
		RoomName   string
		RoomNumber int
		Section    string
		WallNumber int
		DefectName string
	}
	infoMap := map[uint]defectInfo{}

	var defects []models.RoomDefect
	storage.DB.
		Preload("DefectTemplate").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ?", inspectionID).
		Find(&defects)

	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", inspectionID).Find(&rooms)
	roomMap := map[uint]models.InspectionRoom{}
	for _, r := range rooms {
		roomMap[r.ID] = r
	}
	for _, d := range defects {
		r := roomMap[d.RoomID]
		name := d.DefectTemplate.Name
		if d.DefectTemplateID == nil || name == "" {
			name = "Прочее"
		}
		infoMap[d.ID] = defectInfo{
			RoomName:   r.RoomName,
			RoomNumber: r.RoomNumber,
			Section:    d.Section,
			WallNumber: d.WallNumber,
			DefectName: name,
		}
	}

	// Счётчик порядкового номера фото для каждого дефекта
	defectPhotoCount := map[uint]int{}

	for i := range photos {
		p := &photos[i]
		info, ok := infoMap[p.DefectID]
		if !ok {
			continue
		}

		defectPhotoCount[p.DefectID]++
		n := defectPhotoCount[p.DefectID]
		ext := filepath.Ext(p.FileName)

		roomName := sanitizeFolderName(info.RoomName)
		if roomName == "" {
			roomName = fmt.Sprintf("Помещение_%d", info.RoomNumber)
		}
		secFolder := sectionFolderName(info.Section, info.WallNumber)
		relFolder := fmt.Sprintf("inspections/%d/%s/%s", inspectionID, roomName, secFolder)

		defectName := sanitizeFolderName(info.DefectName)
		if defectName == "" {
			defectName = "фото"
		}
		cloudFileName := fmt.Sprintf("%s_%d%s", defectName, n, ext)
		relFile := relFolder + "/" + cloudFileName

		if err := cloudStore.EnsurePath(relFolder); err != nil {
			log.Printf("SyncPhotos EnsurePath %q: %v", relFolder, err)
			continue
		}

		f, err := os.Open(p.FilePath)
		if err != nil {
			log.Printf("SyncPhotos open %q: %v", p.FilePath, err)
			continue
		}

		if err := cloudStore.UploadFile(relFile, f); err != nil {
			f.Close()
			log.Printf("SyncPhotos upload %q: %v", relFile, err)
			continue
		}
		f.Close()

		publicURL, err := cloudStore.PublishFile(relFile)
		if err != nil {
			log.Printf("SyncPhotos publish %q: %v", relFile, err)
		}

		// Удаляем локальный файл после успешной загрузки
		os.Remove(p.FilePath)

		// Обновляем запись в БД
		storage.DB.Model(p).Updates(map[string]interface{}{
			"file_url":  publicURL,
			"file_path": "",
		})
	}

	// Публикуем папку осмотра, если ещё нет ссылки
	var inspection models.Inspection
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		return
	}
	if inspection.PhotoFolderURL == "" {
		inspFolder := fmt.Sprintf("inspections/%d", inspectionID)
		folderURL, err := cloudStore.PublishFolder(inspFolder)
		if err != nil {
			log.Printf("SyncPhotos PublishFolder %q: %v", inspFolder, err)
		} else if folderURL != "" {
			storage.DB.Model(&inspection).Update("photo_folder_url", folderURL)
		}
	}
}

// sanitizeFolderName заменяет символы, небезопасные для имён папок, на подчёркивание.
func sanitizeFolderName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return strings.TrimSpace(replacer.Replace(name))
}
