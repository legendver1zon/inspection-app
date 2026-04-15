package handlers

import (
	"bytes"
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// writeMultipartField записывает текстовое поле в multipart writer.
func writeField(w *multipart.Writer, name, value string) {
	if err := w.WriteField(name, value); err != nil {
		panic(err)
	}
}

// buildEditForm создаёт валидное тело multipart-формы для PostEditInspection.
func buildEditForm(activeRooms int, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	writeField(w, "active_rooms", fmt.Sprintf("%d", activeRooms))
	writeField(w, "address", fields["address"])
	writeField(w, "owner_name", fields["owner_name"])

	for key, val := range fields {
		if key == "address" || key == "owner_name" {
			continue
		}
		writeField(w, key, val)
	}

	for i := 1; i <= activeRooms; i++ {
		iStr := fmt.Sprintf("%d", i)
		nameKey := "room_name_" + iStr
		if _, ok := fields[nameKey]; !ok {
			writeField(w, nameKey, fmt.Sprintf("Помещение %d", i))
		}
		lengthKey := "room_length_" + iStr
		if _, ok := fields[lengthKey]; !ok {
			writeField(w, lengthKey, "5.0")
		}
		widthKey := "room_width_" + iStr
		if _, ok := fields[widthKey]; !ok {
			writeField(w, widthKey, "4.0")
		}
		heightKey := "room_height_" + iStr
		if _, ok := fields[heightKey]; !ok {
			writeField(w, heightKey, "2.7")
		}
	}

	w.Close()
	return body, w.FormDataContentType()
}

// doEditPost выполняет POST к /inspections/:id/edit с multipart-формой.
func doEditPost(r http.Handler, inspID uint, body *bytes.Buffer, contentType, token string) *httptest.ResponseRecorder {
	path := fmt.Sprintf("/inspections/%d/edit", inspID)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- Нормальное сохранение ---

func TestPostEditInspection_SavesRoomsAndHeader(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-save@test.com", "pass", "Иванов Иван", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Старая, 1", "Владелец", "draft", time.Now())

	body, ct := buildEditForm(2, map[string]string{
		"address":      "ул. Новая, 10",
		"owner_name":   "Петров Пётр",
		"general_notes": "Всё хорошо",
		"room_name_1":  "Кухня",
		"room_name_2":  "Зал",
	})

	w := doEditPost(router, insp.ID, body, ct, tok)

	if w.Code != http.StatusFound {
		t.Fatalf("want 302, got %d: %s", w.Code, w.Body.String())
	}

	// Проверяем что шапка обновилась
	var updated models.Inspection
	storage.DB.First(&updated, insp.ID)
	if updated.Address != "ул. Новая, 10" {
		t.Errorf("address: want 'ул. Новая, 10', got %q", updated.Address)
	}
	if updated.OwnerName != "Петров Пётр" {
		t.Errorf("owner_name: want 'Петров Пётр', got %q", updated.OwnerName)
	}
	if updated.GeneralNotes != "Всё хорошо" {
		t.Errorf("general_notes: want 'Всё хорошо', got %q", updated.GeneralNotes)
	}

	// Проверяем комнаты
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Order("room_number").Find(&rooms)
	if len(rooms) != 2 {
		t.Fatalf("rooms count: want 2, got %d", len(rooms))
	}
	if rooms[0].RoomName != "Кухня" {
		t.Errorf("room 1 name: want 'Кухня', got %q", rooms[0].RoomName)
	}
	if rooms[1].RoomName != "Зал" {
		t.Errorf("room 2 name: want 'Зал', got %q", rooms[1].RoomName)
	}
	if rooms[0].Length != 5.0 {
		t.Errorf("room 1 length: want 5.0, got %f", rooms[0].Length)
	}
}

// --- Сохранение дефектов ---

func TestPostEditInspection_SavesDefects(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-defect@test.com", "pass", "Тестов Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Дефект, 5", "Хозяев", "draft", time.Now())

	tmpl := newDefectTemplate(t, "window", "Царапина на стекле")

	defectKey := fmt.Sprintf("defect_%d_1", tmpl.ID)
	body, ct := buildEditForm(1, map[string]string{
		"address":    "ул. Дефект, 5",
		"owner_name": "Хозяев",
		"room_name_1": "Спальня",
		defectKey:    "2 мм",
	})

	w := doEditPost(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302, got %d: %s", w.Code, w.Body.String())
	}

	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Find(&rooms)
	if len(rooms) != 1 {
		t.Fatalf("rooms: want 1, got %d", len(rooms))
	}

	var defects []models.RoomDefect
	storage.DB.Where("room_id = ?", rooms[0].ID).Find(&defects)
	if len(defects) != 1 {
		t.Fatalf("defects: want 1, got %d", len(defects))
	}
	if defects[0].Value != "2 мм" {
		t.Errorf("defect value: want '2 мм', got %q", defects[0].Value)
	}
	if defects[0].Section != "window" {
		t.Errorf("defect section: want 'window', got %q", defects[0].Section)
	}
}

// --- Перезапись: старые данные заменяются новыми ---

func TestPostEditInspection_ReplacesOldRooms(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-replace@test.com", "pass", "Заменов Замен", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Замена, 3", "Старый", "draft", time.Now())

	// Первое сохранение — 1 комната
	body1, ct1 := buildEditForm(1, map[string]string{
		"address":    "ул. Замена, 3",
		"owner_name": "Старый",
		"room_name_1": "Старая комната",
	})
	doEditPost(router, insp.ID, body1, ct1, tok)

	var countBefore int64
	storage.DB.Model(&models.InspectionRoom{}).Where("inspection_id = ?", insp.ID).Count(&countBefore)
	if countBefore != 1 {
		t.Fatalf("before: want 1 room, got %d", countBefore)
	}

	// Второе сохранение — 3 комнаты
	body2, ct2 := buildEditForm(3, map[string]string{
		"address":    "ул. Замена, 3",
		"owner_name": "Новый",
		"room_name_1": "Новая 1",
		"room_name_2": "Новая 2",
		"room_name_3": "Новая 3",
	})
	doEditPost(router, insp.ID, body2, ct2, tok)

	var countAfter int64
	storage.DB.Model(&models.InspectionRoom{}).Where("inspection_id = ?", insp.ID).Count(&countAfter)
	if countAfter != 3 {
		t.Fatalf("after: want 3 rooms, got %d", countAfter)
	}

	// Старая комната не должна остаться
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Order("room_number").Find(&rooms)
	if rooms[0].RoomName != "Новая 1" {
		t.Errorf("room 1: want 'Новая 1', got %q", rooms[0].RoomName)
	}
}

// --- КЛЮЧЕВОЙ ТЕСТ: защита от пустой формы ---

func TestPostEditInspection_EmptyForm_ProtectsExistingData(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-empty@test.com", "pass", "Защитов Защит", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Важная, 7", "Ценные Данные", "draft", time.Now())

	// Создаём реальные комнаты с данными
	room := models.InspectionRoom{
		InspectionID: insp.ID,
		RoomNumber:   1,
		RoomName:     "Кухня с данными",
		Length:       5.5,
		Width:        3.2,
		Height:       2.7,
	}
	storage.DB.Create(&room)

	// Эмулируем пустой POST (все поля пустые — как в инциденте)
	emptyBody := &bytes.Buffer{}
	w := multipart.NewWriter(emptyBody)
	w.Close()

	resp := doEditPost(router, insp.ID, emptyBody, w.FormDataContentType(), tok)

	// Должен быть redirect обратно на edit (не 302 на просмотр)
	if resp.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", resp.Code)
	}
	loc := resp.Header().Get("Location")
	expectedPrefix := fmt.Sprintf("/inspections/%d/edit", insp.ID)
	if len(loc) < len(expectedPrefix) || loc[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("redirect: want %s?error=..., got %q", expectedPrefix, loc)
	}

	// Главное: старые данные ДОЛЖНЫ остаться
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Find(&rooms)
	if len(rooms) != 1 {
		t.Fatalf("rooms after empty POST: want 1 (preserved), got %d", len(rooms))
	}
	if rooms[0].RoomName != "Кухня с данными" {
		t.Errorf("room name after empty POST: want 'Кухня с данными', got %q", rooms[0].RoomName)
	}
	if rooms[0].Length != 5.5 {
		t.Errorf("room length after empty POST: want 5.5, got %f", rooms[0].Length)
	}

	// Шапка тоже не должна измениться
	var check models.Inspection
	storage.DB.First(&check, insp.ID)
	if check.Address != "ул. Важная, 7" {
		t.Errorf("address after empty POST: want 'ул. Важная, 7', got %q", check.Address)
	}
	if check.OwnerName != "Ценные Данные" {
		t.Errorf("owner after empty POST: want 'Ценные Данные', got %q", check.OwnerName)
	}
}

// --- Тест: невалидный Content-Type (не multipart) ---

func TestPostEditInspection_InvalidContentType_ProtectsData(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-badct@test.com", "pass", "Битый Запрос", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Целая, 1", "Сохранёнов", "draft", time.Now())

	room := models.InspectionRoom{
		InspectionID: insp.ID,
		RoomNumber:   1,
		RoomName:     "Важная комната",
	}
	storage.DB.Create(&room)

	// POST с невалидным Content-Type — ParseMultipartForm упадёт
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/inspections/%d/edit", insp.ID),
		bytes.NewBufferString("garbage data"))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("want 302 redirect, got %d", w.Code)
	}

	// Данные должны остаться
	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Find(&rooms)
	if len(rooms) != 1 {
		t.Fatalf("rooms: want 1 (preserved), got %d", len(rooms))
	}
	if rooms[0].RoomName != "Важная комната" {
		t.Errorf("room name: want 'Важная комната', got %q", rooms[0].RoomName)
	}
}

// --- Тест: доступ к чужому осмотру ---

func TestPostEditInspection_ForeignInspection_Forbidden(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	owner := newUser(t, "owner-edit@test.com", "pass", "Хозяев Хозяин", models.RoleInspector)
	other := newUser(t, "other-edit@test.com", "pass", "Чужой Человек", models.RoleInspector)
	insp := newInspection(t, owner.ID, "ул. Чужая, 1", "Чужой", "draft", time.Now())

	body, ct := buildEditForm(1, map[string]string{
		"address":    "ул. Взломанная, 666",
		"owner_name": "Хакер",
	})

	tok := tokenFor(t, other.ID, "inspector")
	w := doEditPost(router, insp.ID, body, ct, tok)

	if w.Code != http.StatusForbidden {
		t.Errorf("foreign edit: want 403, got %d", w.Code)
	}

	// Данные не должны измениться
	var check models.Inspection
	storage.DB.First(&check, insp.ID)
	if check.Address != "ул. Чужая, 1" {
		t.Errorf("address: want 'ул. Чужая, 1', got %q", check.Address)
	}
}

// --- Тест: без авторизации ---

func TestPostEditInspection_Unauthorized_Redirect(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "noauth-edit@test.com", "pass", "Безтокен Токенов", models.RoleInspector)
	insp := newInspection(t, user.ID, "ул. Секретная, 1", "Секретов", "draft", time.Now())

	body, ct := buildEditForm(1, map[string]string{
		"address":    "ул. Изменённая",
		"owner_name": "Хакер",
	})

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/inspections/%d/edit", insp.ID), body)
	req.Header.Set("Content-Type", ct)
	// Нет cookie
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("want 302 (redirect to login), got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("redirect: want /login, got %q", loc)
	}
}

// --- Тест: частичные данные (address есть, active_rooms есть) — сохранение работает ---

func TestPostEditInspection_PartialData_SavesNormally(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)

	user := newUser(t, "edit-partial@test.com", "pass", "Частичный Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Пустая, 0", "Никто", "draft", time.Now())

	// active_rooms=1, address есть, owner_name пустой — это валидная форма
	body, ct := buildEditForm(1, map[string]string{
		"address":    "ул. Обновлённая, 5",
		"owner_name": "",
		"room_name_1": "Прихожая",
	})

	w := doEditPost(router, insp.ID, body, ct, tok)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", w.Code)
	}

	// Должно сохраниться нормально (не заблокировано sanity check)
	var updated models.Inspection
	storage.DB.First(&updated, insp.ID)
	if updated.Address != "ул. Обновлённая, 5" {
		t.Errorf("address: want 'ул. Обновлённая, 5', got %q", updated.Address)
	}

	var rooms []models.InspectionRoom
	storage.DB.Where("inspection_id = ?", insp.ID).Find(&rooms)
	if len(rooms) != 1 {
		t.Fatalf("rooms: want 1, got %d", len(rooms))
	}
}
