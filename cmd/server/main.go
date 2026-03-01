package main

import (
	"fmt"
	"html/template"
	"inspection-app/internal/auth"
	"inspection-app/internal/handlers"
	"inspection-app/internal/seed"
	"inspection-app/internal/storage"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// Подключение к БД
	storage.Connect("inspection.db")
	storage.Migrate()
	seed.SeedDefects()

	r := gin.Default()

	// Загрузка HTML-шаблонов с кастомными функциями
	funcMap := template.FuncMap{
		"string": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
	}
	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/auth/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/inspections/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	// Статические файлы
	r.Static("/static", "./web/static")

	// Маршруты без авторизации
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/inspections")
	})
	r.GET("/login", handlers.GetLogin)
	r.POST("/login", handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", handlers.PostRegister)
	r.POST("/logout", handlers.PostLogout)

	// Маршруты с авторизацией
	protected := r.Group("/")
	protected.Use(auth.RequireAuth())
	{
		// Осмотры
		protected.GET("/inspections", handlers.GetInspections)
		protected.GET("/inspections/new", handlers.GetNewInspection)
		protected.POST("/inspections", handlers.PostInspection)
		protected.GET("/inspections/:id", handlers.GetInspection)
		protected.GET("/inspections/:id/edit", handlers.GetEditInspection)
		protected.POST("/inspections/:id/edit", handlers.PostEditInspection)

		// Генерация документов
		protected.POST("/inspections/:id/generate", handlers.PostGenerateDocument)
		protected.GET("/documents/:id/download", handlers.GetDownloadDocument)

		// Загрузка фото плана
		protected.POST("/inspections/:id/upload-plan", handlers.PostUploadPlan)

		// Профиль
		protected.GET("/profile", handlers.GetProfile)
		protected.POST("/profile", handlers.PostProfile)

		// Админ-панель
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
