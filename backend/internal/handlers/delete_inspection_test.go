package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

func deleteRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/inspections/:id/delete", auth.RequireAuth(), PostDeleteInspection)
	return r
}

func postDelete(r *gin.Engine, id uint, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/inspections/"+strconv.Itoa(int(id))+"/delete", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	return w
}

func TestPostDeleteInspection_Permissions(t *testing.T) {
	setupTestDB(t)
	owner := newUser(t, "owner@test.com", "Test1234!", "Иванов Иван Иванович", models.RoleInspector)
	other := newUser(t, "other@test.com", "Test1234!", "Петров Пётр Петрович", models.RoleInspector)
	admin := newUser(t, "admin@test.com", "Test1234!", "Админов Админ Админович", models.RoleAdmin)
	r := deleteRouter()

	draft := models.Inspection{ActNumber: "1-010126", UserID: owner.ID, Status: "draft"}
	storage.DB.Create(&draft)
	done := models.Inspection{ActNumber: "2-010126", UserID: owner.ID, Status: "completed"}
	storage.DB.Create(&done)
	foreign := models.Inspection{ActNumber: "3-010126", UserID: other.ID, Status: "draft"}
	storage.DB.Create(&foreign)

	ownerTok := tokenFor(t, owner.ID, "inspector")
	adminTok := tokenFor(t, admin.ID, "admin")

	if w := postDelete(r, foreign.ID, ownerTok); w.Code != http.StatusForbidden {
		t.Errorf("stranger: want 403, got %d %s", w.Code, w.Body.String())
	}
	if w := postDelete(r, done.ID, ownerTok); w.Code != http.StatusForbidden {
		t.Errorf("own completed: want 403, got %d %s", w.Code, w.Body.String())
	}
	if w := postDelete(r, draft.ID, ownerTok); w.Code != http.StatusOK {
		t.Errorf("own draft: want 200, got %d %s", w.Code, w.Body.String())
	}
	if err := storage.DB.First(&models.Inspection{}, draft.ID).Error; err == nil {
		t.Error("own draft still exists")
	}
	if w := postDelete(r, done.ID, adminTok); w.Code != http.StatusOK {
		t.Errorf("admin completed: want 200, got %d %s", w.Code, w.Body.String())
	}
	if w := postDelete(r, foreign.ID, adminTok); w.Code != http.StatusOK {
		t.Errorf("admin foreign: want 200, got %d %s", w.Code, w.Body.String())
	}

	// Без X-Requested-With — редирект, как у старой формы
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/inspections/"+strconv.Itoa(int(draft.ID))+"/delete", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: adminTok})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("already deleted: want 404, got %d", w.Code)
	}
}
