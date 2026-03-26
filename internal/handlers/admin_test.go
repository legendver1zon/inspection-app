package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// --- GET /admin/users ---

func TestGetAdminUsers_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /admin/users: got %d, want 200", w.Code)
	}
}

func TestGetAdminUsers_ForbiddenForInspector(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	inspector := newUser(t, "inspector@test.com", "pass123", "Петров Пётр Петрович", models.RoleInspector)
	token := tokenFor(t, inspector.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound && w.Code != http.StatusForbidden {
		t.Errorf("GET /admin/users for inspector: got %d, want 302 or 403", w.Code)
	}
}

// --- GET /admin/users/:id/edit ---

func TestGetAdminEditUser_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/users/"+itoa(target.ID)+"/edit", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /admin/users/:id/edit: got %d, want 200", w.Code)
	}
}

func TestGetAdminEditUser_NotFound_Redirects(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/users/99999/edit", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("GET /admin/users/99999/edit: got %d, want 302", w.Code)
	}
}

// --- POST /admin/users/:id/edit ---

func TestPostAdminEditUser_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Старый Старый Старый", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{
		"full_name": {"Новый Новый Новый"},
		"email":     {"target@test.com"},
		"role":      {"inspector"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /admin/users/:id/edit success: got %d, want 302", w.Code)
	}

	var updated models.User
	storage.DB.First(&updated, target.ID)
	if updated.FullName != "Новый Новый Новый" {
		t.Errorf("full_name not updated: got %q", updated.FullName)
	}
}

func TestPostAdminEditUser_EmptyFields(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{
		"full_name": {""},
		"email":     {""},
		"role":      {"inspector"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /admin/users/:id/edit empty fields: got %d, want 400", w.Code)
	}
}

func TestPostAdminEditUser_ShortPassword(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{
		"full_name":    {"Сидоров Сидор Сидорович"},
		"email":        {"target@test.com"},
		"role":         {"inspector"},
		"new_password": {"abc"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /admin/users/:id/edit short password: got %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "6") {
		t.Error("expected error about minimum password length")
	}
}

func TestPostAdminEditUser_InvalidRole(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{
		"full_name": {"Сидоров Сидор Сидорович"},
		"email":     {"target@test.com"},
		"role":      {"superuser"},
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /admin/users/:id/edit invalid role: got %d, want 400", w.Code)
	}
}

// --- POST /admin/users/:id/role ---

func TestPostAdminChangeRole_SetAdmin(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{"role": {"admin"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/role", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /admin/users/:id/role: got %d, want 302", w.Code)
	}

	var updated models.User
	storage.DB.First(&updated, target.ID)
	if updated.Role != models.RoleAdmin {
		t.Errorf("role not updated: got %q, want admin", updated.Role)
	}
}

func TestPostAdminChangeRole_InvalidRole(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	form := url.Values{"role": {"superuser"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/role", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /admin/users/:id/role invalid: got %d, want 400", w.Code)
	}
}

// --- POST /admin/users/:id/delete ---

func TestDeleteAdminUser_Success(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	target := newUser(t, "target@test.com", "pass123", "Сидоров Сидор Сидорович", models.RoleInspector)
	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(target.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("POST /admin/users/:id/delete: got %d, want 302", w.Code)
	}

	var count int64
	storage.DB.Model(&models.User{}).Where("id = ?", target.ID).Count(&count)
	if count != 0 {
		t.Error("user was not deleted")
	}
}

func TestDeleteAdminUser_CannotDeleteSelf(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass123", "Иванов Иван Иванович", models.RoleAdmin)
	token := tokenFor(t, admin.ID, "admin")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/users/"+itoa(admin.ID)+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("POST /admin/users/:id/delete self: got %d, want 400", w.Code)
	}

	var count int64
	storage.DB.Model(&models.User{}).Where("id = ?", admin.ID).Count(&count)
	if count == 0 {
		t.Error("admin should not be deleted")
	}
}

// --- вспомогательная функция ---

func itoa(id uint) string {
	return fmt.Sprintf("%d", id)
}
