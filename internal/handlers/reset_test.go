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
	"time"
)

// --- GET /forgot-password ---

func TestGetForgotPassword_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/forgot-password", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /forgot-password: got %d, want 200", w.Code)
	}
}

// --- POST /forgot-password ---
// Оба случая (известный и неизвестный email) должны вернуть 200 с "sent"

func TestPostForgotPassword_KnownEmail_ShowsSent(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	form := url.Values{"email": {"user@test.com"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /forgot-password known email: got %d, want 200", w.Code)
	}

	// Проверяем что код записан в БД
	var updated models.User
	storage.DB.First(&updated, user.ID)
	if updated.ResetToken == "" {
		t.Error("expected reset_token to be set after forgot-password")
	}
	if updated.ResetExpiry == nil {
		t.Error("expected reset_expiry to be set after forgot-password")
	}
}

func TestPostForgotPassword_UnknownEmail_AlsoShowsSent(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	// Неизвестный email — ответ должен быть такой же (не раскрываем наличие)
	form := url.Values{"email": {"nobody@test.com"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /forgot-password unknown email: got %d, want 200", w.Code)
	}
}

// --- GET /reset-password ---

func TestGetResetPassword_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reset-password?email=user@test.com", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /reset-password: got %d, want 200", w.Code)
	}
}

// --- POST /reset-password ---

func setResetToken(t *testing.T, userID uint, code string, expiry time.Time) {
	t.Helper()
	storage.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"reset_token":  code,
		"reset_expiry": expiry,
	})
}

func TestPostResetPassword_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass", "Иванов Иван Иванович", models.RoleInspector)
	expiry := time.Now().Add(15 * time.Minute)
	setResetToken(t, user.ID, "123456", expiry)

	form := url.Values{
		"email":    {"user@test.com"},
		"code":     {"123456"},
		"password": {"newpass789"},
		"confirm":  {"newpass789"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /reset-password success: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("POST /reset-password redirect: got %q, want /login...", loc)
	}

	// Новый пароль должен работать
	var updated models.User
	storage.DB.First(&updated, user.ID)
	if !auth.CheckPassword("newpass789", updated.PasswordHash) {
		t.Error("new password was not saved after reset")
	}
	// Токен должен быть очищен
	if updated.ResetToken != "" {
		t.Error("reset_token should be cleared after successful reset")
	}
}

func TestPostResetPassword_WrongCode(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass", "Иванов Иван Иванович", models.RoleInspector)
	expiry := time.Now().Add(15 * time.Minute)
	setResetToken(t, user.ID, "123456", expiry)

	form := url.Values{
		"email":    {"user@test.com"},
		"code":     {"999999"},
		"password": {"newpass789"},
		"confirm":  {"newpass789"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /reset-password wrong code: got %d, want 200 with error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Неверный код") {
		t.Error("expected error about wrong code")
	}
}

func TestPostResetPassword_ExpiredCode(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass", "Иванов Иван Иванович", models.RoleInspector)
	expiry := time.Now().Add(-1 * time.Minute) // уже истёк
	setResetToken(t, user.ID, "123456", expiry)

	form := url.Values{
		"email":    {"user@test.com"},
		"code":     {"123456"},
		"password": {"newpass789"},
		"confirm":  {"newpass789"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /reset-password expired: got %d, want 200 with error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "истёк") {
		t.Error("expected error about expired code")
	}
}

func TestPostResetPassword_PasswordMismatch(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass", "Иванов Иван Иванович", models.RoleInspector)
	expiry := time.Now().Add(15 * time.Minute)
	setResetToken(t, user.ID, "123456", expiry)

	form := url.Values{
		"email":    {"user@test.com"},
		"code":     {"123456"},
		"password": {"newpass789"},
		"confirm":  {"different000"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /reset-password mismatch: got %d, want 200 with error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "не совпадают") {
		t.Error("expected error about password mismatch")
	}
}

func TestPostResetPassword_ShortPassword(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "user@test.com", "oldpass", "Иванов Иван Иванович", models.RoleInspector)
	expiry := time.Now().Add(15 * time.Minute)
	setResetToken(t, user.ID, "123456", expiry)

	form := url.Values{
		"email":    {"user@test.com"},
		"code":     {"123456"},
		"password": {"abc"},
		"confirm":  {"abc"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /reset-password short: got %d, want 200 with error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "6") {
		t.Error("expected error about minimum password length")
	}
}

func TestPostResetPassword_UnknownEmail(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":    {"nobody@test.com"},
		"code":     {"123456"},
		"password": {"newpass789"},
		"confirm":  {"newpass789"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /reset-password unknown email: got %d, want 200 with error", w.Code)
	}
	if !strings.Contains(w.Body.String(), "не найден") {
		t.Error("expected error about user not found")
	}
}
