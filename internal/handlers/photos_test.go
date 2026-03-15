package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// --- Mock CloudStorage ---

type mockCloudStore struct {
	ensurePathErr  error
	uploadFileErr  error
	publishFileURL string
	publishFileErr error
	publishFolURL  string
	publishFolErr  error

	uploadedPaths []string
	ensuredPaths  []string
}

func (m *mockCloudStore) EnsurePath(p string) error {
	m.ensuredPaths = append(m.ensuredPaths, p)
	return m.ensurePathErr
}
func (m *mockCloudStore) UploadFile(p string, _ io.Reader) error {
	m.uploadedPaths = append(m.uploadedPaths, p)
	return m.uploadFileErr
}
func (m *mockCloudStore) PublishFile(_ string) (string, error) {
	return m.publishFileURL, m.publishFileErr
}
func (m *mockCloudStore) PublishFolder(_ string) (string, error) {
	return m.publishFolURL, m.publishFolErr
}

// --- Router with photo routes ---

func setupPhotoRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Static("/static", "../../web/static")

	protected := r.Group("/")
	protected.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("userRole", "inspector")
		c.Next()
	})
	protected.POST("/defects/:id/photos", PostUploadPhoto)
	protected.POST("/photos/:id/delete", DeletePhoto)
	return r
}

func setupPhotoRouterAsAdmin(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Static("/static", "../../web/static")

	protected := r.Group("/")
	protected.Use(func(c *gin.Context) {
		c.Set("userID", uint(99))
		c.Set("userRole", "admin")
		c.Next()
	})
	protected.POST("/defects/:id/photos", PostUploadPhoto)
	protected.POST("/photos/:id/delete", DeletePhoto)
	return r
}

// multipartPhoto создаёт multipart-тело с одним полем "photo".
func multipartPhoto(t *testing.T, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("photo", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	io.WriteString(fw, content)
	w.Close()
	return &buf, w.FormDataContentType()
}

// newDefectWithInspection создаёт цепочку: User → Inspection → Room → Defect.
func newDefectWithInspection(t *testing.T, ownerID uint) (models.Inspection, models.InspectionRoom, models.RoomDefect) {
	t.Helper()
	insp := newInspection(t, ownerID, "ул. Тест, 1", "Тестов Т.Т.", "draft", time.Now())

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Кухня"}
	if err := storage.DB.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}

	tmpl := models.DefectTemplate{Section: "ceiling", Name: "Трещина", OrderIndex: 1}
	storage.DB.Create(&tmpl)

	defect := models.RoomDefect{
		RoomID:           room.ID,
		DefectTemplateID: tmpl.ID,
		DefectTemplate:   tmpl,
		Section:          "ceiling",
		Value:            "2 мм",
	}
	if err := storage.DB.Create(&defect).Error; err != nil {
		t.Fatalf("create defect: %v", err)
	}
	return insp, room, defect
}

// --- Unit tests: sanitizeFolderName ---

func TestSanitizeFolderName_Clean(t *testing.T) {
	if got := sanitizeFolderName("Кухня"); got != "Кухня" {
		t.Errorf("got %q", got)
	}
}

func TestSanitizeFolderName_Slashes(t *testing.T) {
	got := sanitizeFolderName("a/b\\c")
	if strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Errorf("slashes not removed: %q", got)
	}
}

func TestSanitizeFolderName_SpecialChars(t *testing.T) {
	got := sanitizeFolderName("file*name?test\"<>|:")
	for _, ch := range []string{"*", "?", "\"", "<", ">", "|", ":"} {
		if strings.Contains(got, ch) {
			t.Errorf("char %q not removed from %q", ch, got)
		}
	}
}

func TestSanitizeFolderName_TrimSpaces(t *testing.T) {
	if got := sanitizeFolderName("  Кухня  "); got != "Кухня" {
		t.Errorf("spaces not trimmed: %q", got)
	}
}

// --- Unit tests: sectionFolderName ---

func TestSectionFolderName_AllSections(t *testing.T) {
	cases := []struct {
		section    string
		wallNumber int
		want       string
	}{
		{"window", 0, "Окна"},
		{"ceiling", 0, "Потолок"},
		{"floor", 0, "Пол"},
		{"door", 0, "Двери"},
		{"plumbing", 0, "Сантехника"},
		{"wall", 0, "Стены"},
		{"wall", 1, "Стены/Стена_1"},
		{"wall", 4, "Стены/Стена_4"},
		{"unknown", 0, "unknown"},
	}
	for _, c := range cases {
		got := sectionFolderName(c.section, c.wallNumber)
		if got != c.want {
			t.Errorf("sectionFolderName(%q,%d) = %q, want %q", c.section, c.wallNumber, got, c.want)
		}
	}
}

// --- HTTP: PostUploadPhoto ---

func TestPostUploadPhoto_InvalidDefectID(t *testing.T) {
	setupTestDB(t)
	r := setupPhotoRouter(t)

	body, ct := multipartPhoto(t, "test.jpg", "fake-image")
	req := httptest.NewRequest("POST", "/defects/abc/photos", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestPostUploadPhoto_DefectNotFound(t *testing.T) {
	setupTestDB(t)
	r := setupPhotoRouter(t)

	body, ct := multipartPhoto(t, "test.jpg", "fake-image")
	req := httptest.NewRequest("POST", "/defects/9999/photos", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestPostUploadPhoto_Forbidden(t *testing.T) {
	setupTestDB(t)
	// ID=1 займёт "текущий пользователь" middleware, владелец получит ID=2+
	newUser(t, "current@test.com", "pass1234", "Текущий", models.RoleInspector)
	owner := newUser(t, "owner@test.com", "pass1234", "Владелец", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, owner.ID)

	// Роутер с userID=1 (не владелец, не admin)
	r := setupPhotoRouter(t)

	body, ct := multipartPhoto(t, "test.jpg", "fake-image")
	req := httptest.NewRequest("POST", fmt.Sprintf("/defects/%d/photos", defect.ID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

func TestPostUploadPhoto_InvalidFormat(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "insp@test.com", "pass1234", "Инспектор", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, user.ID)

	// Роутер с userID = ID пользователя
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/defects/:id/photos", func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("userRole", "inspector")
		PostUploadPhoto(c)
	})

	body, ct := multipartPhoto(t, "test.exe", "fake-binary")
	req := httptest.NewRequest("POST", fmt.Sprintf("/defects/%d/photos", defect.ID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

func TestPostUploadPhoto_Success(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "insp2@test.com", "pass1234", "Инспектор 2", models.RoleInspector)
	insp, _, defect := newDefectWithInspection(t, user.ID)

	// Создаём временную директорию для загрузки (имитируем web/static/uploads)
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.MkdirAll(filepath.Join(tmpDir, "web", "static", "uploads", "photos"), 0755)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/defects/:id/photos", func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("userRole", "inspector")
		PostUploadPhoto(c)
	})

	// Переходим во временный каталог, чтобы файлы создавались там
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	body, ct := multipartPhoto(t, "photo.jpg", "fake-jpeg-content")
	req := httptest.NewRequest("POST", fmt.Sprintf("/defects/%d/photos", defect.ID), body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] == nil {
		t.Error("response missing 'id'")
	}
	if !strings.HasPrefix(resp["url"].(string), "/static/uploads/photos/") {
		t.Errorf("unexpected url: %v", resp["url"])
	}

	// Проверяем запись в БД
	var count int64
	storage.DB.Model(&models.Photo{}).Where("defect_id = ?", defect.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 photo in DB, got %d", count)
	}

	// Проверяем FilePath привязан к осмотру
	var photo models.Photo
	storage.DB.Where("defect_id = ?", defect.ID).First(&photo)
	if !strings.Contains(photo.FilePath, fmt.Sprintf("%d", insp.ID)) {
		t.Errorf("FilePath doesn't contain inspection ID: %s", photo.FilePath)
	}
	if photo.FilePath == "" {
		t.Error("FilePath should not be empty for local photo")
	}
}

func TestPostUploadPhoto_NoFile(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "insp3@test.com", "pass1234", "Инспектор 3", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, user.ID)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/defects/:id/photos", func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("userRole", "inspector")
		PostUploadPhoto(c)
	})

	// Запрос без файла
	req := httptest.NewRequest("POST", fmt.Sprintf("/defects/%d/photos", defect.ID), nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", w.Code)
	}
}

// --- HTTP: DeletePhoto ---

func TestDeletePhoto_NotFound(t *testing.T) {
	setupTestDB(t)
	r := setupPhotoRouter(t)

	req := httptest.NewRequest("POST", "/photos/9999/delete", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", w.Code)
	}
}

func TestDeletePhoto_Forbidden(t *testing.T) {
	setupTestDB(t)
	// ID=1 займёт "текущий пользователь" middleware, владелец получит ID=2+
	newUser(t, "current2@test.com", "pass1234", "Текущий", models.RoleInspector)
	owner := newUser(t, "owner2@test.com", "pass1234", "Владелец", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, owner.ID)

	// Создаём фото от имени владельца
	photo := models.Photo{DefectID: defect.ID, FileName: "p.jpg", FileURL: "/static/p.jpg", FilePath: "/tmp/p.jpg"}
	storage.DB.Create(&photo)

	// Роутер с userID=1 (чужой)
	r := setupPhotoRouter(t)

	req := httptest.NewRequest("POST", fmt.Sprintf("/photos/%d/delete", photo.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", w.Code)
	}
}

func TestDeletePhoto_Success_LocalFile(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "insp4@test.com", "pass1234", "Инспектор 4", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, user.ID)

	// Создаём временный файл
	tmp, err := os.CreateTemp("", "photo_*.jpg")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmp.Close()
	tmpPath := tmp.Name()

	photo := models.Photo{DefectID: defect.ID, FileName: "p.jpg", FilePath: tmpPath, FileURL: "/static/p.jpg"}
	storage.DB.Create(&photo)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/photos/:id/delete", func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("userRole", "inspector")
		DeletePhoto(c)
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/photos/%d/delete", photo.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200; body: %s", w.Code, w.Body.String())
	}

	// Проверяем что файл удалён с диска
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("local file should have been deleted")
	}

	// Проверяем что запись удалена из БД
	var count int64
	storage.DB.Model(&models.Photo{}).Where("id = ?", photo.ID).Count(&count)
	if count != 0 {
		t.Error("photo record should be deleted from DB")
	}
}

func TestDeletePhoto_AdminCanDeleteAnyPhoto(t *testing.T) {
	setupTestDB(t)
	owner := newUser(t, "owner3@test.com", "pass1234", "Владелец", models.RoleInspector)
	_, _, defect := newDefectWithInspection(t, owner.ID)

	photo := models.Photo{DefectID: defect.ID, FileName: "p.jpg", FileURL: "/static/p.jpg"}
	storage.DB.Create(&photo)

	r := setupPhotoRouterAsAdmin(t)

	req := httptest.NewRequest("POST", fmt.Sprintf("/photos/%d/delete", photo.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

// --- SyncInspectionPhotos ---

func TestSyncInspectionPhotos_NoCloudStore(t *testing.T) {
	setupTestDB(t)
	cloudStore = nil
	// Должен завершиться без паники
	SyncInspectionPhotos(999)
}

func TestSyncInspectionPhotos_NoLocalPhotos(t *testing.T) {
	setupTestDB(t)
	mock := &mockCloudStore{}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	user := newUser(t, "sync1@test.com", "pass1234", "Пользователь", models.RoleInspector)
	insp := newInspection(t, user.ID, "ул. Синк, 1", "Тест", "draft", time.Now())

	// Фото без FilePath (уже синхронизированы)
	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1}
	storage.DB.Create(&room)
	tmpl := newDefectTemplate(t, "floor", "Тест")
	defect := models.RoomDefect{RoomID: room.ID, DefectTemplateID: tmpl.ID, Section: "floor", Value: "да"}
	storage.DB.Create(&defect)
	photo := models.Photo{DefectID: defect.ID, FilePath: "", FileURL: "https://disk.yandex.ru/i/abc"}
	storage.DB.Create(&photo)

	SyncInspectionPhotos(insp.ID)

	// UploadFile не должен вызываться
	if len(mock.uploadedPaths) != 0 {
		t.Errorf("expected 0 uploads, got %d", len(mock.uploadedPaths))
	}
}

func TestSyncInspectionPhotos_UploadAndCleanup(t *testing.T) {
	setupTestDB(t)

	mock := &mockCloudStore{
		publishFileURL: "https://disk.yandex.ru/i/testfile",
		publishFolURL:  "https://disk.yandex.ru/d/testfolder",
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	user := newUser(t, "sync2@test.com", "pass1234", "Пользователь", models.RoleInspector)
	insp := newInspection(t, user.ID, "ул. Синк, 2", "Тест", "draft", time.Now())

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Кухня"}
	storage.DB.Create(&room)

	tmpl := models.DefectTemplate{Section: "ceiling", Name: "Трещина", OrderIndex: 1}
	storage.DB.Create(&tmpl)

	defect := models.RoomDefect{RoomID: room.ID, DefectTemplateID: tmpl.ID, DefectTemplate: tmpl, Section: "ceiling", Value: "3 мм"}
	storage.DB.Create(&defect)

	// Создаём реальный временный файл
	tmp, err := os.CreateTemp("", "photo_*.jpg")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmp.WriteString("fake-jpeg")
	tmp.Close()
	tmpPath := tmp.Name()

	photo := models.Photo{DefectID: defect.ID, FileName: "photo_1.jpg", FilePath: tmpPath, FileURL: "/static/uploads/photos/1/1/photo_1.jpg"}
	storage.DB.Create(&photo)

	SyncInspectionPhotos(insp.ID)

	// Файл должен быть удалён локально
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("local file should be deleted after sync")
	}

	// FilePath в БД должен быть пустым
	var updated models.Photo
	storage.DB.First(&updated, photo.ID)
	if updated.FilePath != "" {
		t.Errorf("FilePath should be empty after sync, got %q", updated.FilePath)
	}
	if updated.FileURL != "https://disk.yandex.ru/i/testfile" {
		t.Errorf("FileURL should be cloud URL, got %q", updated.FileURL)
	}

	// CloudPath должен содержать читаемые имена
	if len(mock.uploadedPaths) == 0 {
		t.Fatal("expected at least one upload")
	}
	uploadPath := mock.uploadedPaths[0]
	if !strings.Contains(uploadPath, "Кухня") {
		t.Errorf("upload path should contain room name 'Кухня', got %q", uploadPath)
	}
	if !strings.Contains(uploadPath, "Потолок") {
		t.Errorf("upload path should contain section 'Потолок', got %q", uploadPath)
	}
	if !strings.Contains(uploadPath, "Трещина") {
		t.Errorf("upload path should contain defect name 'Трещина', got %q", uploadPath)
	}

	// PhotoFolderURL должен быть установлен в БД
	var updatedInsp models.Inspection
	storage.DB.First(&updatedInsp, insp.ID)
	if updatedInsp.PhotoFolderURL != "https://disk.yandex.ru/d/testfolder" {
		t.Errorf("PhotoFolderURL not set, got %q", updatedInsp.PhotoFolderURL)
	}
}

func TestSyncInspectionPhotos_WallDefectPath(t *testing.T) {
	setupTestDB(t)

	mock := &mockCloudStore{
		publishFileURL: "https://disk.yandex.ru/i/wall",
		publishFolURL:  "https://disk.yandex.ru/d/folder",
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	user := newUser(t, "sync3@test.com", "pass1234", "Пользователь", models.RoleInspector)
	insp := newInspection(t, user.ID, "ул. Стен, 1", "Тест", "draft", time.Now())

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Гостиная"}
	storage.DB.Create(&room)

	tmpl := models.DefectTemplate{Section: "wall", Name: "Отклонение", OrderIndex: 1}
	storage.DB.Create(&tmpl)

	defect := models.RoomDefect{RoomID: room.ID, DefectTemplateID: tmpl.ID, DefectTemplate: tmpl, Section: "wall", WallNumber: 2, Value: "5 мм"}
	storage.DB.Create(&defect)

	tmp, _ := os.CreateTemp("", "wall_photo_*.jpg")
	tmp.WriteString("fake")
	tmp.Close()

	photo := models.Photo{DefectID: defect.ID, FileName: "photo_1.jpg", FilePath: tmp.Name()}
	storage.DB.Create(&photo)

	SyncInspectionPhotos(insp.ID)

	if len(mock.uploadedPaths) == 0 {
		t.Fatal("expected upload")
	}
	path := mock.uploadedPaths[0]
	if !strings.Contains(path, "Стена_2") {
		t.Errorf("wall path should contain 'Стена_2', got %q", path)
	}
	if !strings.Contains(path, "Гостиная") {
		t.Errorf("wall path should contain 'Гостиная', got %q", path)
	}

	os.Remove(tmp.Name())
}
