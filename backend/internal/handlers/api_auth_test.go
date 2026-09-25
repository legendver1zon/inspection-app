package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

func apiAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/register", security.RateLimitRegisterJSON(), APIRegister)
	r.POST("/api/forgot-password", security.RateLimitForgotPasswordJSON(), APIForgotPassword)
	r.POST("/api/reset-password", security.RateLimitResetPasswordJSON(), APIResetPassword)
	return r
}

func postJSON(r *gin.Engine, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var parsed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return w, parsed
}

func TestAPIRegister_Success(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	r := apiAuthRouter()

	w, body := postJSON(r, "/api/register", `{"email":"New@Test.com","password":"Test1234!","confirm_password":"Test1234!","full_name":"Петров Пётр Петрович"}`)
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("want 200 ok, got %d %s", w.Code, w.Body.String())
	}
	var u models.User
	if err := storage.DB.Where("email = ?", "new@test.com").First(&u).Error; err != nil {
		t.Fatal("user not created:", err)
	}
	if u.Initials != "Петров П.П." && u.Initials == "" {
		t.Errorf("initials not filled: %q", u.Initials)
	}
	if !auth.CheckPassword("Test1234!", u.PasswordHash) {
		t.Error("password hash mismatch")
	}
}

func TestAPIRegister_ValidationErrors(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	r := apiAuthRouter()
	newUser(t, "taken@test.com", "Test1234!", "Иванов Иван Иванович", models.RoleInspector)

	cases := map[string]string{
		`{"email":"a@b.c","password":"Test1234!","confirm_password":"Test1234!","full_name":"Иванов Иван"}`:                   "Введите полное ФИО",
		`{"email":"a@b.c","password":"Test1234!","confirm_password":"Test1234!","full_name":"Иванов","no_patronymic":true}`:   "Введите Фамилию и Имя",
		`{"email":"a@b.c","password":"Test1234!","confirm_password":"other","full_name":"Иванов Иван Иванович"}`:              "Пароли не совпадают",
		`{"email":"a@b.c","password":"123","confirm_password":"123","full_name":"Иванов Иван Иванович"}`:                      "минимум 6 символов",
		`{"email":"taken@test.com","password":"Test1234!","confirm_password":"Test1234!","full_name":"Иванов Иван Иванович"}`: "уже существует",
		`{"email":"","password":"","confirm_password":"","full_name":""}`:                                                     "Заполните все поля",
	}
	for body, want := range cases {
		w, parsed := postJSON(r, "/api/register", body)
		msg, _ := parsed["error"].(string)
		if w.Code != http.StatusBadRequest || !strings.Contains(msg, want) {
			t.Errorf("%s: want 400 %q, got %d %q", body, want, w.Code, msg)
		}
	}
}

func TestAPIForgotPassword_AlwaysOK_SetsCode(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	r := apiAuthRouter()
	user := newUser(t, "user@test.com", "Test1234!", "Иванов Иван Иванович", models.RoleInspector)

	w, body := postJSON(r, "/api/forgot-password", `{"email":"USER@test.com"}`)
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Fatalf("known email: want 200 ok, got %d %s", w.Code, w.Body.String())
	}
	var updated models.User
	storage.DB.First(&updated, user.ID)
	if len(updated.ResetToken) != 6 || updated.ResetExpiry == nil {
		t.Errorf("reset code not set: %q %v", updated.ResetToken, updated.ResetExpiry)
	}

	w, body = postJSON(r, "/api/forgot-password", `{"email":"nobody@test.com"}`)
	if w.Code != http.StatusOK || body["ok"] != true {
		t.Errorf("unknown email must look identical: got %d %s", w.Code, w.Body.String())
	}
	w, _ = postJSON(r, "/api/forgot-password", `{"email":""}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty email: want 400, got %d", w.Code)
	}
}

func TestAPIResetPassword_Flow(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	r := apiAuthRouter()
	user := newUser(t, "user@test.com", "Old1234!", "Иванов Иван Иванович", models.RoleInspector)
	setResetToken(t, user.ID, "123456", time.Now().Add(10*time.Minute))

	w, parsed := postJSON(r, "/api/reset-password", `{"email":"user@test.com","code":"000000","password":"New1234!","confirm":"New1234!"}`)
	if w.Code != http.StatusBadRequest || parsed["error"] != "Неверный код" {
		t.Fatalf("wrong code: got %d %s", w.Code, w.Body.String())
	}
	w, parsed = postJSON(r, "/api/reset-password", `{"email":"user@test.com","code":"123456","password":"New1234!","confirm":"nope"}`)
	if w.Code != http.StatusBadRequest || parsed["error"] != "Пароли не совпадают" {
		t.Fatalf("mismatch: got %d %s", w.Code, w.Body.String())
	}
	w, parsed = postJSON(r, "/api/reset-password", `{"email":"user@test.com","code":"123456","password":"New1234!","confirm":"New1234!"}`)
	if w.Code != http.StatusOK || parsed["ok"] != true {
		t.Fatalf("success: got %d %s", w.Code, w.Body.String())
	}
	var updated models.User
	storage.DB.First(&updated, user.ID)
	if !auth.CheckPassword("New1234!", updated.PasswordHash) || updated.ResetToken != "" {
		t.Error("password not changed or code not cleared")
	}

	setResetToken(t, user.ID, "654321", time.Now().Add(-time.Minute))
	w, parsed = postJSON(r, "/api/reset-password", `{"email":"user@test.com","code":"654321","password":"New1234!","confirm":"New1234!"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(parsed["error"].(string), "истёк") {
		t.Errorf("expired: got %d %s", w.Code, w.Body.String())
	}
}

func TestAPIResetPassword_RateLimitJSON(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	r := apiAuthRouter()
	newUser(t, "user@test.com", "Old1234!", "Иванов Иван Иванович", models.RoleInspector)

	var last *httptest.ResponseRecorder
	for i := 0; i < 7; i++ {
		last, _ = postJSON(r, "/api/reset-password", `{"email":"user@test.com","code":"000000","password":"New1234!","confirm":"New1234!"}`)
	}
	if last.Code != http.StatusTooManyRequests || !strings.Contains(last.Body.String(), `"error"`) {
		t.Errorf("want 429 JSON after repeated wrong codes, got %d %s", last.Code, last.Body.String())
	}
}
