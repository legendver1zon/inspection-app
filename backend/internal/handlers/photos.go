package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"inspection-app/internal/cloudstorage"
	"inspection-app/internal/locker"
	"inspection-app/internal/logger"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"inspection-app/internal/thumbs"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxPhotoSize       = 20 * 1024 * 1024 // 20 MB
	maxPhotosPerDefect = 30
	syncWorkers        = 3 // макс параллельных загрузок на Yandex Disk
	uploadRetries      = 3
	maxFailRetries     = 5 // макс попыток для одного фото
)

// cloudStore — глобальный экземпляр облачного хранилища.
var cloudStore cloudstorage.FileStorage

// uploadLocker — блокировка параллельной обработки одной инспекции.
var uploadLocker locker.Locker = locker.NewMemory()

// SetCloudStorage инициализирует облачное хранилище для обработчиков фото.
func SetCloudStorage(s cloudstorage.FileStorage) {
	cloudStore = s
}

// SetUploadLocker заменяет реализацию блокировки (по умолчанию MemoryLocker).
// Вызывать в main.go для замены на RedisLocker при масштабировании.
func SetUploadLocker(l locker.Locker) {
	uploadLocker = l
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

	file, ext, ok := readPhotoUpload(c)
	if !ok {
		return
	}
	defer file.Close()

	photo, ok := storePhoto(c, file, ext, models.Photo{InspectionID: inspection.ID, Kind: models.PhotoKindDefect, DefectID: &defect.ID})
	if !ok {
		return
	}
	c.JSON(http.StatusOK, photoJSON(photo, false))
}

var (
	photoSections = map[string]bool{"window": true, "ceiling": true, "wall": true, "floor": true, "door": true, "plumbing": true}
	// Секции фото без дефекта: «overview» — общий вид помещения, остальные — общие замечания по квартире
	generalPhotoKinds = map[string]bool{models.PhotoKindElectricity: true, models.PhotoKindVentilation: true, models.PhotoKindGeneral: true}
	clientIDRe        = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// PostUploadInspectionPhoto обрабатывает POST /inspections/:id/photos.
// Фото привязывается не к id дефекта (он меняется при каждом сохранении формы),
// а к ключу room_number|section|template_id|wall_number: дефект находится по ключу
// или создаётся пустым. client_id делает загрузку идемпотентной для офлайн-очереди.
func PostUploadInspectionPhoto(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}

	bad := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"error": msg}) }

	clientID := strings.TrimSpace(c.PostForm("client_id"))
	if !clientIDRe.MatchString(clientID) {
		bad("client_id обязателен: до 64 символов из A-Z, a-z, 0-9, _ и -")
		return
	}
	section := c.PostForm("section")
	kind := models.PhotoKindDefect
	switch {
	case photoSections[section]:
	case section == "overview":
		kind = models.PhotoKindRoom
	case generalPhotoKinds[section]:
		kind = section
	default:
		bad("Неверная секция фото")
		return
	}
	var err error
	roomNumber := 0
	if s := strings.TrimSpace(c.PostForm("room_number")); s != "" {
		if roomNumber, err = strconv.Atoi(s); err != nil || roomNumber < 0 {
			bad("Неверный номер помещения")
			return
		}
	}
	if generalPhotoKinds[kind] {
		if roomNumber != 0 {
			bad("Фото общих замечаний не привязывается к помещению")
			return
		}
	} else if roomNumber < 1 {
		bad("Неверный номер помещения")
		return
	}
	var templateID *uint
	if s := strings.TrimSpace(c.PostForm("template_id")); s != "" && s != "0" {
		if kind != models.PhotoKindDefect {
			bad("template_id указывается только для фото дефекта")
			return
		}
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			bad("Неверный template_id")
			return
		}
		var tmpl models.DefectTemplate
		if err := storage.DB.First(&tmpl, uint(v)).Error; err != nil {
			bad("Шаблон дефекта не найден")
			return
		}
		if tmpl.Section != section {
			bad("Шаблон дефекта относится к другой секции")
			return
		}
		templateID = &tmpl.ID
	}
	wallNumber := 0
	if s := strings.TrimSpace(c.PostForm("wall_number")); s != "" {
		if wallNumber, err = strconv.Atoi(s); err != nil || wallNumber < 0 || wallNumber > 4 {
			bad("Номер стены должен быть от 1 до 4")
			return
		}
	}
	switch {
	case section == "wall" && templateID != nil && wallNumber == 0:
		bad("Для дефекта стены укажите номер стены (1–4)")
		return
	case (section != "wall" || templateID == nil) && wallNumber != 0:
		bad("Номер стены указывается только для дефектов стен")
		return
	}

	if existing, found := findPhotoByClientID(clientID); found {
		if !photoInInspection(existing, inspection.ID) {
			c.JSON(http.StatusConflict, gin.H{"error": "client_id уже используется в другом осмотре"})
			return
		}
		c.JSON(http.StatusOK, photoJSON(existing, true))
		return
	}

	file, ext, ok := readPhotoUpload(c)
	if !ok {
		return
	}
	defer file.Close()

	photo := models.Photo{InspectionID: inspection.ID, Kind: kind, ClientID: &clientID}
	switch kind {
	case models.PhotoKindRoom:
		if !roomExists(inspection.ID, roomNumber) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Помещение не найдено"})
			return
		}
		photo.RoomNumber = roomNumber
	case models.PhotoKindDefect:
		var room models.InspectionRoom
		if err := storage.DB.Where("inspection_id = ? AND room_number = ?", inspection.ID, roomNumber).
			Order("id desc").First(&room).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Помещение не найдено"})
			return
		}

		q := storage.DB.Where("room_id = ? AND section = ? AND wall_number = ?", room.ID, section, wallNumber)
		if templateID == nil {
			q = q.Where("defect_template_id IS NULL")
		} else {
			q = q.Where("defect_template_id = ?", *templateID)
		}
		var defect models.RoomDefect
		if err := q.Order("id").First(&defect).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Ctx(c.Request.Context()).Error("defect lookup failed", "inspection_id", inspection.ID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка поиска дефекта"})
				return
			}
			defect = models.RoomDefect{RoomID: room.ID, DefectTemplateID: templateID, Section: section, WallNumber: wallNumber}
			if err := storage.DB.Create(&defect).Error; err != nil {
				logger.Ctx(c.Request.Context()).Error("defect create failed", "inspection_id", inspection.ID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания дефекта"})
				return
			}
		}
		photo.DefectID = &defect.ID
	}

	photo, ok = storePhoto(c, file, ext, photo)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, photoJSON(photo, false))
}

func roomExists(inspectionID uint, roomNumber int) bool {
	var n int64
	storage.DB.Model(&models.InspectionRoom{}).Where("inspection_id = ? AND room_number = ?", inspectionID, roomNumber).Count(&n)
	return n > 0
}

func photoJSON(p models.Photo, duplicate bool) gin.H {
	h := gin.H{"id": p.ID, "url": p.FileURL, "filename": p.FileName, "defect_id": p.DefectID}
	if duplicate {
		h["duplicate"] = true
	}
	return h
}

// findPhotoByClientID ищет фото по client_id, включая удалённые: уникальный
// индекс держит и их, а повтор из очереди клиента — та же самая загрузка.
func findPhotoByClientID(clientID string) (models.Photo, bool) {
	var p models.Photo
	err := storage.DB.Unscoped().Where("client_id = ?", clientID).First(&p).Error
	return p, err == nil
}

func photoInInspection(p models.Photo, inspectionID uint) bool {
	return p.InspectionID == inspectionID
}

// readPhotoUpload берёт файл из поля photo и проверяет размер и расширение.
// При ошибке сам пишет 400 и возвращает ok=false; файл закрывает вызывающий.
func readPhotoUpload(c *gin.Context) (multipart.File, string, bool) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не найден в запросе (поле: photo)"})
		return nil, "", false
	}
	if header.Size > maxPhotoSize {
		file.Close()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл слишком большой (максимум 20 МБ)"})
		return nil, "", false
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		file.Close()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Допустимые форматы: jpg, jpeg, png, webp"})
		return nil, "", false
	}
	return file, ext, true
}

// storePhoto кладёт файл в uploads/photos/{inspection}/{группа}, создаёт
// запись Photo, строит миниатюру в фоне и ставит осмотр в очередь облака.
// Группа — дефект, помещение (общий вид) или вид общего замечания.
// Если ответ уже отправлен (ошибка или повтор по client_id) — возвращает ok=false.
func storePhoto(c *gin.Context, file io.Reader, ext string, photo models.Photo) (models.Photo, bool) {
	var photoCount int64
	photoGroup(storage.DB.Model(&models.Photo{}), photo).Count(&photoCount)
	if photoCount >= maxPhotosPerDefect {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Максимум %d фото на дефект", maxPhotosPerDefect)})
		return models.Photo{}, false
	}

	inspStr := strconv.FormatUint(uint64(photo.InspectionID), 10)
	group := photoGroupKey(photo)
	// Уникальное имя через timestamp — исключает race condition при одновременной загрузке
	fileName := fmt.Sprintf("photo_%s_%d%s", group, time.Now().UnixMilli(), ext)

	localDir := filepath.Join("web", "static", "uploads", "photos", inspStr, group)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания директории: " + err.Error()})
		return models.Photo{}, false
	}

	localFile := filepath.Join(localDir, fileName)
	dst, err := os.Create(localFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания файла: " + err.Error()})
		return models.Photo{}, false
	}
	_, copyErr := io.Copy(dst, file)
	dst.Close()
	if copyErr != nil {
		os.Remove(localFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка записи файла: " + copyErr.Error()})
		return models.Photo{}, false
	}

	absPath, _ := filepath.Abs(localFile)
	photo.FileURL = "/static/uploads/photos/" + inspStr + "/" + group + "/" + fileName
	photo.FilePath = absPath
	photo.FileName = fileName
	photo.UploadStatus = "pending"
	if err := storage.DB.Create(&photo).Error; err != nil {
		os.Remove(localFile)
		// Два параллельных запроса с одним client_id: первый уже сохранил фото
		if photo.ClientID != nil && isUniqueConflict(err, "client_id") {
			if existing, found := findPhotoByClientID(*photo.ClientID); found {
				c.JSON(http.StatusOK, photoJSON(existing, true))
				return existing, false
			}
		}
		logger.Ctx(c.Request.Context()).Error("photo create failed", "inspection_id", photo.InspectionID, "group", group, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения записи"})
		return models.Photo{}, false
	}

	go makeThumbAsync(photo)

	inspectionID := photo.InspectionID
	if cloudStore != nil {
		if uploadQueue != nil {
			if err := uploadQueue.Push(context.Background(), inspectionID); err != nil {
				logger.Ctx(c.Request.Context()).Error("redis push failed, fallback sync", "inspection_id", inspectionID, "error", err)
				ScheduleSync(inspectionID)
			}
		} else {
			ScheduleSync(inspectionID)
		}
	}
	return photo, true
}

// photoGroup ограничивает запрос фотографиями той же группы, что и p:
// того же дефекта, общего вида того же помещения или того же вида замечаний.
func photoGroup(db *gorm.DB, p models.Photo) *gorm.DB {
	switch p.Kind {
	case models.PhotoKindRoom:
		return db.Where("inspection_id = ? AND kind = ? AND room_number = ?", p.InspectionID, p.Kind, p.RoomNumber)
	case models.PhotoKindDefect:
		return db.Where("defect_id = ?", p.DefectID)
	default:
		return db.Where("inspection_id = ? AND kind = ?", p.InspectionID, p.Kind)
	}
}

// photoGroupKey — имя группы для каталога на диске и нумерации в облаке.
func photoGroupKey(p models.Photo) string {
	switch p.Kind {
	case models.PhotoKindRoom:
		return "room" + strconv.Itoa(p.RoomNumber)
	case models.PhotoKindDefect:
		if p.DefectID == nil {
			return "0"
		}
		return strconv.FormatUint(uint64(*p.DefectID), 10)
	default:
		return p.Kind
	}
}

// loadPhotoInspection загружает осмотр фото для проверки прав доступа.
// У записей до миграции inspection_id пуст — идём по цепочке
// фото → дефект → помещение → осмотр (Unscoped: архивные дефекты сохраняют фото).
// При обрыве цепочки отвечает 404 и возвращает ok=false.
func loadPhotoInspection(c *gin.Context, photo *models.Photo) (models.Inspection, bool) {
	var inspection models.Inspection
	inspectionID := photo.InspectionID
	if inspectionID == 0 {
		if photo.DefectID == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
			return inspection, false
		}
		var defect models.RoomDefect
		if err := storage.DB.Unscoped().First(&defect, *photo.DefectID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Дефект не найден"})
			return inspection, false
		}
		var room models.InspectionRoom
		if err := storage.DB.Unscoped().First(&room, defect.RoomID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Помещение не найдено"})
			return inspection, false
		}
		inspectionID = room.InspectionID
	}
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Осмотр не найден"})
		return inspection, false
	}
	return inspection, true
}

// authorizePhoto загружает фото по :id и проверяет, что запрос делает владелец
// осмотра или admin. При отказе сам пишет ответ и возвращает ok=false.
func authorizePhoto(c *gin.Context) (models.Photo, bool) {
	var photo models.Photo
	photoID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID фото"})
		return photo, false
	}
	if err := storage.DB.First(&photo, photoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Фото не найдено"})
		return photo, false
	}
	inspection, ok := loadPhotoInspection(c, &photo)
	if !ok {
		return photo, false
	}
	userID := c.GetUint("userID")
	role := c.GetString("userRole")
	if role != "admin" && inspection.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return photo, false
	}
	return photo, true
}

// localPhotoPath возвращает абсолютный путь локального оригинала, если он лежит
// внутри каталога загрузок и существует.
func localPhotoPath(ctx context.Context, photo *models.Photo) (string, bool) {
	if photo.FilePath == "" {
		return "", false
	}
	absPath, err := filepath.Abs(photo.FilePath)
	uploadsDir, dirErr := filepath.Abs(filepath.Join("web", "static", "uploads"))
	if err != nil || dirErr != nil || !strings.HasPrefix(absPath, uploadsDir+string(os.PathSeparator)) {
		logger.Ctx(ctx).Error("photo path outside uploads dir", "photo_id", photo.ID, "path", photo.FilePath)
		return "", false
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", false
	}
	return absPath, true
}

// DeletePhoto обрабатывает POST /photos/:id/delete
func DeletePhoto(c *gin.Context) {
	photo, ok := authorizePhoto(c)
	if !ok {
		return
	}

	if photo.FilePath != "" {
		if err := os.Remove(photo.FilePath); err != nil && !os.IsNotExist(err) {
			logger.Ctx(c.Request.Context()).Warn("не удалось удалить файл фото", "path", photo.FilePath, "error", err)
		}
	}
	if err := os.Remove(thumbs.Path(photo.ID)); err != nil && !os.IsNotExist(err) {
		logger.Ctx(c.Request.Context()).Warn("не удалось удалить миниатюру", "photo_id", photo.ID, "error", err)
	}

	storage.DB.Delete(&photo)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetPhotoDownload — GET /photos/:id/download
// Проксирует скачивание фото: если файл локальный — отдаёт напрямую,
// если в облаке — редиректит на временный URL скачивания.
func GetPhotoDownload(c *gin.Context) {
	photo, ok := authorizePhoto(c)
	if !ok {
		return
	}

	// 1. Локальный файл — отдаём напрямую (только из каталога загрузок)
	if absPath, ok := localPhotoPath(c.Request.Context(), &photo); ok {
		c.File(absPath)
		return
	}

	// 2. Облачный файл — URL начинается с http
	if strings.HasPrefix(photo.FileURL, "http") {
		c.Redirect(http.StatusTemporaryRedirect, photo.FileURL)
		return
	}

	// 3. Облачный файл — относительный путь на Yandex Disk
	if cloudStore != nil && photo.FileURL != "" && !strings.HasPrefix(photo.FileURL, "/static/") {
		downloadURL, err := cloudStore.GetDownloadURL(photo.FileURL)
		if err != nil {
			logger.Ctx(c.Request.Context()).Error("cloud download URL", "photo_id", photo.ID, "error", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Ошибка получения ссылки из облака"})
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, downloadURL)
		return
	}

	// 4. Локальный static URL (legacy)
	if strings.HasPrefix(photo.FileURL, "/static/") {
		c.Redirect(http.StatusTemporaryRedirect, photo.FileURL)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Файл недоступен"})
}

// safeSync запускает SyncInspectionPhotos с recover, чтобы паника в горутине
// не уронила всё приложение.
func safeSync(inspectionID uint) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("SyncInspectionPhotos panic", "inspection_id", inspectionID, "error", r)
		}
	}()
	SyncInspectionPhotos(inspectionID)
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
	case "overview":
		return "Общий_вид"
	case models.PhotoKindElectricity:
		return "Электричество"
	case models.PhotoKindVentilation:
		return "Вентиляция"
	case models.PhotoKindGeneral:
		return "Общие"
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
// Папка называется inspections/{ActNumber}. Если существует старая папка inspections/{ID},
// автоматически переименовывает её. Вызывается перед генерацией PDF и при сохранении акта.
// Возвращает публичную ссылку на папку (сохраняется в inspection.PhotoFolderURL).
func EnsureInspectionFolder(inspectionID uint) (string, error) {
	if cloudStore == nil {
		return "", nil
	}
	var inspection models.Inspection
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		return "", err
	}

	actNumber := sanitizeFolderName(inspection.ActNumber)
	if actNumber == "" {
		actNumber = fmt.Sprintf("%d", inspectionID) // fallback для старых записей
	}
	actFolder := fmt.Sprintf("inspections/%s", actNumber)
	idFolder := fmt.Sprintf("inspections/%d", inspectionID)

	// Ранний выход: если URL уже установлен и старая ID-папка не существует — всё готово
	if inspection.PhotoFolderURL != "" {
		oldExists, err := cloudStore.FolderExists(idFolder)
		if err != nil {
			logger.Warn("EnsureInspectionFolder FolderExists", "path", idFolder, "error", err)
		}
		if !oldExists {
			return inspection.PhotoFolderURL, nil
		}
		// Старая папка ещё существует → нужна миграция (продолжаем)
	}

	// Автомиграция: если есть папка inspections/{ID} и нет inspections/{ActNumber} — переименовываем
	if actNumber != fmt.Sprintf("%d", inspectionID) {
		oldExists, _ := cloudStore.FolderExists(idFolder)
		newExists, _ := cloudStore.FolderExists(actFolder)
		if oldExists && !newExists {
			if err := cloudStore.MoveFolder(idFolder, actFolder); err != nil {
				logger.Error("EnsureInspectionFolder MoveFolder", "from", idFolder, "to", actFolder, "error", err)
				// Не прерываем — пробуем создать заново
			} else {
				logger.Info("EnsureInspectionFolder moved", "from", idFolder, "to", actFolder)
			}
		}
	}

	if err := cloudStore.EnsurePath(actFolder); err != nil {
		return "", fmt.Errorf("EnsureInspectionFolder EnsurePath: %w", err)
	}
	folderURL, err := cloudStore.PublishFolder(actFolder)
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
// Блокировка по inspectionID гарантирует, что два воркера не обработают одну инспекцию одновременно.
func UploadInspectionPhotos(inspectionID uint) {
	if cloudStore == nil {
		return
	}

	// Блокируем инспекцию — второй воркер подождёт
	lockKey := fmt.Sprintf("upload:inspection:%d", inspectionID)
	unlock, err := uploadLocker.Lock(lockKey)
	if err != nil {
		logger.Warn("upload lock busy, will retry later", "inspection_id", inspectionID, "error", err)
		return
	}
	defer unlock()

	startTime := time.Now()

	// Идемпотентность: берём только pending (не done, не uploading)
	var photos []models.Photo
	storage.DB.
		Where("photos.inspection_id = ? AND photos.upload_status IN ('pending','failed')", inspectionID).
		Where("photos.retry_count < ?", maxFailRetries).
		Find(&photos)

	if len(photos) == 0 {
		return
	}

	logger.Info("upload start", "inspection_id", inspectionID, "photos", len(photos))

	// Помечаем как "uploading" (только pending/failed → uploading, никогда done → uploading)
	ids := make([]uint, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	now := time.Now()
	storage.DB.Model(&models.Photo{}).
		Where("id IN ? AND upload_status IN ('pending','failed')", ids).
		Updates(map[string]interface{}{
			"upload_status":   "uploading",
			"last_attempt_at": now,
		})

	// Собираем данные осмотра для построения путей в облаке
	insInfo := buildInspectionInfo(inspectionID)

	// Считаем уже загруженные фото в каждой группе (done + uploading), чтобы не перезаписать файлы
	groupCount := map[string]int{}
	var existingPhotos []models.Photo
	storage.DB.
		Where("photos.inspection_id = ? AND photos.upload_status IN ('done','uploading') AND photos.id NOT IN ?", inspectionID, ids).
		Find(&existingPhotos)
	for _, dp := range existingPhotos {
		groupCount[photoGroupKey(dp)]++
	}

	// Фильтруем: пропускаем фото без локального файла
	var tasks []uploadTask
	for i := range photos {
		p := &photos[i]
		info, ok := photoInfo(*p, insInfo)
		if !ok {
			continue
		}

		// Защита от потери файлов: если файл отсутствует — не пытаемся upload
		if p.FilePath == "" {
			logger.Error("upload skip: file_path empty", "photo_id", p.ID, "inspection_id", inspectionID)
			storage.DB.Model(p).
				Where("upload_status != 'done'").
				Updates(map[string]interface{}{
					"upload_status": "failed",
					"last_error":    "file_path empty — файл потерян",
				})
			continue
		}
		if _, statErr := os.Stat(p.FilePath); os.IsNotExist(statErr) {
			logger.Error("upload skip: local file missing", "photo_id", p.ID, "path", p.FilePath)
			storage.DB.Model(p).
				Where("upload_status != 'done'").
				Updates(map[string]interface{}{
					"upload_status": "failed",
					"last_error":    "local file not found: " + p.FilePath,
				})
			continue
		}

		groupCount[photoGroupKey(*p)]++
		n := groupCount[photoGroupKey(*p)]
		tasks = append(tasks, buildUploadTask(p, info, n))
	}

	if len(tasks) == 0 {
		return
	}

	uploadTasksParallel(tasks, func(t uploadTask, success bool, uploadErr error) {
		now := time.Now()
		if success {
			// Идемпотентность: обновляем только если статус ещё не done
			res := storage.DB.Model(t.photo).
				Where("upload_status != 'done'").
				Updates(map[string]interface{}{
					"file_url":        t.relFile,
					"file_path":       "",
					"upload_status":   "done",
					"last_error":      "",
					"last_attempt_at": now,
				})
			if res.RowsAffected > 0 {
				os.Remove(t.filePath)
			}
		} else {
			errMsg := ""
			if uploadErr != nil {
				errMsg = uploadErr.Error()
				// Обрезаем слишком длинные сообщения (тело ответа API)
				if len(errMsg) > 500 {
					errMsg = errMsg[:500]
				}
			}
			// Не перезаписываем done → failed
			storage.DB.Model(t.photo).
				Where("upload_status != 'done'").
				Updates(map[string]interface{}{
					"upload_status":   "failed",
					"last_error":      errMsg,
					"last_attempt_at": now,
				})
		}
	})

	elapsed := time.Since(startTime)
	logger.Info("upload complete", "inspection_id", inspectionID, "photos", len(tasks), "duration", elapsed.Round(time.Millisecond))

	// Push обновление через WebSocket
	notifyUploadProgress(inspectionID)
}

// notifyUploadProgress собирает текущий статус фото и отправляет через WebSocket.
func notifyUploadProgress(inspectionID uint) {
	NotifyUploadStatus(inspectionID, BuildUploadStatusMap(inspectionID))
}

// BuildUploadStatusMap возвращает map со статусами загрузки фото для осмотра.
// Используется в GetUploadStatus (HTTP) и notifyUploadProgress (WebSocket).
func BuildUploadStatusMap(inspectionID uint) map[string]interface{} {
	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	storage.DB.Model(&models.Photo{}).
		Select("photos.upload_status as status, COUNT(*) as count").
		Where("photos.inspection_id = ?", inspectionID).
		Group("photos.upload_status").
		Scan(&rows)

	pending, uploading, done, failed := int64(0), int64(0), int64(0), int64(0)
	var total int64
	for _, r := range rows {
		switch r.Status {
		case "pending":
			pending = r.Count
		case "uploading":
			uploading = r.Count
		case "done":
			done = r.Count
		case "failed":
			failed = r.Count
		}
		total += r.Count
	}

	return map[string]interface{}{
		"total":     total,
		"pending":   pending,
		"uploading": uploading,
		"done":      done,
		"failed":    failed,
		"all_done":  pending == 0 && uploading == 0 && failed == 0,
	}
}

// SyncInspectionPhotos — синхронный fallback: загружает все фото и публикует папку.
// Используется когда Redis недоступен. Устанавливает upload_status = "pending" для
// всех фото с file_path != ”, затем вызывает UploadInspectionPhotos.
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
		Where("photos.inspection_id = ? AND photos.file_path != '' AND photos.upload_status != 'uploading' AND photos.deleted_at IS NULL", inspectionID).
		Pluck("photos.id", &pendingIDs)
	if len(pendingIDs) > 0 {
		storage.DB.Model(&models.Photo{}).Where("id IN ?", pendingIDs).Update("upload_status", "pending")
	}

	UploadInspectionPhotos(inspectionID)

	// Публикуем папку осмотра
	if _, err := EnsureInspectionFolder(inspectionID); err != nil {
		logger.Error("SyncInspectionPhotos EnsureFolder", "inspection_id", inspectionID, "error", err)
	}
}

// --- внутренние вспомогательные функции ---

type defectInfo struct {
	RoomName   string
	RoomNumber int
	Section    string
	WallNumber int
	DefectName string
	ActNumber  string // номер акта для именования папки на Яндекс Диске
}

type inspectionInfo struct {
	actNumber string
	defects   map[uint]defectInfo
	rooms     map[int]string // номер помещения → название
}

func buildInspectionInfo(inspectionID uint) inspectionInfo {
	info := inspectionInfo{defects: map[uint]defectInfo{}, rooms: map[int]string{}}

	// Получаем номер акта для именования папки
	var inspection models.Inspection
	if err := storage.DB.First(&inspection, inspectionID).Error; err != nil {
		logger.Warn("buildInspectionInfo: осмотр не найден", "inspection_id", inspectionID, "error", err)
	}
	actNumber := sanitizeFolderName(inspection.ActNumber)
	if actNumber == "" {
		actNumber = fmt.Sprintf("%d", inspectionID) // fallback для старых записей
	}
	info.actNumber = actNumber

	var defects []models.RoomDefect
	storage.DB.Unscoped().
		Preload("DefectTemplate").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ?", inspectionID).
		Find(&defects)

	var rooms []models.InspectionRoom
	storage.DB.Unscoped().Where("inspection_id = ?", inspectionID).Order("id").Find(&rooms)
	roomMap := map[uint]models.InspectionRoom{}
	for _, r := range rooms {
		roomMap[r.ID] = r
		// Название по номеру: актуальное помещение главнее архивного
		if _, ok := info.rooms[r.RoomNumber]; !ok || !r.DeletedAt.Valid {
			info.rooms[r.RoomNumber] = r.RoomName
		}
	}
	for _, d := range defects {
		r := roomMap[d.RoomID]
		name := d.DefectTemplate.Name
		if d.DefectTemplateID == nil || name == "" {
			name = "Прочее"
		}
		info.defects[d.ID] = defectInfo{
			RoomName:   r.RoomName,
			RoomNumber: r.RoomNumber,
			Section:    d.Section,
			WallNumber: d.WallNumber,
			DefectName: name,
			ActNumber:  actNumber,
		}
	}
	return info
}

// photoInfo описывает, куда в облаке кладётся фото: дефекта — рядом с дефектом,
// общего вида — в папку помещения, общих замечаний — в «Общие_замечания».
func photoInfo(p models.Photo, ins inspectionInfo) (defectInfo, bool) {
	switch p.Kind {
	case models.PhotoKindRoom:
		return defectInfo{
			RoomName: ins.rooms[p.RoomNumber], RoomNumber: p.RoomNumber,
			Section: "overview", DefectName: sectionFolderName("overview", 0), ActNumber: ins.actNumber,
		}, true
	case models.PhotoKindElectricity, models.PhotoKindVentilation, models.PhotoKindGeneral:
		return defectInfo{
			RoomName: "Общие_замечания", Section: p.Kind,
			DefectName: sectionFolderName(p.Kind, 0), ActNumber: ins.actNumber,
		}, true
	}
	if p.DefectID == nil {
		return defectInfo{}, false
	}
	d, ok := ins.defects[*p.DefectID]
	return d, ok
}

func buildUploadTask(p *models.Photo, info defectInfo, n int) uploadTask {
	ext := filepath.Ext(p.FileName)
	roomName := sanitizeFolderName(info.RoomName)
	if roomName == "" {
		roomName = fmt.Sprintf("Помещение_%d", info.RoomNumber)
	}
	secFolder := sectionFolderName(info.Section, info.WallNumber)
	relFolder := fmt.Sprintf("inspections/%s/%s/%s", info.ActNumber, roomName, secFolder)

	defectName := sanitizeFolderName(info.DefectName)
	if defectName == "" {
		defectName = "фото"
	}
	cloudFileName := fmt.Sprintf("%s_%d_id%d%s", defectName, n, p.ID, ext)
	return uploadTask{
		photo:     p,
		relFolder: relFolder,
		relFile:   relFolder + "/" + cloudFileName,
		filePath:  p.FilePath,
	}
}

// uploadTasksParallel выполняет загрузку задач параллельно (до syncWorkers горутин).
// callback вызывается для каждой задачи с результатом (success, error).
func uploadTasksParallel(tasks []uploadTask, callback func(uploadTask, bool, error)) {
	// Фаза 1: создаём ВСЕ папки последовательно (Яндекс Диск блокирует при параллельных mkdir)
	createdFolders := map[string]bool{}
	for _, t := range tasks {
		if createdFolders[t.relFolder] {
			continue
		}
		if err := cloudStore.EnsurePath(t.relFolder); err != nil {
			logger.Error("upload EnsurePath", "folder", t.relFolder, "error", err)
		}
		createdFolders[t.relFolder] = true
	}

	// Фаза 2: загружаем файлы параллельно (папки уже созданы)
	sem := make(chan struct{}, syncWorkers)
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t uploadTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fileStart := time.Now()

			data, err := os.ReadFile(t.filePath)
			if err != nil {
				logger.Error("upload read file", "photo_id", t.photo.ID, "path", t.filePath, "error", err)
				callback(t, false, fmt.Errorf("read local file: %w", err))
				return
			}

			var uploadErr error
			for attempt := 0; attempt < uploadRetries; attempt++ {
				uploadErr = cloudStore.UploadFile(t.relFile, bytes.NewReader(data))
				if uploadErr == nil {
					break
				}

				retryable := cloudstorage.IsRetryable(uploadErr)
				logger.Warn("upload attempt failed",
					"photo_id", t.photo.ID,
					"attempt", attempt+1,
					"max", uploadRetries,
					"file", t.relFile,
					"retryable", retryable,
					"error", uploadErr,
				)

				// Permanent ошибка (4xx кроме 429) — не повторяем
				if !retryable {
					break
				}

				if attempt < uploadRetries-1 {
					// Exponential backoff: 2s, 4s, 8s
					time.Sleep(time.Duration(1<<uint(attempt+1)) * time.Second)
				}
			}
			if uploadErr != nil {
				logger.Error("upload failed permanently",
					"photo_id", t.photo.ID,
					"file", t.relFile,
					"retryable", cloudstorage.IsRetryable(uploadErr),
					"error", uploadErr,
				)
				callback(t, false, uploadErr)
				return
			}

			elapsed := time.Since(fileStart)
			logger.Info("upload file ok",
				"photo_id", t.photo.ID,
				"file", t.relFile,
				"size_kb", len(data)/1024,
				"duration", elapsed.Round(time.Millisecond),
			)
			callback(t, true, nil)
		}(task)
	}
	wg.Wait()
}

// sanitizeFolderName заменяет символы, небезопасные для имён папок, на подчёркивание.
// Имя из одних точек/пробелов (".", "..") — ссылка на родительскую папку, а не имя:
// возвращаем пустую строку, чтобы сработал fallback вызывающего кода.
func sanitizeFolderName(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	clean := strings.TrimSpace(replacer.Replace(name))
	if strings.Trim(clean, ". ") == "" {
		return ""
	}
	return clean
}
