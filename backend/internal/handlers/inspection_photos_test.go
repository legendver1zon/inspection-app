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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// chdirUploads переводит процесс во временный каталог с web/static/uploads,
// чтобы загруженные файлы не попадали в репозиторий.
func chdirUploads(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "web", "static", "uploads", "photos"), 0755)
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	return tmp
}

// keyedPhotoForm собирает multipart для POST /inspections/:id/photos.
func keyedPhotoForm(t *testing.T, fields map[string]string, filename string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		writeField(w, k, v)
	}
	if filename != "" {
		fw, err := w.CreateFormFile("photo", filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		io.WriteString(fw, "fake-jpeg-content")
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

func postKeyedPhoto(r http.Handler, inspID uint, body *bytes.Buffer, ct, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/inspections/%d/photos", inspID), body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v; body=%s", err, w.Body.String())
	}
	return resp
}

// keyedFixture — владелец, токен, осмотр с помещением №1 и шаблон потолка.
func keyedFixture(t *testing.T, email string) (*gin.Engine, models.Inspection, models.InspectionRoom, models.DefectTemplate, string) {
	t.Helper()
	setupTestDB(t)
	router := setupRouter(t)
	user := newUser(t, email, "pass", "Ключев Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Ключевая, 1", "Хозяев", "draft", time.Now())
	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Кухня"}
	if err := storage.DB.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	tmpl := newDefectTemplate(t, "ceiling", "Трещина")
	return router, insp, room, tmpl, tok
}

func TestPostUploadInspectionPhoto_CreatesDefectByKey(t *testing.T) {
	router, insp, room, tmpl, tok := keyedFixture(t, "keyed-create@test.com")
	chdirUploads(t)

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "ceiling",
		"template_id": fmt.Sprint(tmpl.ID), "client_id": "abc-123_XYZ",
	}, "photo.jpg")
	w := postKeyedPhoto(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if resp["duplicate"] != nil {
		t.Errorf("первая загрузка не должна быть duplicate: %v", resp)
	}

	var defects []models.RoomDefect
	storage.DB.Where("room_id = ?", room.ID).Find(&defects)
	if len(defects) != 1 {
		t.Fatalf("want 1 defect, got %d", len(defects))
	}
	d := defects[0]
	if d.Value != "" || d.Section != "ceiling" || d.DefectTemplateID == nil || *d.DefectTemplateID != tmpl.ID || d.WallNumber != 0 {
		t.Errorf("дефект создан не по ключу: %+v", d)
	}
	if uint(resp["defect_id"].(float64)) != d.ID {
		t.Errorf("defect_id в ответе: want %d, got %v", d.ID, resp["defect_id"])
	}

	var photo models.Photo
	if err := storage.DB.Where("defect_id = ?", d.ID).First(&photo).Error; err != nil {
		t.Fatalf("фото не привязано: %v", err)
	}
	if photo.ClientID == nil || *photo.ClientID != "abc-123_XYZ" {
		t.Errorf("client_id не сохранён: %+v", photo.ClientID)
	}
	if _, err := os.Stat(photo.FilePath); err != nil {
		t.Errorf("файл не сохранён: %v", err)
	}
	if uint(resp["id"].(float64)) != photo.ID {
		t.Errorf("id в ответе: want %d, got %v", photo.ID, resp["id"])
	}
}

func TestPostUploadInspectionPhoto_ExistingDefect(t *testing.T) {
	router, insp, room, tmpl, tok := keyedFixture(t, "keyed-existing@test.com")
	chdirUploads(t)

	existing := models.RoomDefect{RoomID: room.ID, DefectTemplateID: &tmpl.ID, Section: "ceiling", Value: "2 мм"}
	storage.DB.Create(&existing)

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "ceiling",
		"template_id": fmt.Sprint(tmpl.ID), "wall_number": "0", "client_id": "exist-1",
	}, "photo.jpg")
	w := postKeyedPhoto(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if uint(resp["defect_id"].(float64)) != existing.ID {
		t.Errorf("want привязку к существующему дефекту %d, got %v", existing.ID, resp["defect_id"])
	}
	var count int64
	storage.DB.Model(&models.RoomDefect{}).Where("room_id = ?", room.ID).Count(&count)
	if count != 1 {
		t.Errorf("новый дефект создаваться не должен, got %d", count)
	}
	storage.DB.Model(&models.Photo{}).Where("defect_id = ?", existing.ID).Count(&count)
	if count != 1 {
		t.Errorf("want 1 photo, got %d", count)
	}
}

func TestPostUploadInspectionPhoto_WallAndNotesKeys(t *testing.T) {
	router, insp, room, _, tok := keyedFixture(t, "keyed-wall@test.com")
	chdirUploads(t)
	wallTmpl := newDefectTemplate(t, "wall", "Отклонение")

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "wall",
		"template_id": fmt.Sprint(wallTmpl.ID), "wall_number": "3", "client_id": "wall-3",
	}, "photo.jpg")
	if w := postKeyedPhoto(router, insp.ID, body, ct, tok); w.Code != http.StatusOK {
		t.Fatalf("wall: want 200, got %d: %s", w.Code, w.Body.String())
	}
	body, ct = keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "floor", "template_id": "", "client_id": "notes-floor",
	}, "photo.png")
	if w := postKeyedPhoto(router, insp.ID, body, ct, tok); w.Code != http.StatusOK {
		t.Fatalf("notes: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var defects []models.RoomDefect
	storage.DB.Where("room_id = ?", room.ID).Order("id").Find(&defects)
	if len(defects) != 2 {
		t.Fatalf("want 2 defects, got %d", len(defects))
	}
	if d := defects[0]; d.Section != "wall" || d.WallNumber != 3 || d.DefectTemplateID == nil || d.Value != "" {
		t.Errorf("дефект стены: %+v", d)
	}
	if d := defects[1]; d.Section != "floor" || d.WallNumber != 0 || d.DefectTemplateID != nil || d.Notes != "" {
		t.Errorf("дефект «Прочее»: %+v", d)
	}
}

func TestPostUploadInspectionPhoto_DuplicateClientID(t *testing.T) {
	router, insp, room, tmpl, tok := keyedFixture(t, "keyed-dup@test.com")
	tmp := chdirUploads(t)

	fields := map[string]string{
		"room_number": "1", "section": "ceiling",
		"template_id": fmt.Sprint(tmpl.ID), "client_id": "dup-001",
	}
	body, ct := keyedPhotoForm(t, fields, "photo.jpg")
	first := decodeJSON(t, postKeyedPhoto(router, insp.ID, body, ct, tok))

	body, ct = keyedPhotoForm(t, fields, "photo.jpg")
	w := postKeyedPhoto(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("повтор: want 200, got %d: %s", w.Code, w.Body.String())
	}
	second := decodeJSON(t, w)
	if second["id"] != first["id"] || second["duplicate"] != true {
		t.Errorf("повтор должен вернуть то же фото с duplicate=true: first=%v second=%v", first, second)
	}
	if second["url"] != first["url"] || second["defect_id"] != first["defect_id"] {
		t.Errorf("ответ повтора отличается: first=%v second=%v", first, second)
	}

	var count int64
	storage.DB.Model(&models.Photo{}).Count(&count)
	if count != 1 {
		t.Errorf("want 1 photo record, got %d", count)
	}
	var defect models.RoomDefect
	storage.DB.Where("room_id = ?", room.ID).First(&defect)
	files, _ := os.ReadDir(filepath.Join(tmp, "web", "static", "uploads", "photos", fmt.Sprint(insp.ID), fmt.Sprint(defect.ID)))
	if len(files) != 1 {
		t.Errorf("want 1 file on disk, got %d", len(files))
	}
}

func TestPostUploadInspectionPhoto_Stranger_Forbidden(t *testing.T) {
	router, insp, _, tmpl, _ := keyedFixture(t, "keyed-owner@test.com")
	chdirUploads(t)
	stranger := newUser(t, "keyed-stranger@test.com", "pass", "Чужой Человек", models.RoleInspector)
	tok := tokenFor(t, stranger.ID, "inspector")

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "client_id": "stranger-1",
	}, "photo.jpg")
	if w := postKeyedPhoto(router, insp.ID, body, ct, tok); w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d: %s", w.Code, w.Body.String())
	}
	var count int64
	storage.DB.Model(&models.Photo{}).Count(&count)
	if count != 0 {
		t.Errorf("чужое фото не должно сохраняться, got %d", count)
	}
}

func TestPostUploadInspectionPhoto_RoomNotFound(t *testing.T) {
	router, insp, _, tmpl, tok := keyedFixture(t, "keyed-room@test.com")
	chdirUploads(t)

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "7", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "client_id": "room-7",
	}, "photo.jpg")
	w := postKeyedPhoto(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", w.Code, w.Body.String())
	}
	if resp := decodeJSON(t, w); resp["error"] != "Помещение не найдено" {
		t.Errorf("текст ошибки: %v", resp["error"])
	}
}

func TestPostUploadInspectionPhoto_BadInput(t *testing.T) {
	router, insp, _, tmpl, tok := keyedFixture(t, "keyed-bad@test.com")
	chdirUploads(t)
	wallTmpl := newDefectTemplate(t, "wall", "Трещина")

	cases := []struct {
		name   string
		fields map[string]string
	}{
		{"стена вне 1..4", map[string]string{"room_number": "1", "section": "wall", "template_id": fmt.Sprint(wallTmpl.ID), "wall_number": "5", "client_id": "w5"}},
		{"стена без номера", map[string]string{"room_number": "1", "section": "wall", "template_id": fmt.Sprint(wallTmpl.ID), "client_id": "w0"}},
		{"номер стены не для стены", map[string]string{"room_number": "1", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "wall_number": "2", "client_id": "c2"}},
		{"неверная секция", map[string]string{"room_number": "1", "section": "roof", "template_id": fmt.Sprint(tmpl.ID), "client_id": "roof"}},
		{"шаблон другой секции", map[string]string{"room_number": "1", "section": "floor", "template_id": fmt.Sprint(tmpl.ID), "client_id": "mismatch"}},
		{"без client_id", map[string]string{"room_number": "1", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID)}},
		{"client_id с пробелом", map[string]string{"room_number": "1", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "client_id": "a b"}},
		{"room_number=0", map[string]string{"room_number": "0", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "client_id": "r0"}},
	}
	for _, tc := range cases {
		body, ct := keyedPhotoForm(t, tc.fields, "photo.jpg")
		if w := postKeyedPhoto(router, insp.ID, body, ct, tok); w.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d: %s", tc.name, w.Code, w.Body.String())
		}
	}
	var count int64
	storage.DB.Model(&models.RoomDefect{}).Count(&count)
	if count != 0 {
		t.Errorf("при 400 дефекты создаваться не должны, got %d", count)
	}
}

// Сквозной сценарий: фото пришло по ключу, потом форма сохранена с picked-флагом —
// фото остаётся у актуального дефекта.
func TestPostUploadInspectionPhoto_SurvivesEditResave(t *testing.T) {
	router, insp, _, tmpl, tok := keyedFixture(t, "keyed-resave@test.com")
	chdirUploads(t)

	body, ct := keyedPhotoForm(t, map[string]string{
		"room_number": "1", "section": "ceiling", "template_id": fmt.Sprint(tmpl.ID), "client_id": "resave-1",
	}, "photo.jpg")
	resp := decodeJSON(t, postKeyedPhoto(router, insp.ID, body, ct, tok))
	photoID := uint(resp["id"].(float64))
	oldDefectID := uint(resp["defect_id"].(float64))

	editBody, editCT := buildEditForm(1, map[string]string{
		"address": "ул. Ключевая, 1", "owner_name": "Хозяев",
		fmt.Sprintf("picked_%d_1", tmpl.ID): "1",
	})
	if w := doEditPost(router, insp.ID, editBody, editCT, tok); w.Code != http.StatusFound {
		t.Fatalf("edit: want 302, got %d", w.Code)
	}

	var photo models.Photo
	storage.DB.First(&photo, photoID)
	var defect models.RoomDefect
	if photo.DefectID == nil {
		t.Fatal("у фото дефекта должен быть defect_id")
	}
	if err := storage.DB.First(&defect, *photo.DefectID).Error; err != nil {
		t.Fatalf("фото указывает на удалённый дефект %d: %v", *photo.DefectID, err)
	}
	if defect.ID == oldDefectID {
		t.Errorf("после сохранения дефект должен быть пересоздан, а фото перепривязано")
	}
	if defect.Section != "ceiling" || defect.DefectTemplateID == nil || *defect.DefectTemplateID != tmpl.ID {
		t.Errorf("фото перепривязано не туда: %+v", defect)
	}
}
