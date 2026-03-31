package main

import (
	"context"
	"fmt"
	"html/template"
	"inspection-app/internal/auth"
	"inspection-app/internal/cloudstorage"
	"inspection-app/internal/handlers"
	"inspection-app/internal/logger"
	"inspection-app/internal/models"
	"inspection-app/internal/queue"
	"inspection-app/internal/security"
	"inspection-app/internal/templatefuncs"
	"inspection-app/internal/seed"
	"inspection-app/internal/storage"
	"inspection-app/internal/worker"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func setupLogger() {
	logger.Init()
}

func main() {
	_ = godotenv.Load() // загружает .env если есть (игнорирует ошибку если файл отсутствует)
	setupLogger()
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		security.InitWithRedis(redisURL)
		logger.Info("rate limiter: redis")
	} else {
		security.Init()
		logger.Info("rate limiter: in-memory")
	}
	storage.ConnectFromEnv()
	storage.Migrate()
	seed.SeedDefects()
	seed.SeedTestUser()

	// Инициализация облачного хранилища (Яндекс Диск)
	if token := os.Getenv("YADISK_TOKEN"); token != "" {
		rootDir := os.Getenv("YADISK_ROOT")
		handlers.SetCloudStorage(cloudstorage.NewYandexDisk(token, rootDir))
		logger.Info("cloud storage enabled", "provider", "yandex_disk")
	} else {
		logger.Warn("cloud storage disabled", "reason", "YADISK_TOKEN not set")
	}

	// Инициализация Redis-очереди и фонового воркера загрузки фото
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	q, err := queue.NewFromEnv()
	if err != nil {
		logger.Warn("redis unavailable, sync photo upload", "error", err)
	} else if q != nil {
		handlers.SetUploadQueue(q)
		uploader := worker.New(q)
		uploader.Start(ctx, 5)
		defer uploader.Stop()
		logger.Info("redis connected, worker started", "goroutines", 5)
	} else {
		logger.Warn("redis not configured", "reason", "REDIS_URL not set")
	}

	r := gin.New()
	// В Docker-среде доверяем только localhost; при Nginx reverse proxy — добавить IP прокси
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(logger.PanicRecovery())
	r.Use(logger.RequestLogger())

	// Health-check для мониторинга
	r.GET("/healthz", func(c *gin.Context) {
		result := gin.H{"status": "ok"}
		status := http.StatusOK

		// Проверка БД
		sqlDB, err := storage.DB.DB()
		if err != nil {
			result["db"] = "unavailable"
			result["status"] = "error"
			status = http.StatusServiceUnavailable
		} else if err := sqlDB.Ping(); err != nil {
			result["db"] = "ping failed"
			result["status"] = "error"
			status = http.StatusServiceUnavailable
		} else {
			result["db"] = "ok"
		}

		// Проверка диска (через df, работает в Linux/Docker)
		if out, err := exec.Command("df", "--output=pcent", "/").Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) >= 2 {
				pctStr := strings.TrimSpace(strings.TrimSuffix(lines[len(lines)-1], "%"))
				if pct, err := strconv.Atoi(pctStr); err == nil {
					result["disk_used_pct"] = fmt.Sprintf("%d%%", pct)
					if pct > 90 {
						result["disk"] = "critical"
						result["status"] = "warning"
					} else if pct > 80 {
						result["disk"] = "warning"
					} else {
						result["disk"] = "ok"
					}
				}
			}
		}

		c.JSON(status, result)
	})

	tmpl := template.New("").Funcs(templatefuncs.FuncMap())
	tmpl = template.Must(tmpl.ParseGlob("web/templates/partials/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/auth/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/inspections/*.html"))
	tmpl = template.Must(tmpl.ParseGlob("web/templates/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	r.Static("/static", "./web/static")

	r.GET("/", func(c *gin.Context) {
		if tok, err := c.Cookie("token"); err == nil {
			if claims, err := auth.ParseToken(tok); err == nil {
				var u models.User
				if storage.DB.First(&u, claims.UserID).Error == nil {
					c.Redirect(http.StatusFound, "/inspections")
					return
				}
			}
		}
		c.Redirect(http.StatusFound, "/login")
	})
	r.GET("/login", handlers.GetLogin)
	r.POST("/login", security.RateLimitLogin(), handlers.PostLogin)
	r.GET("/register", handlers.GetRegister)
	r.POST("/register", security.RateLimitRegister(), handlers.PostRegister)
	r.POST("/logout", handlers.PostLogout)
	r.GET("/forgot-password", handlers.GetForgotPassword)
	r.POST("/forgot-password", security.RateLimitForgotPassword(), handlers.PostForgotPassword)
	r.GET("/reset-password", handlers.GetResetPassword)
	r.POST("/reset-password", handlers.PostResetPassword)

	protected := r.Group("/")
	protected.Use(auth.RequireAuth())
	protected.Use(func(c *gin.Context) {
		userID := c.GetUint("userID")
		var u models.User
		if storage.DB.First(&u, userID).Error != nil {
			auth.ClearAuthCookie(c)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	})
	{
		protected.GET("/dashboard", handlers.GetDashboard)
		protected.GET("/inspections", handlers.GetInspections)
		protected.GET("/inspections/new", handlers.GetNewInspection)
		protected.GET("/inspections/:id", handlers.GetInspection)
		protected.GET("/inspections/:id/edit", handlers.GetEditInspection)
		protected.POST("/inspections/:id/edit", handlers.PostEditInspection)
		protected.POST("/inspections/:id/status", handlers.PostUpdateStatus)
		protected.GET("/inspections/:id/upload-status", handlers.GetUploadStatus)

		protected.POST("/inspections/:id/generate", handlers.PostGenerateDocument)
		protected.GET("/documents/:id/download", handlers.GetDownloadDocument)

		protected.POST("/inspections/:id/upload-plan", handlers.PostUploadPlan)

		protected.GET("/profile", handlers.GetProfile)
		protected.POST("/profile", handlers.PostProfile)
		protected.POST("/profile/avatar", handlers.PostUploadAvatar)

		protected.POST("/documents/:id/delete", handlers.PostDeleteDocument)
		protected.POST("/defects/:id/photos", handlers.PostUploadPhoto)
		protected.POST("/photos/:id/delete", handlers.DeletePhoto)

		admin := protected.Group("/admin")
		admin.Use(auth.RequireAdmin())
		{
			admin.GET("/users", handlers.GetAdminUsers)
			admin.GET("/users/:id/edit", handlers.GetAdminEditUser)
			admin.POST("/users/:id/edit", handlers.PostAdminEditUser)
			admin.POST("/users/:id/role", handlers.PostAdminChangeRole)
			admin.POST("/users/:id/delete", handlers.DeleteAdminUser)
			admin.DELETE("/users/:id", handlers.DeleteAdminUser)
			admin.POST("/inspections/:id/delete", handlers.PostDeleteInspection)
		}
	}

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	go func() {
		logger.Info("server started", "addr", ":8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server start failed: %v", err)
		}
	}()

	// Ждём сигнала остановки
	<-ctx.Done()
	logger.Info("server shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
