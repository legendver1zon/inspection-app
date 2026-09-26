package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

// apiRouterAs — роутер с уже «вошедшим» пользователем для JSON-API и удаления фото.
func apiRouterAs(userID uint, role string) *gin.Engine {
	r := gin.New()
	as := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("userID", userID)
			c.Set("userRole", role)
			h(c)
		}
	}
	r.GET("/api/inspections/:id", as(APIGetInspection))
	r.GET("/api/inspections/:id/edit-data", as(APIGetEditData))
	r.POST("/photos/:id/delete", as(DeletePhoto))
	return r
}

func getJSON(t *testing.T, r http.Handler, path string) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: want 200, got %d: %s", path, w.Code, w.Body.String())
	}
	return decodeJSON(t, w)
}

func firstRoomPhotos(t *testing.T, rooms interface{}) []interface{} {
	t.Helper()
	list, ok := rooms.([]interface{})
	if !ok || len(list) == 0 {
		t.Fatalf("rooms: want non-empty list, got %v", rooms)
	}
	photos, _ := list[0].(map[string]interface{})["photos"].([]interface{})
	return photos
}

func TestPostUploadInspectionPhoto_RoomOverview(t *testing.T) {
	router, insp, room, _, tok := keyedFixture(t, "overview@test.com")
	chdirUploads(t)

	body, ct := keyedPhotoForm(t, map[string]string{"room_number": "1", "section": "overview", "client_id": "ov-1"}, "photo.jpg")
	w := postKeyedPhoto(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if resp := decodeJSON(t, w); resp["defect_id"] != nil {
		t.Errorf("у фото общего вида не должно быть defect_id: %v", resp)
	}

	var photo models.Photo
	if err := storage.DB.Where("client_id = ?", "ov-1").First(&photo).Error; err != nil {
		t.Fatalf("photo not found: %v", err)
	}
	if photo.Kind != models.PhotoKindRoom || photo.RoomNumber != 1 || photo.InspectionID != insp.ID || photo.DefectID != nil {
		t.Errorf("photo = kind %q room %d insp %d defect %v", photo.Kind, photo.RoomNumber, photo.InspectionID, photo.DefectID)
	}
	var n int64
	storage.DB.Model(&models.RoomDefect{}).Where("room_id = ?", room.ID).Count(&n)
	if n != 0 {
		t.Errorf("общий вид не должен создавать дефект, got %d", n)
	}

	api := apiRouterAs(insp.UserID, "inspector")
	ed := getJSON(t, api, fmt.Sprintf("/api/inspections/%d/edit-data", insp.ID))
	if got := firstRoomPhotos(t, ed["rooms"]); len(got) != 1 {
		t.Errorf("edit-data: want 1 фото общего вида, got %v", got)
	}
	gp, _ := ed["act"].(map[string]interface{})["general_photos"].(map[string]interface{})
	for _, k := range []string{"electricity", "ventilation", "general"} {
		if _, ok := gp[k]; !ok {
			t.Errorf("edit-data: нет ключа general_photos.%s: %v", k, gp)
		}
	}
	view := getJSON(t, api, fmt.Sprintf("/api/inspections/%d", insp.ID))
	if got := firstRoomPhotos(t, view["inspection"].(map[string]interface{})["rooms"]); len(got) != 1 {
		t.Errorf("view: want 1 фото общего вида, got %v", got)
	}
}

func TestPostUploadInspectionPhoto_GeneralKinds(t *testing.T) {
	router, insp, _, tmpl, tok := keyedFixture(t, "general@test.com")
	chdirUploads(t)

	cases := []struct {
		name   string
		fields map[string]string
		code   int
	}{
		{"электрика без помещения", map[string]string{"section": "electricity", "client_id": "g-1"}, http.StatusOK},
		{"вентиляция с номером помещения", map[string]string{"section": "ventilation", "room_number": "1", "client_id": "g-2"}, http.StatusBadRequest},
		{"общие замечания с шаблоном", map[string]string{"section": "general", "template_id": fmt.Sprint(tmpl.ID), "client_id": "g-3"}, http.StatusBadRequest},
		{"неизвестная секция", map[string]string{"section": "bogus", "room_number": "1", "client_id": "g-4"}, http.StatusBadRequest},
		{"общий вид несуществующего помещения", map[string]string{"section": "overview", "room_number": "5", "client_id": "g-5"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		body, ct := keyedPhotoForm(t, tc.fields, "photo.jpg")
		if w := postKeyedPhoto(router, insp.ID, body, ct, tok); w.Code != tc.code {
			t.Errorf("%s: want %d, got %d: %s", tc.name, tc.code, w.Code, w.Body.String())
		}
	}

	var photo models.Photo
	if err := storage.DB.Where("client_id = ?", "g-1").First(&photo).Error; err != nil {
		t.Fatalf("photo not found: %v", err)
	}
	if photo.Kind != models.PhotoKindElectricity || photo.RoomNumber != 0 || photo.DefectID != nil || photo.InspectionID != insp.ID {
		t.Errorf("photo = kind %q room %d insp %d defect %v", photo.Kind, photo.RoomNumber, photo.InspectionID, photo.DefectID)
	}
	ed := getJSON(t, apiRouterAs(insp.UserID, "inspector"), fmt.Sprintf("/api/inspections/%d/edit-data", insp.ID))
	gp, _ := ed["act"].(map[string]interface{})["general_photos"].(map[string]interface{})
	if list, _ := gp["electricity"].([]interface{}); len(list) != 1 {
		t.Errorf("edit-data: want 1 фото электрики, got %v", gp)
	}
}

func createPhoto(t *testing.T, p models.Photo) models.Photo {
	t.Helper()
	p.FileName = "p.jpg"
	p.UploadStatus = "done"
	if err := storage.DB.Create(&p).Error; err != nil {
		t.Fatalf("create photo: %v", err)
	}
	return p
}

func TestPostEditInspection_RoomPrev_MovesPhotos(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)
	user := newUser(t, "prev@test.com", "pass", "Прежний Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Прежняя, 1", "Хозяев", "draft", time.Now())
	tmpl := newDefectTemplate(t, "floor", "Скол")

	body, ct := buildEditForm(2, map[string]string{
		"address": "ул. Прежняя, 1", "owner_name": "Хозяев",
		"room_name_1": "Кухня", "room_name_2": "Спальня",
		fmt.Sprintf("defect_%d_2", tmpl.ID): "1",
	})
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("first save: want 302, got %d: %s", w.Code, w.Body.String())
	}
	old := roomDefects(t, insp.ID, 2)
	if len(old) != 1 {
		t.Fatalf("first save: want 1 defect in room 2, got %d", len(old))
	}
	kitchen := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindRoom, RoomNumber: 1})
	bedroom := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindRoom, RoomNumber: 2})
	defectPhoto := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindDefect, DefectID: uptr(old[0].ID)})

	// Кухню удалили: спальня стала помещением 1 и сообщает прежний номер 2
	body, ct = buildEditForm(1, map[string]string{
		"address": "ул. Прежняя, 1", "owner_name": "Хозяев",
		"room_name_1": "Спальня", "room_prev_1": "2",
		fmt.Sprintf("defect_%d_1", tmpl.ID): "1",
	})
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("second save: want 302, got %d: %s", w.Code, w.Body.String())
	}
	fresh := roomDefects(t, insp.ID, 1)
	if len(fresh) != 1 {
		t.Fatalf("second save: want 1 defect in room 1, got %d", len(fresh))
	}

	var moved, relinked models.Photo
	storage.DB.First(&moved, bedroom.ID)
	if moved.RoomNumber != 1 {
		t.Errorf("фото спальни должно переехать в помещение 1, got %d", moved.RoomNumber)
	}
	storage.DB.First(&relinked, defectPhoto.ID)
	if relinked.DefectID == nil || *relinked.DefectID != fresh[0].ID {
		t.Errorf("фото дефекта должно перепривязаться к новому дефекту %d, got %v", fresh[0].ID, relinked.DefectID)
	}
	var gone models.Photo
	storage.DB.Unscoped().First(&gone, kitchen.ID)
	if !gone.DeletedAt.Valid {
		t.Errorf("фото удалённой кухни должно уйти в архив")
	}
}

func TestPostEditInspection_WithoutPrev_KeepsRoomPhotos(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)
	user := newUser(t, "noprev@test.com", "pass", "Старый Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Старая, 1", "Хозяев", "draft", time.Now())

	fields := map[string]string{"address": "ул. Старая, 1", "owner_name": "Хозяев", "room_name_1": "Зал"}
	body, ct := buildEditForm(1, fields)
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("first save: want 302, got %d", w.Code)
	}
	photo := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindRoom, RoomNumber: 1})

	body, ct = buildEditForm(1, fields)
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("second save: want 302, got %d", w.Code)
	}
	var p models.Photo
	if err := storage.DB.First(&p, photo.ID).Error; err != nil || p.RoomNumber != 1 {
		t.Errorf("без room_prev фото остаётся в помещении 1: err=%v room=%d", err, p.RoomNumber)
	}
}

func TestDeletePhoto_RoomOverview_OwnerOnly(t *testing.T) {
	_, insp, _, _, _ := keyedFixture(t, "delov@test.com")
	photo := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindRoom, RoomNumber: 1})
	other := newUser(t, "delov-other@test.com", "pass", "Чужой Тест", models.RoleInspector)
	path := fmt.Sprintf("/photos/%d/delete", photo.ID)

	w := httptest.NewRecorder()
	apiRouterAs(other.ID, "inspector").ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("чужой: want 403, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	apiRouterAs(insp.UserID, "inspector").ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
	if w.Code != http.StatusOK {
		t.Errorf("владелец: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var n int64
	storage.DB.Model(&models.Photo{}).Where("id = ?", photo.ID).Count(&n)
	if n != 0 {
		t.Errorf("фото должно быть удалено")
	}
}
