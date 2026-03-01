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

	"github.com/gin-gonic/gin"
)

func main() {
	storage.Connect("inspection.db")
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
	}

	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/auth/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/inspections/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.Static("/static", "./web/static")

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/inspections")
	})
	r.GET("/login", handlers.GetLogin)
	r.POST("/login", handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", handlers.PostRegister)
	r.POST("/logout", handlers.PostLogout)

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
