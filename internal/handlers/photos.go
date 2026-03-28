package handlers

import (
	"bytes"
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
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxPhotoSize       = 20 * 1024 * 1024 // 20 MB
	maxPhotosPerDefect = 30
	syncWorkers        = 5
	uploadRetries      = 3
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

	if header.Size > maxPhotoSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл слишком большой (максимум 20 МБ)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Допустимые форматы: jpg, jpeg, png, webp"})
		return
	}

	// Определяем локальный путь
	var photoCount int64
	storage.DB.Model(&models.Photo{}).Where("defect_id = ?", defectID).Count(&photoCount)
	if photoCount >= maxPhotosPerDefect {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Максимум %d фото на дефект", maxPhotosPerDefect)})
		return
	}
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
		DefectID:     uint(defectID),
		FileURL:      staticURL,
		FilePath:     absPath,
		FileName:     fileName,
		UploadStatus: "pending",
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

// uploadTask описывает одно фото для загрузки в облако.
type uploadTask struct {
	photo     *models.Photo
	relFolder string
	relFile   string
	filePath  string
}

// EnsureInspectionFolder создаёт и публикует корневую папку осмотра на Яндекс Диске.
// Вызывается синхронно перед генерацией PDF — быстро (2-3 запроса к облаку).
// Возвращает публичную ссылку на папку (сохраняется в inspection.PhotoFolderURL).
func EnsureInspectionFolder(inspectionID uint) (string, error) {
	if cloudStore == nil {
		return "", nil
	}
	var inspection models.Inspection
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		return "", err
	}
	if inspection.PhotoFolderURL != "" {
		return inspection.PhotoFolderURL, nil
	}
	inspFolder := fmt.Sprintf("inspections/%d", inspectionID)
	if err := cloudStore.EnsurePath(inspFolder); err != nil {
		return "", fmt.Errorf("EnsureInspectionFolder EnsurePath: %w", err)
	}
	folderURL, err := cloudStore.PublishFolder(inspFolder)
	if err != nil {
		return "", fmt.Errorf("EnsureInspectionFolder PublishFolder: %w", err)
	}
	if folderURL != "" {
		storage.DB.Model(&inspection).Update("photo_folder_url", folderURL)
	}
	return folderURL, nil
}

// UploadInspectionPhotos загружает фото с upload_status = "pending" на Яндекс Диск.
// Вызывается асинхронно из фонового воркера.
func UploadInspectionPhotos(inspectionID uint) {
	if cloudStore == nil {
		return
	}

	// Собираем только фото со статусом "pending"
	var photos []models.Photo
	storage.DB.
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'pending'", inspectionID).
		Find(&photos)

	if len(photos) == 0 {
		return
	}

	// Помечаем как "uploading"
	ids := make([]uint, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	storage.DB.Model(&models.Photo{}).Where("id IN ?", ids).Update("upload_status", "uploading")

	// Собираем данные дефектов для построения путей
	infoMap := buildDefectInfoMap(inspectionID)

	defectPhotoCount := map[uint]int{}
	var tasks []uploadTask
	for i := range photos {
		p := &photos[i]
		info, ok := infoMap[p.DefectID]
		if !ok {
			continue
		}
		defectPhotoCount[p.DefectID]++
		n := defectPhotoCount[p.DefectID]
		tasks = append(tasks, buildUploadTask(p, info, inspectionID, n))
	}

	uploadTasksParallel(tasks, func(t uploadTask, success bool, publicURL string) {
		if success {
			os.Remove(t.filePath)
			storage.DB.Model(t.photo).Updates(map[string]interface{}{
				"file_url":      publicURL,
				"file_path":     "",
				"upload_status": "done",
			})
		} else {
			storage.DB.Model(t.photo).Update("upload_status", "failed")
		}
	})
}

// SyncInspectionPhotos — синхронный fallback: загружает все фото и публикует папку.
// Используется когда Redis недоступен. Устанавливает upload_status = "pending" для
// всех фото с file_path != '', затем вызывает UploadInspectionPhotos.
func SyncInspectionPhotos(inspectionID uint) {
	if cloudStore == nil {
		return
	}
	// Переводим в pending все фото с локальным файлом (для совместимости со старыми записями).
	// Используем подзапрос, т.к. PostgreSQL не поддерживает JOIN в UPDATE через GORM.
	var pendingIDs []uint
	storage.DB.
		Table("photos").
		Select("photos.id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.file_path != '' AND photos.upload_status != 'uploading' AND photos.deleted_at IS NULL", inspectionID).
		Pluck("photos.id", &pendingIDs)
	if len(pendingIDs) > 0 {
		storage.DB.Model(&models.Photo{}).Where("id IN ?", pendingIDs).Update("upload_status", "pending")
	}

	UploadInspectionPhotos(inspectionID)

	// Публикуем папку осмотра
	if _, err := EnsureInspectionFolder(inspectionID); err != nil {
		log.Printf("SyncInspectionPhotos EnsureFolder: %v", err)
	}
}

// --- внутренние вспомогательные функции ---

type defectInfo struct {
	RoomName   string
	RoomNumber int
	Section    string
	WallNumber int
	DefectName string
}

func buildDefectInfoMap(inspectionID uint) map[uint]defectInfo {
	infoMap := map[uint]defectInfo{}

	var defects []models.RoomDefect
	storage.DB.Unscoped().
		Preload("DefectTemplate").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ?", inspectionID).
		Find(&defects)

	var rooms []models.InspectionRoom
	storage.DB.Unscoped().Where("inspection_id = ?", inspectionID).Find(&rooms)
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
	return infoMap
}

func buildUploadTask(p *models.Photo, info defectInfo, inspectionID uint, n int) uploadTask {
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
	return uploadTask{
		photo:     p,
		relFolder: relFolder,
		relFile:   relFolder + "/" + cloudFileName,
		filePath:  p.FilePath,
	}
}

// uploadTasksParallel выполняет загрузку задач параллельно (до syncWorkers горутин).
// callback вызывается для каждой задачи с результатом (success, publicURL).
func uploadTasksParallel(tasks []uploadTask, callback func(uploadTask, bool, string)) {
	createdFolders := map[string]bool{}
	var folderMu sync.Mutex

	sem := make(chan struct{}, syncWorkers)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t uploadTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			folderMu.Lock()
			needCreate := !createdFolders[t.relFolder]
			if needCreate {
				createdFolders[t.relFolder] = true
			}
			folderMu.Unlock()

			if needCreate {
				if err := cloudStore.EnsurePath(t.relFolder); err != nil {
					log.Printf("uploadTasks EnsurePath %q: %v", t.relFolder, err)
					folderMu.Lock()
					delete(createdFolders, t.relFolder)
					folderMu.Unlock()
					callback(t, false, "")
					return
				}
			}

			data, err := os.ReadFile(t.filePath)
			if err != nil {
				log.Printf("uploadTasks read %q: %v", t.filePath, err)
				callback(t, false, "")
				return
			}

			var uploadErr error
			for attempt := 0; attempt < uploadRetries; attempt++ {
				uploadErr = cloudStore.UploadFile(t.relFile, bytes.NewReader(data))
				if uploadErr == nil {
					break
				}
				log.Printf("uploadTasks upload attempt %d/%d %q: %v", attempt+1, uploadRetries, t.relFile, uploadErr)
				if attempt < uploadRetries-1 {
					time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
				}
			}
			if uploadErr != nil {
				log.Printf("uploadTasks upload failed %q: %v", t.relFile, uploadErr)
				callback(t, false, "")
				return
			}

			publicURL, err := cloudStore.PublishFile(t.relFile)
			if err != nil {
				log.Printf("uploadTasks publish %q: %v", t.relFile, err)
			}
			callback(t, true, publicURL)
		}(task)
	}
	wg.Wait()
}

// sanitizeFolderName заменяет символы, небезопасные для имён папок, на подчёркивание.
func sanitizeFolderName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return strings.TrimSpace(replacer.Replace(name))
}
