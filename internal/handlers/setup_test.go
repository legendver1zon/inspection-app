package handlers

import (
	"fmt"
	"html/template"
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/security"
	"inspection-app/internal/storage"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

var testDBOnce sync.Once

// wallRow используется в funcMap для шаблонов (аналогично main.go)
type wallRow struct {
	Name, W1, W2, W3, W4 string
}

// setupTestDB подключается к тестовой PostgreSQL и сбрасывает схему.
// Требует переменную окружения TEST_DATABASE_URL.
// Если не задана — тест пропускается.
func setupTestDB(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL не задан; запустите: docker compose up postgres -d && export TEST_DATABASE_URL=postgres://inspection:secret@localhost:5432/inspection_test?sslmode=disable")
	}
	// Подключаемся к БД только один раз для всего тестового прогона,
	// чтобы не исчерпать лимит соединений PostgreSQL при большом числе тестов.
	testDBOnce.Do(func() {
		storage.Connect(dsn)
		security.Init()
	})
	// Сбрасываем все таблицы и накатываем миграции заново — полная изоляция
	storage.DB.Exec("DROP SCHEMA public CASCADE")
	storage.DB.Exec("CREATE SCHEMA public")
	storage.Migrate()
}

// setupRouter создаёт тестовый Gin-роутер с шаблонами и маршрутами.
func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.SetHTMLTemplate(loadTemplates(t))

	// Публичные маршруты
	r.GET("/login", GetLogin)
	r.POST("/login", PostLogin)
	r.GET("/register", GetRegister)
	r.POST("/register", PostRegister)
	r.POST("/logout", PostLogout)

	// Защищённые маршруты
	protected := r.Group("/")
	protected.Use(auth.RequireAuth())
	protected.Use(func(c *gin.Context) {
		userID := c.GetUint("userID")
		var u models.User
		if storage.DB.First(&u, userID).Error != nil {
			c.SetCookie("token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	})
	{
		protected.GET("/inspections", GetInspections)
		protected.GET("/inspections/new", GetNewInspection)
		protected.GET("/inspections/:id", GetInspection)
		protected.GET("/inspections/:id/edit", GetEditInspection)
		protected.POST("/inspections/:id/edit", PostEditInspection)
		protected.POST("/inspections/:id/generate", PostGenerateDocument)

		protected.POST("/documents/:id/delete", PostDeleteDocument)
		protected.GET("/documents/:id/download", GetDownloadDocument)

		protected.GET("/profile", GetProfile)
		protected.POST("/profile", PostProfile)

		admin := protected.Group("/admin")
		admin.Use(auth.RequireAdmin())
		{
			admin.GET("/users", GetAdminUsers)
			admin.GET("/users/:id/edit", GetAdminEditUser)
			admin.POST("/users/:id/edit", PostAdminEditUser)
			admin.POST("/users/:id/role", PostAdminChangeRole)
			admin.POST("/users/:id/delete", DeleteAdminUser)
			admin.POST("/inspections/:id/delete", PostDeleteInspection)
		}
	}

	// Публичные маршруты для reset/forgot (без RequireAuth)
	r.GET("/forgot-password", GetForgotPassword)
	r.POST("/forgot-password", PostForgotPassword)
	r.GET("/reset-password", GetResetPassword)
	r.POST("/reset-password", PostResetPassword)

	return r
}

func loadTemplates(t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.New("").Funcs(testFuncMap())
	tmpl = template.Must(tmpl.ParseGlob("../../web/templates/partials/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("../../web/templates/auth/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("../../web/templates/inspections/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("../../web/templates/admin/*.html"))
	return tmpl
}

// testFuncMap — копия funcMap из main.go для использования в тестах.
func testFuncMap() template.FuncMap {
	return template.FuncMap{
		"string": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
		"initials2": func(s string) string {
			parts := strings.Fields(s)
			result := ""
			for i, p := range parts {
				if i >= 2 {
					break
				}
				r := []rune(p)
				if len(r) > 0 {
					result += string(r[0])
				}
			}
			if result == "" {
				return "?"
			}
			return result
		},
		"defectVal": func(roomMap map[int]*models.InspectionRoom, roomNum int, templateID uint, wallNum int) string {
			if room, ok := roomMap[roomNum]; ok && room != nil {
				for _, d := range room.Defects {
					if d.DefectTemplateID != nil && *d.DefectTemplateID == templateID && d.WallNumber == wallNum {
						return d.Value
					}
				}
			}
			return ""
		},
		"notesVal": func(roomMap map[int]*models.InspectionRoom, roomNum int, section string) string {
			if room, ok := roomMap[roomNum]; ok && room != nil {
				for _, d := range room.Defects {
					if d.DefectTemplateID == nil && d.Section == section {
						return d.Notes
					}
				}
			}
			return ""
		},
		"roomField": func(roomMap map[int]*models.InspectionRoom, roomNum int, field string) string {
			if room, ok := roomMap[roomNum]; ok && room != nil {
				switch field {
				case "name":
					return room.RoomName
				case "window_type":
					return room.WindowType
				case "wall_type":
					return room.WallType
				case "length":
					if room.Length != 0 {
						return fmt.Sprintf("%g", room.Length)
					}
				case "width":
					if room.Width != 0 {
						return fmt.Sprintf("%g", room.Width)
					}
				case "height":
					if room.Height != 0 {
						return fmt.Sprintf("%g", room.Height)
					}
				}
			}
			return ""
		},
		"roomExists": func(roomMap map[int]*models.InspectionRoom, roomNum int) bool {
			room, ok := roomMap[roomNum]
			return ok && room != nil
		},
		"add": func(a, b int) int { return a + b },
		"roomHasDefects": func(room models.InspectionRoom) bool {
			for _, d := range room.Defects {
				if d.Value != "" || d.Notes != "" {
					return true
				}
			}
			return false
		},
		"roomHasSection": func(room models.InspectionRoom, section string) bool {
			for _, d := range room.Defects {
				if d.Section == section && (d.Value != "" || d.Notes != "") {
					return true
				}
			}
			return false
		},
		"sectionDefects": func(room models.InspectionRoom, section string) []models.RoomDefect {
			var result []models.RoomDefect
			for _, d := range room.Defects {
				if d.Section == section && d.Notes == "" && d.Value != "" {
					result = append(result, d)
				}
			}
			return result
		},
		"sectionNotes": func(room models.InspectionRoom, section string) string {
			for _, d := range room.Defects {
				if d.Section == section && d.Notes != "" {
					return d.Notes
				}
			}
			return ""
		},
		"wallRows": func(room models.InspectionRoom) []wallRow {
			return nil
		},
		"windowTypeName": func(t string) string {
			switch t {
			case "pvc":
				return "ПВХ"
			case "al":
				return "Al"
			case "wood":
				return "Дерево"
			}
			return ""
		},
		"wallTypeName": func(t string) string {
			var names []string
			for _, p := range strings.Split(t, ",") {
				switch strings.TrimSpace(p) {
				case "paint":
					names = append(names, "Окраска")
				case "tile":
					names = append(names, "Плитка")
				case "gkl":
					names = append(names, "ГКЛ")
				}
			}
			return strings.Join(names, "/")
		},
		"hasWallType": func(wallType, checkType string) bool {
			for _, p := range strings.Split(wallType, ",") {
				if strings.TrimSpace(p) == checkType {
					return true
				}
			}
			return false
		},
	}
}

// --- Вспомогательные фабрики тестовых данных ---

func newUser(t *testing.T, email, password, fullName string, role models.Role) models.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	user := models.User{
		Email:        email,
		PasswordHash: hash,
		FullName:     fullName,
		Initials:     buildInitials(fullName),
		Role:         role,
	}
	if err := storage.DB.Create(&user).Error; err != nil {
		t.Fatalf("newUser: %v", err)
	}
	return user
}

func newDefectTemplate(t *testing.T, section, name string) models.DefectTemplate {
	t.Helper()
	tmpl := models.DefectTemplate{Section: section, Name: name}
	if err := storage.DB.Create(&tmpl).Error; err != nil {
		t.Fatalf("newDefectTemplate: %v", err)
	}
	return tmpl
}

func newInspection(t *testing.T, userID uint, address, ownerName, status string, date time.Time) models.Inspection {
	t.Helper()
	insp := models.Inspection{
		ActNumber: fmt.Sprintf("ACT-%d", time.Now().UnixNano()),
		UserID:    userID,
		Date:      date,
		Address:   address,
		OwnerName: ownerName,
		Status:    status,
	}
	if err := storage.DB.Create(&insp).Error; err != nil {
		t.Fatalf("newInspection: %v", err)
	}
	return insp
}

func tokenFor(t *testing.T, userID uint, role string) string {
	t.Helper()
	tok, err := auth.GenerateToken(userID, role)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	return tok
}
