package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

func pngDataURL(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 60, 24))
	for x := 5; x < 55; x++ {
		img.Set(x, 12, color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func withFields(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func postEditAsFetch(r *gin.Engine, inspID uint, body *bytes.Buffer, ct, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/inspections/%d/edit", inspID), body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func postJSONTo(r http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSignatures_OwnerLocksAct(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)
	chdirUploads(t)
	user := newUser(t, "sign-owner@test.com", "pass", "Подписов Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Подписная, 1", "Иванов", "draft", time.Now())
	fields := map[string]string{"address": "ул. Подписная, 1", "owner_name": "Иванов", "room_name_1": "Зал"}

	// Подпись собственника приходит вместе с правками
	body, ct := buildEditForm(1, withFields(fields, map[string]string{
		"signature_owner": pngDataURL(t), "signature_owner_at": "2026-09-26T15:40:00+05:00",
	}))
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("save with signature: want 302, got %d: %s", w.Code, w.Body.String())
	}
	var sig models.Signature
	if err := storage.DB.Where("inspection_id = ? AND role = ?", insp.ID, models.SignatureRoleOwner).First(&sig).Error; err != nil {
		t.Fatalf("signature row: %v", err)
	}
	if sig.SignedAt.UTC().Hour() != 10 || sig.SignedAt.UTC().Minute() != 40 {
		t.Errorf("SignedAt должно сохранить время с телефона (15:40+05 = 10:40 UTC), got %v", sig.SignedAt.UTC())
	}
	if sig.TZOffsetMin != 300 || sig.SignedLocal().Format("15:04") != "15:40" {
		t.Errorf("в акте печатается местное время телефона: offset=%d local=%s", sig.TZOffsetMin, sig.SignedLocal().Format("15:04"))
	}
	if _, err := os.Stat(uploadsPath(sig.FilePath)); err != nil {
		t.Fatalf("файл подписи не записан: %v", err)
	}
	api := apiRouterAs(user.ID, "inspector")
	ed := getJSON(t, api, fmt.Sprintf("/api/inspections/%d/edit-data", insp.ID))
	act := ed["act"].(map[string]interface{})
	if act["locked"] != true {
		t.Errorf("edit-data: акт должен быть locked, got %v", act["locked"])
	}
	if sigs, _ := act["signatures"].(map[string]interface{}); sigs["owner"] == nil || sigs["inspector"] != nil {
		t.Errorf("edit-data signatures: %v", act["signatures"])
	}

	// Повторное сохранение закрыто (403 для fetch, редирект с ошибкой для HTML)
	body, ct = buildEditForm(1, fields)
	if w := postEditAsFetch(router, insp.ID, body, ct, tok); w.Code != http.StatusForbidden {
		t.Errorf("locked save: want 403, got %d: %s", w.Code, w.Body.String())
	}
	body, ct = buildEditForm(1, fields)
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "error=") {
		t.Errorf("locked save (html): want redirect with error, got %d %s", w.Code, w.Header().Get("Location"))
	}

	// Удаление фото закрыто
	photo := createPhoto(t, models.Photo{InspectionID: insp.ID, Kind: models.PhotoKindRoom, RoomNumber: 1})
	w := httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/photos/%d/delete", photo.ID), nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("delete photo on locked act: want 403, got %d", w.Code)
	}

	// Картинку подписи видит владелец, чужой — нет
	w = httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/inspections/%d/signature/owner", insp.ID), nil))
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/png") {
		t.Errorf("owner signature image: want 200 png, got %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	other := newUser(t, "sign-other@test.com", "pass", "Чужой Тест", models.RoleInspector)
	w = httptest.NewRecorder()
	apiRouterAs(other.ID, "inspector").ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/inspections/%d/signature/owner", insp.ID), nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("stranger signature image: want 403, got %d", w.Code)
	}

	// Снятие подписи открывает акт и удаляет файл
	body, ct = buildEditForm(1, withFields(fields, map[string]string{"signature_owner_clear": "1"}))
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("clear signature: want 302, got %d", w.Code)
	}
	if ownerSigned(insp.ID) {
		t.Errorf("после signature_owner_clear акт должен быть открыт")
	}
	if _, err := os.Stat(uploadsPath(sig.FilePath)); err == nil {
		t.Errorf("файл снятой подписи должен быть удалён")
	}
}

func TestSignatures_InspectorFromProfile(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)
	chdirUploads(t)
	user := newUser(t, "sign-profile@test.com", "pass", "Профилев Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Профильная, 2", "Петров", "draft", time.Now())
	api := apiRouterAs(user.ID, "inspector")

	w := postJSONTo(api, "/api/profile/signature", fmt.Sprintf(`{"data_url":%q}`, pngDataURL(t)))
	if w.Code != http.StatusOK {
		t.Fatalf("set profile signature: want 200, got %d: %s", w.Code, w.Body.String())
	}
	if u := decodeJSON(t, w)["user"].(map[string]interface{}); u["has_signature"] != true || u["signature_url"] == "" {
		t.Errorf("user after set: %v", u)
	}
	w = httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/profile/signature", nil))
	if w.Code != http.StatusOK {
		t.Errorf("get profile signature: want 200, got %d", w.Code)
	}

	body, ct := buildEditForm(1, map[string]string{
		"address": "ул. Профильная, 2", "owner_name": "Петров", "room_name_1": "Зал",
		"signature_inspector_from_profile": "1", "signature_inspector_at": "2026-09-26T16:00:00+05:00",
	})
	if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
		t.Fatalf("save with profile signature: want 302, got %d", w.Code)
	}
	var sig models.Signature
	if err := storage.DB.Where("inspection_id = ? AND role = ?", insp.ID, models.SignatureRoleInspector).First(&sig).Error; err != nil {
		t.Fatalf("inspector signature row: %v", err)
	}
	if ownerSigned(insp.ID) {
		t.Errorf("подпись инспектора не должна блокировать акт")
	}

	w = postJSONTo(api, "/api/profile/signature/delete", "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete profile signature: want 200, got %d", w.Code)
	}
	if u := decodeJSON(t, w)["user"].(map[string]interface{}); u["has_signature"] != false {
		t.Errorf("user after delete: %v", u)
	}
	// Снимок в акте живёт своей жизнью
	if _, err := os.Stat(uploadsPath(sig.FilePath)); err != nil {
		t.Errorf("подпись в акте должна остаться после удаления из профиля: %v", err)
	}

	body, ct = buildEditForm(1, map[string]string{
		"address": "ул. Профильная, 2", "owner_name": "Петров", "room_name_1": "Зал",
		"signature_inspector_from_profile": "1",
	})
	if w := postEditAsFetch(router, insp.ID, body, ct, tok); w.Code != http.StatusBadRequest {
		t.Errorf("from_profile без подписи в профиле: want 400, got %d", w.Code)
	}
}

func TestHideClimate_SavedAndInherited(t *testing.T) {
	setupTestDB(t)
	router := setupRouter(t)
	user := newUser(t, "climate@test.com", "pass", "Летов Тест", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	insp := newInspection(t, user.ID, "ул. Летняя, 3", "Хозяев", "draft", time.Now())
	api := apiRouterAs(user.ID, "inspector")
	fields := map[string]string{"address": "ул. Летняя, 3", "owner_name": "Хозяев", "room_name_1": "Зал"}

	for _, tc := range []struct {
		val  string
		want bool
	}{{"1", true}, {"0", false}, {"1", true}} {
		body, ct := buildEditForm(1, withFields(fields, map[string]string{"hide_climate": tc.val}))
		if w := doEditPost(router, insp.ID, body, ct, tok); w.Code != http.StatusFound {
			t.Fatalf("save hide_climate=%s: want 302, got %d", tc.val, w.Code)
		}
		ed := getJSON(t, api, fmt.Sprintf("/api/inspections/%d/edit-data", insp.ID))
		if got := ed["act"].(map[string]interface{})["hide_climate"]; got != tc.want {
			t.Errorf("hide_climate=%s: edit-data = %v, want %v", tc.val, got, tc.want)
		}
	}
	// Без поля (старая форма) значение не трогаем
	body, ct := buildEditForm(1, fields)
	doEditPost(router, insp.ID, body, ct, tok)
	var reloaded models.Inspection
	storage.DB.First(&reloaded, insp.ID)
	if !reloaded.HideClimate {
		t.Errorf("сохранение без hide_climate не должно сбрасывать флаг")
	}

	// Новый акт наследует настройку
	w := postJSONTo(api, "/api/inspections", "")
	if w.Code != http.StatusOK {
		t.Fatalf("create: want 200, got %d: %s", w.Code, w.Body.String())
	}
	newID := uint(decodeJSON(t, w)["id"].(float64))
	var created models.Inspection
	storage.DB.First(&created, newID)
	if !created.HideClimate {
		t.Errorf("новый акт должен унаследовать hide_climate=true")
	}
}
