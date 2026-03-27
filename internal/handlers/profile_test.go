package handlers

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- GET /profile ---

func TestGetProfile_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/profile", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /profile: got %d, want 200", w.Code)
	}
}

func TestGetProfile_Unauthorized(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/profile", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("GET /profile without auth: got %d, want 302", w.Code)
	}
}

// --- POST /profile ---

func TestPostProfile_UpdateName(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name": {"Петров Пётр Петрович"},
		"initials":  {"Петров П. П."},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /profile update name: got %d, want 200", w.Code)
	}

	var updated models.User
	storage.DB.First(&updated, user.ID)
	if updated.FullName != "Петров Пётр Петрович" {
		t.Errorf("full_name not updated: got %q", updated.FullName)
	}
	if updated.Initials != "Петров П. П." {
		t.Errorf("initials not updated: got %q", updated.Initials)
	}
}

func TestPostProfile_EmptyFields(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name": {""},
		"initials":  {""},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /profile empty fields: got %d, want 400", w.Code)
	}
}

func TestPostProfile_ChangePassword_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name":            {"Иванов Иван Иванович"},
		"initials":             {"Иванов И. И."},
		"current_password":     {"oldpass123"},
		"new_password":         {"NewPass1!"},
		"confirm_new_password": {"NewPass1!"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /profile change password: got %d, want 200", w.Code)
	}

	// Проверяем что новый пароль работает
	var updated models.User
	storage.DB.First(&updated, user.ID)
	if !auth.CheckPassword("NewPass1!", updated.PasswordHash) {
		t.Error("new password was not saved correctly")
	}
}

func TestPostProfile_ChangePassword_WrongCurrent(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "correctpass", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name":            {"Иванов Иван Иванович"},
		"initials":             {"Иванов И. И."},
		"current_password":     {"wrongpass"},
		"new_password":         {"newpass456"},
		"confirm_new_password": {"newpass456"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /profile wrong current password: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Неверный текущий пароль") {
		t.Error("expected error about wrong current password")
	}
}

func TestPostProfile_ChangePassword_ShortNew(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name":            {"Иванов Иван Иванович"},
		"initials":             {"Иванов И. И."},
		"current_password":     {"pass123"},
		"new_password":         {"abc"},
		"confirm_new_password": {"abc"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /profile short new password: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "6") {
		t.Error("expected error about minimum password length")
	}
}

func TestPostProfile_ChangePassword_Mismatch(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{
		"full_name":            {"Иванов Иван Иванович"},
		"initials":             {"Иванов И. И."},
		"current_password":     {"pass123"},
		"new_password":         {"newpass456"},
		"confirm_new_password": {"different789"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /profile password mismatch: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Пароли не совпадают") {
		t.Error("expected error about password mismatch")
	}
}
