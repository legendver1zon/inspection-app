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
	"inspection-app/internal/seed"
	"inspection-app/internal/storage"
	"inspection-app/internal/templatefuncs"
	"inspection-app/internal/worker"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
		uploader := worker.New(q, handlers.UploadInspectionPhotos)
		uploader.Start(ctx, 5)
		defer uploader.Stop()
		logger.Info("redis connected, worker started", "goroutines", 5)
	} else {
		logger.Warn("redis not configured", "reason", "REDIS_URL not set")
	}

	// Self-heal loop: retry failed + восстановление зависших uploading (работает без Redis)
	waitSelfHeal := handlers.StartSelfHealLoop(ctx)
	defer waitSelfHeal()

	r := gin.New()
	// В Docker-среде доверяем только localhost; при Nginx reverse proxy — добавить IP прокси
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(logger.PanicRecovery())
	r.Use(logger.RequestLogger())
	// Security headers
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// unsafe-inline: в шаблонах остаются inline-скрипты и onclick-обработчики;
		// cdnjs — Cropper.js (edit.html), qrserver — QR-код папки фото (view.html).
		// При выносе inline-кода и вендоринге зависимостей можно ужесточить до 'self'
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; "+
				"style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; "+
				"img-src 'self' data: blob: https://api.qrserver.com; "+
				"connect-src 'self'; worker-src 'self'; manifest-src 'self'; "+
				"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		c.Next()
	})

	// Ограничение размера тела запроса (защита от DoS): загрузка фото — до 200 МБ
	// (множественный выбор файлов), остальные формы — до 10 МБ.
	r.Use(func(c *gin.Context) {
		limit := int64(10 << 20)
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/defects/") || strings.HasSuffix(p, "/upload-plan") || p == "/profile/avatar" ||
			(strings.HasPrefix(p, "/inspections/") && strings.HasSuffix(p, "/photos")) {
			limit = 200 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	})

	// Кеш для проверки диска (обновляется не чаще раза в 30 секунд)
	var diskCache struct {
		sync.Mutex
		pct     int
		status  string
		updated time.Time
	}
	getDiskUsage := func() (int, string) {
		diskCache.Lock()
		defer diskCache.Unlock()
		if time.Since(diskCache.updated) < 30*time.Second {
			return diskCache.pct, diskCache.status
		}
		out, err := exec.Command("df", "--output=pcent", "/").Output()
		if err != nil {
			return -1, ""
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) < 2 {
			return -1, ""
		}
		pctStr := strings.TrimSpace(strings.TrimSuffix(lines[len(lines)-1], "%"))
		pct, err := strconv.Atoi(pctStr)
		if err != nil {
			return -1, ""
		}
		s := "ok"
		if pct > 90 {
			s = "critical"
		} else if pct > 80 {
			s = "warning"
		}
		diskCache.pct = pct
		diskCache.status = s
		diskCache.updated = time.Now()
		return pct, s
	}

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

		// Проверка диска (кешировано, обновляется раз в 30 сек)
		if pct, diskStatus := getDiskUsage(); pct >= 0 {
			result["disk_used_pct"] = fmt.Sprintf("%d%%", pct)
			result["disk"] = diskStatus
			if diskStatus == "critical" {
				result["status"] = "warning"
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

	// React SPA: если рядом лежит собранный фронтенд (web/spa или SPA_DIR),
	// страницы отдаёт он; экшены, скачивания и API остаются на своих маршрутах.
	// Без сборки (например, в dev с Vite) работают старые HTML-шаблоны.
	spaDir := os.Getenv("SPA_DIR")
	if spaDir == "" {
		spaDir = "./web/spa"
	}
	spaIndex := filepath.Join(spaDir, "index.html")
	_, spaStatErr := os.Stat(spaIndex)
	spaEnabled := spaStatErr == nil
	serveSPA := func(c *gin.Context) { c.File(spaIndex) }
	if spaEnabled {
		r.Static("/assets", filepath.Join(spaDir, "assets"))
		logger.Info("SPA frontend enabled", "dir", spaDir)
	} else {
		logger.Info("SPA frontend not found, serving HTML templates", "checked", spaIndex)
	}

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
	if spaEnabled {
		r.GET("/login", serveSPA)
	} else {
		r.GET("/login", handlers.GetLogin)
	}
	r.POST("/login", security.RateLimitLogin(), handlers.PostLogin)
	if spaEnabled {
		r.GET("/register", serveSPA)
		r.GET("/forgot-password", serveSPA)
		r.GET("/reset-password", serveSPA)
	} else {
		r.GET("/register", handlers.GetRegister)
		r.GET("/forgot-password", handlers.GetForgotPassword)
		r.GET("/reset-password", handlers.GetResetPassword)
	}
	r.POST("/register", security.RateLimitRegister(), handlers.PostRegister)
	r.POST("/logout", handlers.PostLogout)
	r.POST("/forgot-password", security.RateLimitForgotPassword(), handlers.PostForgotPassword)
	r.POST("/reset-password", security.RateLimitResetPassword(), handlers.PostResetPassword)

	// JSON-API для React-фронтенда
	api := r.Group("/api")
	{
		api.POST("/login", handlers.APILogin)
		api.POST("/logout", handlers.APILogout)
		api.POST("/register", security.RateLimitRegisterJSON(), handlers.APIRegister)
		api.POST("/forgot-password", security.RateLimitForgotPasswordJSON(), handlers.APIForgotPassword)
		api.POST("/reset-password", security.RateLimitResetPasswordJSON(), handlers.APIResetPassword)
		apiAuthed := api.Group("/")
		apiAuthed.Use(handlers.APIAuth())
		{
			apiAuthed.GET("/me", handlers.APIMe)
			apiAuthed.GET("/inspections", handlers.APIListInspections)
			apiAuthed.POST("/inspections", handlers.APICreateInspection)
			apiAuthed.GET("/inspections/:id", handlers.APIGetInspection)
			apiAuthed.GET("/inspections/:id/edit-data", handlers.APIGetEditData)
			apiAuthed.GET("/dashboard", handlers.APIDashboard)
			apiAuthed.POST("/profile", handlers.APIUpdateProfile)

			apiAdmin := apiAuthed.Group("/")
			apiAdmin.Use(handlers.APIAdminOnly())
			{
				apiAdmin.GET("/users", handlers.APIListUsers)
				apiAdmin.POST("/users/:id", handlers.APIUpdateUser)
				apiAdmin.POST("/users/:id/delete", handlers.APIDeleteUser)
			}
		}
	}

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
		// Пользователь уже загружен — хэндлеры берут его из контекста (handlers.CurrentUser)
		c.Set("currentUser", u)
		c.Next()
	})
	{
		// Страницы: в SPA-режиме их рендерит React (auth проверяет API),
		// иначе — старые шаблоны за HTML-авторизацией
		if spaEnabled {
			r.GET("/dashboard", serveSPA)
			r.GET("/inspections", serveSPA)
			r.GET("/inspections/:id", serveSPA)
			r.GET("/inspections/:id/edit", serveSPA)
			r.GET("/profile", serveSPA)
			r.GET("/admin/users", serveSPA)
		} else {
			protected.GET("/dashboard", handlers.GetDashboard)
			protected.GET("/inspections", handlers.GetInspections)
			protected.GET("/inspections/:id", handlers.GetInspection)
			protected.GET("/inspections/:id/edit", handlers.GetEditInspection)
			protected.GET("/profile", handlers.GetProfile)
		}
		protected.GET("/inspections/new", handlers.GetNewInspection)
		protected.POST("/inspections/:id/edit", handlers.PostEditInspection)
		protected.GET("/api/inspections/:id/check-act-number", handlers.GetCheckActNumber)
		protected.POST("/inspections/:id/status", handlers.PostUpdateStatus)
		protected.POST("/inspections/:id/delete", handlers.PostDeleteInspection)
		protected.GET("/inspections/:id/upload-status", handlers.GetUploadStatus)
		protected.GET("/inspections/:id/ws", handlers.WsUploadStatus)

		protected.POST("/inspections/:id/generate", handlers.PostGenerateDocument)
		protected.GET("/documents/:id/download", handlers.GetDownloadDocument)

		protected.POST("/inspections/:id/upload-plan", handlers.PostUploadPlan)
		protected.POST("/inspections/:id/photos", handlers.PostUploadInspectionPhoto)

		protected.POST("/profile", handlers.PostProfile)
		protected.POST("/profile/avatar", handlers.PostUploadAvatar)

		protected.POST("/documents/:id/delete", handlers.PostDeleteDocument)
		protected.POST("/defects/:id/photos", handlers.PostUploadPhoto)
		protected.POST("/photos/:id/delete", handlers.DeletePhoto)
		protected.GET("/photos/:id/download", handlers.GetPhotoDownload)
		protected.GET("/photos/:id/thumb", handlers.GetPhotoThumb)

		admin := protected.Group("/admin")
		admin.Use(security.RateLimitAdmin())
		admin.Use(auth.RequireAdmin())
		{
			if !spaEnabled {
				admin.GET("/users", handlers.GetAdminUsers)
				admin.GET("/users/:id/edit", handlers.GetAdminEditUser)
			}
			admin.POST("/users/:id/edit", handlers.PostAdminEditUser)
			admin.POST("/users/:id/role", handlers.PostAdminChangeRole)
			admin.POST("/users/:id/delete", handlers.DeleteAdminUser)
			admin.DELETE("/users/:id", handlers.DeleteAdminUser)
			admin.POST("/inspections/:id/delete", handlers.PostDeleteInspection)
		}
	}

	// SPA-fallback: неизвестные GET-адреса — клиентские маршруты React.
	// Файлы из корня сборки (sw.js, registerSW.js, manifest.webmanifest, иконки)
	// отдаются как файлы, иначе service worker получил бы index.html.
	if spaEnabled {
		_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
		spaFile := func(clean string) (string, bool) {
			if clean == "/" || strings.Contains(clean, "..") {
				return "", false
			}
			fp := filepath.Join(spaDir, filepath.FromSlash(clean))
			st, err := os.Stat(fp)
			if err != nil || !st.Mode().IsRegular() {
				return "", false
			}
			return fp, true
		}
		r.NoRoute(func(c *gin.Context) {
			p := c.Request.URL.Path
			if c.Request.Method != http.MethodGet ||
				strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/static") || strings.HasPrefix(p, "/assets") {
				c.JSON(http.StatusNotFound, gin.H{"error": "Не найдено"})
				return
			}
			clean := path.Clean("/" + p)
			if fp, ok := spaFile(clean); ok {
				if clean == "/sw.js" || clean == "/registerSW.js" {
					c.Header("Cache-Control", "no-cache")
				}
				if clean == "/sw.js" {
					c.Header("Service-Worker-Allowed", "/")
				}
				c.File(fp)
				return
			}
			serveSPA(c)
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	go func() {
		logger.Info("server started", "addr", ":"+port)
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
