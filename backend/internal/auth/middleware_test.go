package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func protectedRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/secret", RequireAuth(), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func signedAt(t *testing.T, issued time.Time) string {
	t.Helper()
	claims := Claims{UserID: 1, Role: "inspector", RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(issued.Add(SessionTTL)),
		IssuedAt:  jwt.NewNumericDate(issued),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRequireAuth_NoCookie_NavigationRedirects(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secret", nil)
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	protectedRouter().ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Fatalf("want 302 /login, got %d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestRequireAuth_NoCookie_FetchGets401JSON(t *testing.T) {
	for _, h := range []http.Header{
		{"Sec-Fetch-Mode": {"cors"}},
		{"X-Requested-With": {"XMLHttpRequest"}},
		{"Accept": {"application/json"}},
	} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/secret", nil)
		req.Header = h
		protectedRouter().ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Сессия истекла") {
			t.Errorf("headers %v: want 401 JSON, got %d %s", h, w.Code, w.Body.String())
		}
	}
}

func TestRequireAuth_FreshToken_NoRenewal(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secret", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: signedAt(t, time.Now())})
	protectedRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Errorf("fresh token must not be reissued, got %v", w.Result().Cookies())
	}
}

func TestRequireAuth_OldToken_Renewed(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secret", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: signedAt(t, time.Now().Add(-2*RenewAfter))})
	protectedRouter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var renewed *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "token" {
			renewed = ck
		}
	}
	if renewed == nil || renewed.Value == "" {
		t.Fatal("expected reissued token cookie")
	}
	if renewed.MaxAge != int(SessionTTL.Seconds()) {
		t.Errorf("cookie MaxAge = %d, want %d", renewed.MaxAge, int(SessionTTL.Seconds()))
	}
	claims, err := ParseToken(renewed.Value)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(claims.ExpiresAt.Time) < SessionTTL-time.Minute {
		t.Errorf("renewed token must expire in ~%v, got %v", SessionTTL, time.Until(claims.ExpiresAt.Time))
	}
}

func TestRequireAuth_ExpiredToken_ClearsCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/secret", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: signedAt(t, time.Now().Add(-SessionTTL-time.Hour))})
	req.Header.Set("Sec-Fetch-Mode", "cors")
	protectedRouter().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "token" && ck.MaxAge >= 0 {
			t.Errorf("expired token cookie must be cleared, got MaxAge %d", ck.MaxAge)
		}
	}
}

func TestShouldRenew(t *testing.T) {
	if ShouldRenew(&Claims{RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())}}) {
		t.Error("fresh claims must not renew")
	}
	if !ShouldRenew(&Claims{RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now().Add(-25 * time.Hour))}}) {
		t.Error("day-old claims must renew")
	}
	if !ShouldRenew(&Claims{}) {
		t.Error("claims without iat must renew")
	}
}

func TestGenerateToken_SessionTTL(t *testing.T) {
	tok, err := GenerateToken(7, "admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	left := time.Until(claims.ExpiresAt.Time)
	if left < SessionTTL-time.Minute || left > SessionTTL {
		t.Errorf("token lifetime = %v, want ~%v", left, SessionTTL)
	}
}
