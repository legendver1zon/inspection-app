package handlers

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- GET /login ---

func TestGetLogin_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /login: got %d, want 200", w.Code)
	}
}

// --- POST /login ---

func TestPostLogin_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)

	form := url.Values{"email": {"user@test.com"}, "password": {"pass123"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /login success: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/inspections" {
		t.Errorf("POST /login redirect: got %q, want /inspections", loc)
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "token=") {
		t.Error("POST /login: expected token cookie in response")
	}
}

func TestPostLogin_WrongPassword(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	newUser(t, "user@test.com", "correctpass", "Иванов Иван Иванович", models.RoleInspector)

	form := url.Values{"email": {"user@test.com"}, "password": {"wrongpass"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /login wrong password: got %d, want 401", w.Code)
	}
}

func TestPostLogin_UnknownEmail(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{"email": {"nobody@test.com"}, "password": {"anypass"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST /login unknown email: got %d, want 401", w.Code)
	}
}

func TestPostLogin_EmptyFields(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{"email": {""}, "password": {""}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /login empty fields: got %d, want 400", w.Code)
	}
}

// --- POST /register ---

func TestPostRegister_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {"new@test.com"},
		"password":         {"pass123"},
		"confirm_password": {"pass123"},
		"full_name":        {"Петров Пётр Петрович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /register success: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("POST /register redirect: got %q, want /login...", loc)
	}
}

func TestPostRegister_FirstUserBecomesAdmin(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {"admin@test.com"},
		"password":         {"pass123"},
		"confirm_password": {"pass123"},
		"full_name":        {"Иванов Иван Иванович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("POST /register first user: got %d, want 302", w.Code)
	}

	var user models.User
	if err := storage.DB.Where("email = ?", "admin@test.com").First(&user).Error; err != nil {
		t.Fatalf("user not found after registration: %v", err)
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("first registered user: got role %q, want admin", user.Role)
	}
}

func TestPostRegister_PasswordMismatch(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {"user@test.com"},
		"password":         {"pass123"},
		"confirm_password": {"different"},
		"full_name":        {"Сидоров Сидор Сидорович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /register password mismatch: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Пароли не совпадают") {
		t.Error("expected error message about password mismatch")
	}
}

func TestPostRegister_ShortPassword(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {"user@test.com"},
		"password":         {"abc"},
		"confirm_password": {"abc"},
		"full_name":        {"Тестов Тест Тестович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /register short password: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "минимум 6") {
		t.Error("expected error about minimum password length")
	}
}

func TestPostRegister_DuplicateEmail(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	newUser(t, "taken@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)

	form := url.Values{
		"email":            {"taken@test.com"},
		"password":         {"pass123"},
		"confirm_password": {"pass123"},
		"full_name":        {"Другой Человек Иванович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /register duplicate email: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "уже существует") {
		t.Error("expected error about duplicate email")
	}
}

func TestPostRegister_EmptyFields(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {""},
		"password":         {""},
		"confirm_password": {""},
		"full_name":        {""},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /register empty fields: got %d, want 400", w.Code)
	}
}

// --- POST /logout ---

func TestPostLogout_ClearsCookieAndRedirects(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /logout: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("POST /logout redirect: got %q, want /login", loc)
	}
	// Cookie должна быть сброшена (Max-Age=-1)
	cookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "token=;") && !strings.Contains(cookie, "Max-Age=0") && !strings.Contains(cookie, "Max-Age=-1") {
		t.Errorf("POST /logout: expected token cookie cleared, got: %q", cookie)
	}
}

// --- Email case-insensitivity ---

// Регистрация с заглавным email → сохраняется в lowercase
func TestPostRegister_EmailStoredLowercase(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	form := url.Values{
		"email":            {"User@TEST.com"},
		"password":         {"pass123"},
		"confirm_password": {"pass123"},
		"full_name":        {"Тестов Тест Тестович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("POST /register uppercase email: got %d, want 302", w.Code)
	}
	var user models.User
	if err := storage.DB.Where("email = ?", "user@test.com").First(&user).Error; err != nil {
		t.Error("email should be stored in lowercase")
	}
}

// Повторная регистрация с другим регистром → ошибка "уже существует"
func TestPostRegister_DuplicateEmailCaseInsensitive(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	newUser(t, "taken@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)

	form := url.Values{
		"email":            {"TAKEN@TEST.COM"},
		"password":         {"pass123"},
		"confirm_password": {"pass123"},
		"full_name":        {"Другой Человек Иванович"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /register duplicate email (uppercase): got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "уже существует") {
		t.Error("expected error about duplicate email")
	}
}

// Вход с заглавным email → успех
func TestPostLogin_EmailCaseInsensitive(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	newUser(t, "user@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)

	form := url.Values{"email": {"USER@TEST.COM"}, "password": {"pass123"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /login uppercase email: got %d, want 302", w.Code)
	}
}

// --- buildInitials (внутренняя функция) ---

func TestBuildInitials(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Иванов Иван Иванович", "Иванов И. И."},
		{"Петров Пётр", "Петров П."},
		{"Сидоров", "Сидоров"},
		{"", ""},
	}
	for _, tc := range cases {
		got := buildInitials(tc.input)
		if got != tc.want {
			t.Errorf("buildInitials(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
