package handlers

import (
	"bytes"
	"fmt"
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setupSecurityRouter — роутер с rate limiting middleware (как в production main.go).
func setupSecurityRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetHTMLTemplate(loadTemplates(t))

	r.GET("/login", GetLogin)
	r.POST("/login", security.RateLimitLogin(), PostLogin)
	r.GET("/register", GetRegister)
	r.POST("/register", security.RateLimitRegister(), PostRegister)
	r.GET("/forgot-password", GetForgotPassword)
	r.POST("/forgot-password", security.RateLimitForgotPassword(), PostForgotPassword)

	protected := r.Group("/")
	protected.Use(auth.RequireAuth())
	protected.Use(func(c *gin.Context) {
		userID := c.GetUint("userID")
		var u models.User
		if storage.DB.First(&u, userID).Error != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("userRole", string(u.Role))
		c.Next()
	})
	{
		protected.GET("/inspections/new", GetNewInspection)
		protected.POST("/profile/avatar", PostUploadAvatar)
		protected.POST("/inspections/:id/upload-plan", PostUploadPlan)
	}

	return r
}

// resetAllLimiters сбрасывает все rate limiter на свежие экземпляры.
// Вызывается в начале каждого теста rate limiting, чтобы избежать интерференции.
func resetAllLimiters() {
	security.LoginLimiter = security.NewMemoryRateLimiter(5, 15*time.Minute)
	security.RegisterLimiter = security.NewMemoryRateLimiter(3, time.Hour)
	security.ForgotPasswordLimiter = security.NewMemoryRateLimiter(3, time.Hour)
	security.InspectionLimiter = security.NewMemoryRateLimiter(20, time.Hour)
}

// postForm отправляет POST с application/x-www-form-urlencoded телом.
func postForm(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// postFormWithCookie отправляет POST с cookie авторизации.
func postFormWithCookie(r *gin.Engine, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// getWithCookie отправляет GET с cookie авторизации.
func getWithCookie(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== Rate Limiting ====================

// TestPostLogin_RateLimit — 5 неудачных попыток → 6-я блокируется (429).
func TestPostLogin_RateLimit(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()

	newUser(t, "ratelimit_login@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	r := setupSecurityRouter(t)

	// 5 неудачных попыток — все должны возвращать 401
	for i := 1; i <= 5; i++ {
		w := postForm(r, "/login", "email=ratelimit_login@test.com&password=WrongPass1!")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("попытка %d: ожидали 401, получили %d", i, w.Code)
		}
	}

	// 6-я попытка — должна быть заблокирована (429)
	w := postForm(r, "/login", "email=ratelimit_login@test.com&password=WrongPass1!")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("ожидали 429 после 5 неудачных попыток, получили %d; тело: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "мин") {
		t.Error("ответ 429 должен содержать время до разблокировки")
	}
}

// TestPostLogin_ResetOnSuccess — неудачи до блока, затем успех сбрасывает счётчик.
func TestPostLogin_ResetOnSuccess(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()

	newUser(t, "login_reset@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	r := setupSecurityRouter(t)

	// 3 неудачи
	for i := 0; i < 3; i++ {
		postForm(r, "/login", "email=login_reset@test.com&password=WrongPass1!")
	}

	// Успешный вход — сбрасывает счётчик
	w := postForm(r, "/login", "email=login_reset@test.com&password=Secret1!")
	if w.Code != http.StatusFound {
		t.Fatalf("успешный вход: ожидали 302, получили %d", w.Code)
	}

	// После сброса снова можно делать 5 попыток
	for i := 1; i <= 5; i++ {
		w = postForm(r, "/login", "email=login_reset@test.com&password=WrongPass1!")
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("после сброса попытка %d не должна быть заблокирована", i)
		}
	}
}

// TestPostRegister_RateLimit — 3 успешные регистрации → 4-я блокируется (429).
func TestPostRegister_RateLimit(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()

	r := setupSecurityRouter(t)

	for i := 1; i <= 3; i++ {
		body := fmt.Sprintf(
			"email=regrl%d@test.com&password=Secret1!&confirm_password=Secret1!&full_name=Тест+Тест+Тестов",
			i,
		)
		w := postForm(r, "/register", body)
		if w.Code != http.StatusFound {
			t.Fatalf("регистрация %d: ожидали 302, получили %d; тело: %s", i, w.Code, w.Body.String())
		}
	}

	// 4-я регистрация — должна быть заблокирована
	w := postForm(r, "/register",
		"email=regrl4@test.com&password=Secret1!&confirm_password=Secret1!&full_name=Тест+Тест+Тестов")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("ожидали 429 после 3 регистраций, получили %d; тело: %s", w.Code, w.Body.String())
	}
}

// TestPostForgotPassword_RateLimit — 3 запроса → 4-й блокируется (429).
func TestPostForgotPassword_RateLimit(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()

	r := setupSecurityRouter(t)

	// 3 запроса (email не обязан существовать — счётчик инкрементируется всегда)
	for i := 1; i <= 3; i++ {
		w := postForm(r, "/forgot-password", "email=noone@test.com")
		if w.Code != http.StatusOK {
			t.Fatalf("запрос %d: ожидали 200, получили %d", i, w.Code)
		}
	}

	// 4-й запрос — заблокирован
	w := postForm(r, "/forgot-password", "email=noone@test.com")
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("ожидали 429 после 3 запросов сброса пароля, получили %d; тело: %s", w.Code, w.Body.String())
	}
}

// ==================== Password Policy ====================

// TestPostRegister_PasswordPolicy — слабые пароли отклоняются с описанием проблемы.
func TestPostRegister_PasswordPolicy(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	r := setupSecurityRouter(t)

	cases := []struct {
		password string
		wantErr  string
	}{
		{"ab", "6"},            // слишком короткий
		{"secret1!", "заглавн"}, // нет заглавной
		{"Secret!", "цифр"},    // нет цифры
		{"Secret1", "спецсимвол"}, // нет спецсимвола
	}

	for _, tc := range cases {
		body := fmt.Sprintf(
			"email=pwtest@test.com&password=%s&confirm_password=%s&full_name=Тест+Тест+Тестов",
			tc.password, tc.password,
		)
		w := postForm(r, "/register", body)
		if w.Code == http.StatusFound {
			t.Errorf("пароль %q должен быть отклонён, но был принят", tc.password)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.wantErr) {
			t.Errorf("пароль %q: ожидали ошибку с %q, получили: %s", tc.password, tc.wantErr, w.Body.String())
		}
	}
}

// TestPostRegister_PasswordPolicy_StrongPass — сильный пароль принимается.
func TestPostRegister_PasswordPolicy_StrongPass(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	r := setupSecurityRouter(t)

	w := postForm(r, "/register",
		"email=strong@test.com&password=Secret1!&confirm_password=Secret1!&full_name=Тест+Тест+Тестов")
	if w.Code != http.StatusFound {
		t.Errorf("сильный пароль должен быть принят; код: %d; тело: %s", w.Code, w.Body.String())
	}
}

// ==================== MIME Validation ====================

// fakeMultipartFile создаёт multipart-запрос с файлом с произвольным содержимым.
func fakeMultipartFile(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	fw.Write(content)
	w.Close()
	return buf, w.FormDataContentType()
}

// TestPostUploadAvatar_MIMEValidation_RejectsNonImage — ZIP-файл с расширением .jpg отклоняется.
func TestPostUploadAvatar_MIMEValidation_RejectsNonImage(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "avatar_mime@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	r := setupSecurityRouter(t)

	// ZIP-сигнатура (PK\x03\x04) но расширение .jpg
	zipBytes := []byte{0x50, 0x4B, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00}
	body, ct := fakeMultipartFile(t, "avatar", "photo.jpg", zipBytes)

	req := httptest.NewRequest("POST", "/profile/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ZIP-файл с .jpg расширением должен быть отклонён (400), получили %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "изображени") {
		t.Errorf("ответ должен объяснять проблему, получили: %s", w.Body.String())
	}
}

// TestPostUploadAvatar_MIMEValidation_AcceptsJPEG — настоящий JPEG принимается.
func TestPostUploadAvatar_MIMEValidation_AcceptsJPEG(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "avatar_ok@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	r := setupSecurityRouter(t)

	// JPEG SOI маркер + минимальный APP0 header
	jpegBytes := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
	}
	body, ct := fakeMultipartFile(t, "avatar", "photo.jpg", jpegBytes)

	req := httptest.NewRequest("POST", "/profile/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем редирект на /profile (302) — файл принят
	if w.Code != http.StatusFound {
		t.Errorf("JPEG должен быть принят (302), получили %d; тело: %s", w.Code, w.Body.String())
	}
}

// TestPostUploadAvatar_MIMEValidation_RejectsWrongExt — правильный MIME но неверное расширение (.exe).
func TestPostUploadAvatar_MIMEValidation_RejectsWrongExt(t *testing.T) {
	setupTestDB(t)
	user := newUser(t, "avatar_ext@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	r := setupSecurityRouter(t)

	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	body, ct := fakeMultipartFile(t, "avatar", "virus.exe", jpegBytes)

	req := httptest.NewRequest("POST", "/profile/avatar", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("файл .exe должен быть отклонён (400), получили %d", w.Code)
	}
}

// ==================== Inspection Rate Limit ====================

// TestGetNewInspection_InspectionLimit — после исчерпания лимита создание актов блокируется.
func TestGetNewInspection_InspectionLimit(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	// Используем лимит 3 вместо 20, чтобы не создавать 20 записей в БД
	security.InspectionLimiter = security.NewMemoryRateLimiter(3, time.Hour)

	user := newUser(t, "insp_limit@test.com", "Secret1!", "Тест Тест Тестов", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")
	r := setupSecurityRouter(t)

	// 3 создания — все должны успешно перенаправлять на edit (302)
	for i := 1; i <= 3; i++ {
		w := getWithCookie(r, "/inspections/new", tok)
		if w.Code != http.StatusFound {
			t.Fatalf("создание %d: ожидали 302, получили %d; тело: %s", i, w.Code, w.Body.String())
		}
	}

	// 4-е создание — должно быть заблокировано (429)
	w := getWithCookie(r, "/inspections/new", tok)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("ожидали 429 после %d актов, получили %d; тело: %s", 3, w.Code, w.Body.String())
	}
}

// TestGetNewInspection_AdminBypassesLimit — admin создаёт акты без ограничений.
func TestGetNewInspection_AdminBypassesLimit(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	security.InspectionLimiter = security.NewMemoryRateLimiter(1, time.Hour) // лимит 1 для обычных

	admin := newUser(t, "admin_limit@test.com", "Secret1!", "Админ Админ Adminов", models.RoleAdmin)
	tok := tokenFor(t, admin.ID, "admin")
	r := setupSecurityRouter(t)

	// Admin без ограничений — 5 создания без блокировки
	for i := 1; i <= 5; i++ {
		w := getWithCookie(r, "/inspections/new", tok)
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("admin не должен блокироваться при создании акта %d", i)
		}
		if w.Code != http.StatusFound {
			t.Errorf("создание акта %d: ожидали 302, получили %d", i, w.Code)
		}
	}
}
