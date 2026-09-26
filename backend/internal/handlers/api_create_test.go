package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
)

func TestAPICreateInspection_CreatesDraftWithNumber(t *testing.T) {
	setupTestDB(t)
	resetAllLimiters()
	t.Cleanup(resetAllLimiters)
	user := newUser(t, "insp@test.com", "Test1234!", "Иванов Иван Иванович", models.RoleInspector)
	token := tokenFor(t, user.ID, "inspector")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/inspections", APIAuth(), APICreateInspection)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/inspections", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", w.Code, w.Body.String())
	}
	var body struct {
		ID        uint   `json:"id"`
		ActNumber string `json:"act_number"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID == 0 || !strings.HasPrefix(body.ActNumber, "1-") {
		t.Errorf("unexpected body %+v", body)
	}
	var insp models.Inspection
	if err := storage.DB.First(&insp, body.ID).Error; err != nil {
		t.Fatal("inspection not stored:", err)
	}
	if insp.UserID != user.ID || insp.Status != "draft" || insp.ActNumber != body.ActNumber {
		t.Errorf("stored inspection = %+v", insp)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/inspections", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("without cookie: want 401, got %d", w.Code)
	}
}
