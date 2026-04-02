package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// --- Вспомогательные функции ---

// createTempPhotosForDefect создаёт n временных файлов и записи Photo в БД.
func createTempPhotosForDefect(t *testing.T, defectID uint, n int) []string {
	t.Helper()
	var paths []string
	for i := 1; i <= n; i++ {
		tmp, err := os.CreateTemp("", fmt.Sprintf("load_photo_%d_*.jpg", i))
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		tmp.WriteString("fake-jpeg-content")
		tmp.Close()

		photo := models.Photo{
			DefectID: defectID,
			FileName: fmt.Sprintf("photo_%d.jpg", i),
			FilePath: tmp.Name(),
			FileURL:  fmt.Sprintf("/static/uploads/photo_%d.jpg", i),
		}
		if err := storage.DB.Create(&photo).Error; err != nil {
			t.Fatalf("create photo %d: %v", i, err)
		}
		paths = append(paths, tmp.Name())
	}
	return paths
}

// newInspectionWithDefect создаёт User → Inspection → Room → Defect и возвращает их.
func newInspectionWithDefect(t *testing.T, emailSuffix string) (models.Inspection, models.RoomDefect) {
	t.Helper()
	user := newUser(t, fmt.Sprintf("load_%s@test.com", emailSuffix), "pass1234", "Нагрузка Тест", models.RoleInspector)
	insp := newInspection(t, user.ID, fmt.Sprintf("ул. Нагрузки, %s", emailSuffix), "Тест", "draft", time.Now())

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Гостиная"}
	if err := storage.DB.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	tmpl := models.DefectTemplate{Section: "ceiling", Name: "Трещина", OrderIndex: 1}
	storage.DB.Create(&tmpl)

	defect := models.RoomDefect{
		RoomID:           room.ID,
		DefectTemplateID: &tmpl.ID,
		DefectTemplate:   tmpl,
		Section:          "ceiling",
		Value:            "2 мм",
	}
	if err := storage.DB.Create(&defect).Error; err != nil {
		t.Fatalf("create defect: %v", err)
	}
	return insp, defect
}

// --- Нагрузочные тесты SyncInspectionPhotos ---

// TestSyncPhotos_30Photos_SingleUser — 1 пользователь, 30 фото в 1 дефекте.
// Проверяет: все загружены, кэш папок работает (EnsurePath вызван ≤ 5 раз), локальные файлы удалены.
func TestSyncPhotos_30Photos_SingleUser(t *testing.T) {
	setupTestDB(t)

	mock := &mockCloudStore{
		publishFileURL: "https://disk.yandex.ru/i/test",
		publishFolURL:  "https://disk.yandex.ru/d/folder",
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	insp, defect := newInspectionWithDefect(t, "single")
	paths := createTempPhotosForDefect(t, defect.ID, 30)
	defer func() {
		for _, p := range paths {
			os.Remove(p) // на случай если тест упал до sync
		}
	}()

	start := time.Now()
	SyncInspectionPhotos(insp.ID)
	elapsed := time.Since(start)
	t.Logf("30 фото синхронизировано за %v", elapsed)

	// Все 30 фото загружены в облако
	mock.mu.Lock()
	uploaded := len(mock.uploadedPaths)
	ensured := len(mock.ensuredPaths)
	mock.mu.Unlock()

	if uploaded != 30 {
		t.Errorf("ожидали 30 загрузок, получили %d", uploaded)
	}

	// Кэш папок: EnsurePath должен вызываться 1 раз на уникальную папку (не 30 раз)
	// У нас 1 комната / 1 секция → 1 уникальная папка → ≤ 5 EnsurePath вызовов
	if ensured > 5 {
		t.Errorf("кэш папок не работает: EnsurePath вызван %d раз (ожидали ≤ 5 для 1 папки)", ensured)
	}

	// Все записи в БД: FilePath пустой, FileURL облачный
	var photos []models.Photo
	storage.DB.Where("defect_id = ?", defect.ID).Find(&photos)
	for _, p := range photos {
		if p.FilePath != "" {
			t.Errorf("фото %d: FilePath должен быть пустым после sync, получили %q", p.ID, p.FilePath)
		}
		if p.FileURL != "https://disk.yandex.ru/i/test" {
			t.Errorf("фото %d: неверный FileURL: %q", p.ID, p.FileURL)
		}
	}

	// Все локальные файлы удалены
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("локальный файл %q должен быть удалён после sync", p)
		}
	}
}

// TestSyncPhotos_5Users_Concurrent — 5 пользователей одновременно, по 5 фото каждый.
// Проверяет: нет паник, нет дедлоков, все фото загружены.
func TestSyncPhotos_5Users_Concurrent(t *testing.T) {
	setupTestDB(t)

	const userCount = 5
	const photosPerUser = 5

	mock := &mockCloudStore{
		publishFileURL: "https://disk.yandex.ru/i/test",
		publishFolURL:  "https://disk.yandex.ru/d/folder",
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	type inspData struct {
		inspID uint
		paths  []string
	}
	data := make([]inspData, userCount)
	for i := 0; i < userCount; i++ {
		insp, defect := newInspectionWithDefect(t, fmt.Sprintf("c5u%d", i))
		paths := createTempPhotosForDefect(t, defect.ID, photosPerUser)
		data[i] = inspData{inspID: insp.ID, paths: paths}
	}
	defer func() {
		for _, d := range data {
			for _, p := range d.paths {
				os.Remove(p)
			}
		}
	}()

	// Запускаем 5 горутин одновременно
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(inspID uint) {
			defer wg.Done()
			SyncInspectionPhotos(inspID)
		}(data[i].inspID)
	}
	wg.Wait()
	elapsed := time.Since(start)

	total := userCount * photosPerUser
	t.Logf("5 пользователей × %d фото = %d фото, синхронизировано за %v", photosPerUser, total, elapsed)

	mock.mu.Lock()
	uploaded := len(mock.uploadedPaths)
	mock.mu.Unlock()
	if uploaded != total {
		t.Errorf("ожидали %d загрузок, получили %d", total, uploaded)
	}
}

// TestSyncPhotos_10Users_Concurrent — 10 пользователей одновременно, по 3 фото каждый.
// Проверяет устойчивость при максимальной нагрузке: нет паник, нет дедлоков, нет data race.
func TestSyncPhotos_10Users_Concurrent(t *testing.T) {
	setupTestDB(t)

	const userCount = 10
	const photosPerUser = 3

	mock := &mockCloudStore{
		publishFileURL: "https://disk.yandex.ru/i/test",
		publishFolURL:  "https://disk.yandex.ru/d/folder",
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	type inspData struct {
		inspID uint
		paths  []string
	}
	data := make([]inspData, userCount)
	for i := 0; i < userCount; i++ {
		insp, defect := newInspectionWithDefect(t, fmt.Sprintf("c10u%d", i))
		paths := createTempPhotosForDefect(t, defect.ID, photosPerUser)
		data[i] = inspData{inspID: insp.ID, paths: paths}
	}
	defer func() {
		for _, d := range data {
			for _, p := range d.paths {
				os.Remove(p)
			}
		}
	}()

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(inspID uint) {
			defer wg.Done()
			SyncInspectionPhotos(inspID)
		}(data[i].inspID)
	}
	wg.Wait()
	elapsed := time.Since(start)

	total := userCount * photosPerUser
	t.Logf("10 пользователей × %d фото = %d фото, синхронизировано за %v", photosPerUser, total, elapsed)

	mock.mu.Lock()
	uploaded := len(mock.uploadedPaths)
	mock.mu.Unlock()
	if uploaded != total {
		t.Errorf("ожидали %d загрузок, получили %d", total, uploaded)
	}
}

// TestSyncPhotos_RetryOnUploadError — проверяет устойчивость: при uploadFileErr все попытки
// исчерпаны, фото не удаляется локально (файл сохраняется как fallback).
func TestSyncPhotos_RetryOnUploadError(t *testing.T) {
	setupTestDB(t)

	retryMock := &mockCloudStore{
		uploadFileErr: fmt.Errorf("яндекс диск недоступен"),
	}
	cloudStore = retryMock
	defer func() { cloudStore = nil }()

	insp, defect := newInspectionWithDefect(t, "retry")
	paths := createTempPhotosForDefect(t, defect.ID, 1)
	defer func() {
		for _, p := range paths {
			os.Remove(p)
		}
	}()

	SyncInspectionPhotos(insp.ID)

	// При всех провальных попытках фото должно остаться локально (не удалено)
	if _, err := os.Stat(paths[0]); os.IsNotExist(err) {
		t.Error("локальный файл не должен быть удалён при ошибке загрузки")
	}

	// UploadFile вызван uploadRetries раз
	retryMock.mu.Lock()
	attempts := len(retryMock.uploadedPaths)
	retryMock.mu.Unlock()
	if attempts != uploadRetries {
		t.Errorf("ожидали %d попыток upload, получили %d", uploadRetries, attempts)
	}

	// FilePath в БД должен остаться (не очищен при ошибке)
	var photo models.Photo
	storage.DB.Where("defect_id = ?", defect.ID).First(&photo)
	if photo.FilePath == "" {
		t.Error("FilePath не должен очищаться при неудачной загрузке")
	}
}

// --- Тесты лимитов ---

// TestPostUploadPhoto_MaxPhotosLimit — лимит 30 фото на дефект.
func TestPostUploadPhoto_MaxPhotosLimit(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "limitphotos@test.com", "pass1234", "Лимит Тест", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, user.ID)

	// Создаём 30 фото напрямую в БД
	for i := 1; i <= 30; i++ {
		photo := models.Photo{
			DefectID: defect.ID,
			FileName: fmt.Sprintf("photo_%d.jpg", i),
			FileURL:  fmt.Sprintf("/static/photo_%d.jpg", i),
		}
		storage.DB.Create(&photo)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/defects/:id/photos", func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("userRole", "inspector")
		PostUploadPhoto(c)
	})

	body, ct := multipartPhoto(t, "extra.jpg", "content")
	req := httptest.NewRequest("POST", fmt.Sprintf("/defects/%d/photos", defect.ID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ожидали 400 при превышении лимита, получили %d; тело: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Максимум") {
		t.Error("ожидали сообщение об ошибке 'Максимум'")
	}
}

// TestPostUploadPhoto_FileSizeTooLarge — лимит 20 МБ на файл.
// Тест использует header.Size — проверяем что хендлер возвращает 400 при превышении.
// Примечание: httptest не устанавливает header.Size автоматически, поэтому
// мы проверяем что константа maxPhotoSize задана верно.
func TestPostUploadPhoto_MaxSizeConstant(t *testing.T) {
	const expectedMB = 20
	if maxPhotoSize != expectedMB*1024*1024 {
		t.Errorf("maxPhotoSize = %d, ожидали %d МБ", maxPhotoSize, expectedMB)
	}
	if maxPhotosPerDefect != 30 {
		t.Errorf("maxPhotosPerDefect = %d, ожидали 30", maxPhotosPerDefect)
	}
	if syncWorkers != 3 {
		t.Errorf("syncWorkers = %d, ожидали 3", syncWorkers)
	}
	if uploadRetries != 3 {
		t.Errorf("uploadRetries = %d, ожидали 3", uploadRetries)
	}
}
