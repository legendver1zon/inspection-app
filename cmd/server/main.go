package main

import (
	"fmt"
	"html/template"
	"inspection-app/internal/auth"
	"inspection-app/internal/handlers"
	"inspection-app/internal/models"
	"inspection-app/internal/seed"
	"inspection-app/internal/storage"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type wallRow struct {
	Name, W1, W2, W3, W4 string
}

func dbPath() string {
	if p := os.Getenv("DB_PATH"); p != "" {
		return p
	}
	return "inspection.db"
}

func main() {
	storage.Connect(dbPath())
	storage.Migrate()
	seed.SeedDefects()

	r := gin.Default()

	funcMap := template.FuncMap{
		"string": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
		// defectVal — значение дефекта для комнаты из roomMap
		"defectVal": func(roomMap map[int]*models.InspectionRoom, roomNum int, templateID uint, wallNum int) string {
			if room, ok := roomMap[roomNum]; ok && room != nil {
				for _, d := range room.Defects {
					if d.DefectTemplateID == templateID && d.WallNumber == wallNum {
						return d.Value
					}
				}
			}
			return ""
		},
		// notesVal — текст "Прочее" для секции комнаты
		"notesVal": func(roomMap map[int]*models.InspectionRoom, roomNum int, section string) string {
			if room, ok := roomMap[roomNum]; ok && room != nil {
				for _, d := range room.Defects {
					if d.DefectTemplateID == 0 && d.Section == section {
						return d.Notes
					}
				}
			}
			return ""
		},
		// roomField — поле замеров/типов комнаты
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
				case "w1h":
					if room.Window1Height != 0 {
						return fmt.Sprintf("%g", room.Window1Height)
					}
				case "w1w":
					if room.Window1Width != 0 {
						return fmt.Sprintf("%g", room.Window1Width)
					}
				case "w2h":
					if room.Window2Height != 0 {
						return fmt.Sprintf("%g", room.Window2Height)
					}
				case "w2w":
					if room.Window2Width != 0 {
						return fmt.Sprintf("%g", room.Window2Width)
					}
				case "dh":
					if room.DoorHeight != 0 {
						return fmt.Sprintf("%g", room.DoorHeight)
					}
				case "dw":
					if room.DoorWidth != 0 {
						return fmt.Sprintf("%g", room.DoorWidth)
					}
				}
			}
			return ""
		},
		// roomExists — есть ли комната в roomMap
		"roomExists": func(roomMap map[int]*models.InspectionRoom, roomNum int) bool {
			room, ok := roomMap[roomNum]
			return ok && room != nil
		},
		// add — сложение для шаблонов
		"add": func(a, b int) int { return a + b },

		// roomHasDefects — есть ли хоть один дефект в помещении
		"roomHasDefects": func(room models.InspectionRoom) bool {
			for _, d := range room.Defects {
				if d.Value != "" || d.Notes != "" {
					return true
				}
			}
			return false
		},

		// roomHasSection — есть ли данные в секции помещения
		"roomHasSection": func(room models.InspectionRoom, section string) bool {
			for _, d := range room.Defects {
				if d.Section == section && (d.Value != "" || d.Notes != "") {
					return true
				}
			}
			return false
		},

		// sectionDefects — дефекты секции (только с Value, без Notes)
		"sectionDefects": func(room models.InspectionRoom, section string) []models.RoomDefect {
			var result []models.RoomDefect
			for _, d := range room.Defects {
				if d.Section == section && d.Notes == "" && d.Value != "" {
					result = append(result, d)
				}
			}
			return result
		},

		// sectionNotes — текст "Прочее" для секции
		"sectionNotes": func(room models.InspectionRoom, section string) string {
			for _, d := range room.Defects {
				if d.Section == section && d.Notes != "" {
					return d.Notes
				}
			}
			return ""
		},

		// wallRows — дефекты стен, сгруппированные по шаблону (для таблицы ст1-ст4)
		"wallRows": func(room models.InspectionRoom) []wallRow {
			type entry struct {
				name   string
				values [5]string
			}
			entries := make(map[uint]*entry)
			order := []uint{}
			for _, d := range room.Defects {
				if d.Section != "wall" || d.Notes != "" || d.WallNumber < 1 || d.WallNumber > 4 {
					continue
				}
				if _, ok := entries[d.DefectTemplateID]; !ok {
					entries[d.DefectTemplateID] = &entry{name: d.DefectTemplate.Name}
					order = append(order, d.DefectTemplateID)
				}
				entries[d.DefectTemplateID].values[d.WallNumber] = d.Value
			}
			rows := make([]wallRow, 0, len(order))
			for _, id := range order {
				e := entries[id]
				rows = append(rows, wallRow{Name: e.name, W1: e.values[1], W2: e.values[2], W3: e.values[3], W4: e.values[4]})
			}
			return rows
		},

		// windowTypeName — отображаемое название типа окна
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

		// wallTypeName — отображаемые названия типов стен (через запятую)
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

		// hasWallType — есть ли конкретный тип в строке wall_type
		"hasWallType": func(wallType, checkType string) bool {
			for _, p := range strings.Split(wallType, ",") {
				if strings.TrimSpace(p) == checkType {
					return true
				}
			}
			return false
		},
	}

	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/auth/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/inspections/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.Static("/static", "./web/static")

	r.GET("/", func(c *gin.Context) {
		if tok, err := c.Cookie("token"); err == nil {
			if _, err := auth.ParseToken(tok); err == nil {
				c.Redirect(http.StatusFound, "/inspections")
				return
			}
		}
		c.Redirect(http.StatusFound, "/login")
	})
	r.GET("/login", handlers.GetLogin)
	r.POST("/login", handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", handlers.PostRegister)
	r.POST("/logout", handlers.PostLogout)
	r.GET("/forgot-password", handlers.GetForgotPassword)
	r.POST("/forgot-password", handlers.PostForgotPassword)
	r.GET("/reset-password", handlers.GetResetPassword)
	r.POST("/reset-password", handlers.PostResetPassword)

	protected := r.Group("/")
	protected.Use(auth.RequireAuth())
	{
		protected.GET("/inspections", handlers.GetInspections)
		protected.GET("/inspections/new", handlers.GetNewInspection)
		protected.POST("/inspections", handlers.PostInspection)
		protected.GET("/inspections/:id", handlers.GetInspection)
		protected.GET("/inspections/:id/edit", handlers.GetEditInspection)
		protected.POST("/inspections/:id/edit", handlers.PostEditInspection)

		protected.POST("/inspections/:id/generate", handlers.PostGenerateDocument)
		protected.GET("/documents/:id/download", handlers.GetDownloadDocument)

		protected.POST("/inspections/:id/upload-plan", handlers.PostUploadPlan)

		protected.GET("/profile", handlers.GetProfile)
		protected.POST("/profile", handlers.PostProfile)

		admin := protected.Group("/admin")
		admin.Use(auth.RequireAdmin())
		{
			admin.GET("/users", handlers.GetAdminUsers)
			admin.POST("/users/:id/role", handlers.PostAdminChangeRole)
			admin.DELETE("/users/:id", handlers.DeleteAdminUser)
		}
	}

	log.Println("Сервер запущен: http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
