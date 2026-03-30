package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsProduction_Release(t *testing.T) {
	os.Setenv("GIN_MODE", "release")
	defer os.Unsetenv("GIN_MODE")

	if !IsProduction() {
		t.Error("IsProduction должен возвращать true при GIN_MODE=release")
	}
}

func TestIsProduction_Debug(t *testing.T) {
	os.Setenv("GIN_MODE", "debug")
	defer os.Unsetenv("GIN_MODE")

	if IsProduction() {
		t.Error("IsProduction должен возвращать false при GIN_MODE=debug")
	}
}

func TestIsProduction_Empty(t *testing.T) {
	os.Unsetenv("GIN_MODE")

	if IsProduction() {
		t.Error("IsProduction должен возвращать false при пустом GIN_MODE")
	}
}

func TestSetAuthCookie_SetsHttpOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	os.Unsetenv("GIN_MODE")
	SetAuthCookie(c, "test-token-123")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}

	cookie := cookies[0]
	if cookie.Name != "token" {
		t.Errorf("Ожидалось имя cookie 'token', получено '%s'", cookie.Name)
	}
	if cookie.Value != "test-token-123" {
		t.Errorf("Ожидалось значение 'test-token-123', получено '%s'", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Error("Cookie должна быть HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("Ожидался SameSite=Lax, получено %v", cookie.SameSite)
	}
}

func TestSetAuthCookie_SecureInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	os.Setenv("GIN_MODE", "release")
	defer os.Unsetenv("GIN_MODE")

	SetAuthCookie(c, "prod-token")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if !cookies[0].Secure {
		t.Error("В production cookie должна быть Secure")
	}
}

func TestClearAuthCookie_RemovesCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	os.Unsetenv("GIN_MODE")
	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("MaxAge должен быть -1 для удаления, получено %d", cookies[0].MaxAge)
	}
}
