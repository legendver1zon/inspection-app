package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestContext() (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return w, c
}

// --- IsProduction ---

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

// --- isSecureCookie ---

func TestIsSecureCookie_True(t *testing.T) {
	os.Setenv("COOKIE_SECURE", "true")
	defer os.Unsetenv("COOKIE_SECURE")

	if !isSecureCookie() {
		t.Error("isSecureCookie должен возвращать true при COOKIE_SECURE=true")
	}
}

func TestIsSecureCookie_False(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")

	if isSecureCookie() {
		t.Error("isSecureCookie должен возвращать false по умолчанию")
	}
}

func TestIsSecureCookie_InvalidValue(t *testing.T) {
	os.Setenv("COOKIE_SECURE", "yes")
	defer os.Unsetenv("COOKIE_SECURE")

	if isSecureCookie() {
		t.Error("isSecureCookie должен возвращать false при значении != 'true'")
	}
}

// --- SetAuthCookie ---

func TestSetAuthCookie_SetsHttpOnly(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

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
}

func TestSetAuthCookie_SameSiteLax(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	SetAuthCookie(c, "token-abc")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("Ожидался SameSite=Lax, получено %v", cookies[0].SameSite)
	}
}

func TestSetAuthCookie_NotSecureByDefault(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	SetAuthCookie(c, "http-token")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].Secure {
		t.Error("Cookie не должна быть Secure без COOKIE_SECURE=true (HTTP-сайт)")
	}
}

func TestSetAuthCookie_SecureWhenExplicit(t *testing.T) {
	os.Setenv("COOKIE_SECURE", "true")
	defer os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	SetAuthCookie(c, "https-token")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if !cookies[0].Secure {
		t.Error("Cookie должна быть Secure при COOKIE_SECURE=true")
	}
}

func TestSetAuthCookie_MaxAge24Hours(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	SetAuthCookie(c, "token-xyz")

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].MaxAge != 86400 {
		t.Errorf("MaxAge должен быть 86400 (24ч), получено %d", cookies[0].MaxAge)
	}
}

// --- ClearAuthCookie ---

func TestClearAuthCookie_RemovesCookie(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("MaxAge должен быть -1 для удаления, получено %d", cookies[0].MaxAge)
	}
}

func TestClearAuthCookie_SameSiteLax(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Errorf("Ожидался SameSite=Lax при очистке, получено %v", cookies[0].SameSite)
	}
}

func TestClearAuthCookie_PathRoot(t *testing.T) {
	os.Unsetenv("COOKIE_SECURE")
	w, c := newTestContext()

	ClearAuthCookie(c)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("Cookie не установлена")
	}
	if cookies[0].Path != "/" {
		t.Errorf("Path должен быть '/', получено '%s'", cookies[0].Path)
	}
}
