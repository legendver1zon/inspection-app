package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- вспомогательная функция ---

func newDocument(t *testing.T, inspectionID, generatedBy uint) models.Document {
	t.Helper()
	doc := models.Document{
		InspectionID: inspectionID,
		Format:       "pdf",
		FilePath:     "",
		GeneratedBy:  generatedBy,
	}
	if err := storage.DB.Create(&doc).Error; err != nil {
		t.Fatalf("newDocument: %v", err)
	}
	return doc
}

// --- POST /documents/:id/delete ---

func TestPostDeleteDocument_NotFound(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/documents/99999/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("POST /documents/99999/delete: got %d, want 404", w.Code)
	}
}

func TestPostDeleteDocument_Forbidden(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	other := newUser(t, "other@test.com", "pass123", "Другой Другой Другой", models.RoleInspector)

	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())
	doc := newDocument(t, insp.ID, owner.ID)

	token := tokenFor(t, other.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/documents/"+itoa(doc.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("POST /documents/:id/delete forbidden: got %d, want 403", w.Code)
	}
}

func TestPostDeleteDocument_OwnerCanDelete(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())
	doc := newDocument(t, insp.ID, owner.ID)

	token := tokenFor(t, owner.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/documents/"+itoa(doc.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /documents/:id/delete owner: got %d, want 302", w.Code)
	}

	var count int64
	storage.DB.Model(&models.Document{}).Where("id = ?", doc.ID).Count(&count)
	if count != 0 {
		t.Error("document was not deleted")
	}
}

func TestPostDeleteDocument_AdminCanDelete(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)

	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())
	doc := newDocument(t, insp.ID, owner.ID)

	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/documents/"+itoa(doc.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /documents/:id/delete admin: got %d, want 302", w.Code)
	}
}

func TestPostDeleteDocument_DeletesPhysicalFile(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())

	// Создаём временный файл
	tmpFile, err := os.CreateTemp(t.TempDir(), "test_doc_*.pdf")
	if err != nil {
		t.Fatalf("cannot create temp file: %v", err)
	}
	tmpFile.Close()
	absPath, _ := filepath.Abs(tmpFile.Name())

	doc := models.Document{
		InspectionID: insp.ID,
		Format:       "pdf",
		FilePath:     absPath,
		GeneratedBy:  owner.ID,
	}
	storage.DB.Create(&doc)

	token := tokenFor(t, owner.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/documents/"+itoa(doc.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /documents/:id/delete with file: got %d, want 302", w.Code)
	}

	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Error("physical file was not deleted")
	}
}

// --- GET /documents/:id/download ---

func TestGetDownloadDocument_NotFound(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/documents/99999/download", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /documents/99999/download: got %d, want 404", w.Code)
	}
}

func TestGetDownloadDocument_Forbidden(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	other := newUser(t, "other@test.com", "pass123", "Другой Другой Другой", models.RoleInspector)

	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())
	doc := newDocument(t, insp.ID, owner.ID)

	token := tokenFor(t, other.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/documents/"+itoa(doc.ID)+"/download", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("GET /documents/:id/download forbidden: got %d, want 403", w.Code)
	}
}

func TestGetDownloadDocument_FileMissing_DeletesRecord(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())

	// Указываем несуществующий путь к файлу
	doc := models.Document{
		InspectionID: insp.ID,
		Format:       "pdf",
		FilePath:     "/nonexistent/path/file.pdf",
		GeneratedBy:  owner.ID,
	}
	storage.DB.Create(&doc)

	token := tokenFor(t, owner.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/documents/"+itoa(doc.ID)+"/download", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	// Должен перенаправить обратно и удалить запись
	if w.Code != http.StatusFound {
		t.Errorf("GET /documents/:id/download missing file: got %d, want 302", w.Code)
	}

	var count int64
	storage.DB.Model(&models.Document{}).Where("id = ?", doc.ID).Count(&count)
	if count != 0 {
		t.Error("stale document record should be deleted when file is missing")
	}
}

func TestGetDownloadDocument_NoFilePath(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass123", "Владелец Владелец Владелец", models.RoleInspector)
	insp := newInspection(t, owner.ID, "ул. Ленина, 1", "Собственник", "draft", time.Now())
	doc := newDocument(t, insp.ID, owner.ID) // FilePath пустой

	token := tokenFor(t, owner.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/documents/"+itoa(doc.ID)+"/download", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /documents/:id/download no filepath: got %d, want 404", w.Code)
	}
}
