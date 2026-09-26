package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"
)

// createLocalPhoto создаёт файл в web/static/uploads (относительно CWD теста)
// и запись Photo с локальным FilePath.
func createLocalPhoto(t *testing.T, defectID uint, name, content string) models.Photo {
	t.Helper()
	dir := filepath.Join("web", "static", "uploads", "testphotos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll("web") })

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	photo := models.Photo{
		DefectID: uptr(defectID),
		FileName: name,
		FilePath: path,
		FileURL:  "/static/uploads/testphotos/" + name,
	}
	if err := storage.DB.Create(&photo).Error; err != nil {
		t.Fatalf("create photo: %v", err)
	}
	return photo
}

func getPhotoDownload(r http.Handler, photoID uint, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/photos/"+itoa(photoID)+"/download", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	return w
}

// Владелец скачивает фото архивного (soft-deleted) дефекта — цепочка авторизации
// через Unscoped не рвётся. До фикса loadPhotoInspection владелец получал 403.
func TestGetPhotoDownload_ArchivedDefect_OwnerGetsFile(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	insp, defect := newInspectionWithDefect(t, "arch-owner")
	photo := createLocalPhoto(t, defect.ID, "arch.jpg", "fake-photo-bytes")

	if err := storage.DB.Delete(&models.RoomDefect{}, defect.ID).Error; err != nil {
		t.Fatalf("soft delete defect: %v", err)
	}

	w := getPhotoDownload(r, photo.ID, tokenFor(t, insp.UserID, "inspector"))
	if w.Code != http.StatusOK {
		t.Fatalf("владелец, архивный дефект: got %d, want 200; тело: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "fake-photo-bytes" {
		t.Error("тело ответа не совпадает с содержимым файла")
	}
}

// Чужой инспектор не получает фото архивного дефекта — авторизация не потерялась.
func TestGetPhotoDownload_ArchivedDefect_StrangerForbidden(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	_, defect := newInspectionWithDefect(t, "arch-stranger")
	photo := createLocalPhoto(t, defect.ID, "arch2.jpg", "secret-bytes")
	if err := storage.DB.Delete(&models.RoomDefect{}, defect.ID).Error; err != nil {
		t.Fatalf("soft delete defect: %v", err)
	}

	stranger := newUser(t, "stranger_dl@test.com", "pass123", "Чужой Чужой Чужой", models.RoleInspector)
	w := getPhotoDownload(r, photo.ID, tokenFor(t, stranger.ID, "inspector"))
	if w.Code != http.StatusForbidden {
		t.Errorf("чужой инспектор: got %d, want 403", w.Code)
	}
}

// Фото удалённого (soft-deleted) ОСМОТРА — 404: дефект и помещение ищутся
// Unscoped, но сам осмотр — нет, обрыв цепочки отвечает явной ошибкой.
func TestGetPhotoDownload_DeletedInspection_NotFound(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	insp, defect := newInspectionWithDefect(t, "deleted-insp")
	photo := createLocalPhoto(t, defect.ID, "del.jpg", "bytes")

	if err := storage.DB.Delete(&models.Inspection{}, insp.ID).Error; err != nil {
		t.Fatalf("soft delete inspection: %v", err)
	}

	w := getPhotoDownload(r, photo.ID, tokenFor(t, insp.UserID, "inspector"))
	if w.Code != http.StatusNotFound {
		t.Errorf("фото удалённого осмотра: got %d, want 404", w.Code)
	}
}

// FilePath вне web/static/uploads не отдаётся напрямую: срабатывает
// защита каталога, ответ уходит в fallback (редирект на FileURL).
func TestGetPhotoDownload_FileOutsideUploads_NotServed(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	insp, defect := newInspectionWithDefect(t, "outside-dir")

	tmp, err := os.CreateTemp("", "outside_*.jpg")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmp.WriteString("outside-content")
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	photo := models.Photo{
		DefectID: uptr(defect.ID),
		FileName: "outside.jpg",
		FilePath: tmp.Name(),
		FileURL:  "/static/uploads/none.jpg",
	}
	if err := storage.DB.Create(&photo).Error; err != nil {
		t.Fatalf("create photo: %v", err)
	}

	w := getPhotoDownload(r, photo.ID, tokenFor(t, insp.UserID, "inspector"))
	if w.Code == http.StatusOK && w.Body.String() == "outside-content" {
		t.Fatal("файл вне uploads-каталога отдан напрямую")
	}
	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("ожидали 307 (fallback на FileURL), получили %d", w.Code)
	}
}

// Владелец удаляет фото архивного дефекта — та же Unscoped-цепочка в DeletePhoto.
func TestDeletePhoto_ArchivedDefect_OwnerAllowed(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	insp, defect := newInspectionWithDefect(t, "arch-del")
	photo := createLocalPhoto(t, defect.ID, "arch3.jpg", "bytes")
	if err := storage.DB.Delete(&models.RoomDefect{}, defect.ID).Error; err != nil {
		t.Fatalf("soft delete defect: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/photos/"+itoa(photo.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tokenFor(t, insp.UserID, "inspector")})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("удаление фото архивного дефекта владельцем: got %d, want 200; тело: %s", w.Code, w.Body.String())
	}
	var count int64
	storage.DB.Model(&models.Photo{}).Where("id = ?", photo.ID).Count(&count)
	if count != 0 {
		t.Error("фото не удалено")
	}
}
