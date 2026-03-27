package security

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MemoryRateLimiter — потокобезопасный rate limiter на основе скользящего окна.
// Хранит счётчики неудачных попыток в памяти.
// Для продакшна с несколькими инстансами заменить на Redis-backed реализацию (Section 14).
type MemoryRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*limitWindow
	max     int
	window  time.Duration
}

type limitWindow struct {
	count     int
	expiresAt time.Time
}

// NewMemoryRateLimiter создаёт лимитер с заданными параметрами.
// max — максимальное число попыток в окне.
// window — продолжительность окна.
func NewMemoryRateLimiter(max int, window time.Duration) *MemoryRateLimiter {
	rl := &MemoryRateLimiter{
		windows: make(map[string]*limitWindow),
		max:     max,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

// Check проверяет, не превышен ли лимит для ключа. НЕ изменяет счётчик.
func (rl *MemoryRateLimiter) Check(key string) (allowed bool, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	w, ok := rl.windows[key]
	if !ok || time.Now().After(w.expiresAt) {
		return true, 0
	}
	if w.count >= rl.max {
		return false, time.Until(w.expiresAt)
	}
	return true, 0
}

// Increment фиксирует неудачную попытку для ключа.
func (rl *MemoryRateLimiter) Increment(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	w, ok := rl.windows[key]
	if !ok || now.After(w.expiresAt) {
		rl.windows[key] = &limitWindow{count: 1, expiresAt: now.Add(rl.window)}
		return
	}
	w.count++
}

// CheckAndIncrement атомарно проверяет и увеличивает счётчик.
// Используется для ограничения ресурсоёмких операций (создание актов).
func (rl *MemoryRateLimiter) CheckAndIncrement(key string) (allowed bool, retryAfter time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	w, ok := rl.windows[key]
	if !ok || now.After(w.expiresAt) {
		rl.windows[key] = &limitWindow{count: 1, expiresAt: now.Add(rl.window)}
		return true, 0
	}
	if w.count >= rl.max {
		return false, time.Until(w.expiresAt)
	}
	w.count++
	return true, 0
}

// Reset сбрасывает счётчик для ключа (вызывается при успешном входе).
func (rl *MemoryRateLimiter) Reset(key string) {
	rl.mu.Lock()
	delete(rl.windows, key)
	rl.mu.Unlock()
}

// cleanup периодически удаляет истёкшие окна, чтобы не росла память.
func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rl.mu.Lock()
		for k, w := range rl.windows {
			if now.After(w.expiresAt) {
				delete(rl.windows, k)
			}
		}
		rl.mu.Unlock()
	}
}

// --- Глобальные лимитеры (инициализируются через Init() в main.go) ---

var (
	LoginLimiter          *MemoryRateLimiter
	RegisterLimiter       *MemoryRateLimiter
	ForgotPasswordLimiter *MemoryRateLimiter
	InspectionLimiter     *MemoryRateLimiter
)

// Init инициализирует все rate limiter с настройками по умолчанию.
func Init() {
	LoginLimiter          = NewMemoryRateLimiter(5, 15*time.Minute) // 5 неудачных попыток / 15 мин с IP
	RegisterLimiter       = NewMemoryRateLimiter(3, time.Hour)       // 3 регистрации / час с IP
	ForgotPasswordLimiter = NewMemoryRateLimiter(3, time.Hour)       // 3 сброса пароля / час с IP
	InspectionLimiter     = NewMemoryRateLimiter(20, time.Hour)      // 20 новых актов / час на пользователя
}

// --- Тексты сообщений об ограничениях ---

func loginBlockedMsg(retryAfter time.Duration) string {
	mins := int(retryAfter.Minutes()) + 1
	return fmt.Sprintf(
		"Слишком много неудачных попыток входа — ваш IP временно заблокирован для защиты аккаунтов.\n"+
			"Что произошло: вы (или кто-то с вашего IP) ввели неверный пароль 5 раз подряд.\n"+
			"Что делать: убедитесь, что вводите верный email и пароль, и попробуйте снова через %d мин.",
		mins,
	)
}

func registerBlockedMsg(retryAfter time.Duration) string {
	mins := int(retryAfter.Minutes()) + 1
	return fmt.Sprintf(
		"Слишком много попыток регистрации с вашего IP-адреса.\n"+
			"Что произошло: с вашего IP зарегистрировано несколько аккаунтов за последний час.\n"+
			"Что делать: подождите %d мин и попробуйте снова.",
		mins,
	)
}

func forgotPasswordBlockedMsg(retryAfter time.Duration) string {
	mins := int(retryAfter.Minutes()) + 1
	return fmt.Sprintf(
		"Слишком много запросов на сброс пароля с вашего IP-адреса.\n"+
			"Что произошло: защита от перебора — 3 запроса в час.\n"+
			"Что делать: подождите %d мин или обратитесь к администратору.",
		mins,
	)
}

func InspectionBlockedMsg(retryAfter time.Duration) string {
	mins := int(retryAfter.Minutes()) + 1
	return fmt.Sprintf(
		"Вы создаёте акты слишком часто. Лимит: 20 актов в час.\n"+
			"Что делать: подождите %d мин или обратитесь к администратору.",
		mins,
	)
}

// --- Gin middleware ---

// CheckInspectionLimit атомарно проверяет и инкрементирует лимит создания актов для пользователя.
// Администраторы лимит не имеют.
// Возвращает (true, "") если разрешено, (false, сообщение) если лимит превышен.
func CheckInspectionLimit(userID uint, role string) (bool, string) {
	if role == "admin" {
		return true, ""
	}
	key := fmt.Sprintf("insp:%d", userID)
	allowed, retryAfter := InspectionLimiter.CheckAndIncrement(key)
	if !allowed {
		return false, InspectionBlockedMsg(retryAfter)
	}
	return true, ""
}

// RateLimitLogin — middleware для POST /login.
// Проверяет лимит ПЕРЕД попыткой входа. Сам счётчик инкрементируется в PostLogin при неудаче.
func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := LoginLimiter.Check(c.ClientIP())
		if !allowed {
			Log(EventLoginBlocked, c.ClientIP(), "")
			c.HTML(http.StatusTooManyRequests, "login.html", gin.H{
				"title": "Вход",
				"error": loginBlockedMsg(retryAfter),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitRegister — middleware для POST /register.
func RateLimitRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := RegisterLimiter.Check(c.ClientIP())
		if !allowed {
			Log(EventRegisterBlocked, c.ClientIP(), "")
			c.HTML(http.StatusTooManyRequests, "register.html", gin.H{
				"title": "Регистрация",
				"error": registerBlockedMsg(retryAfter),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitForgotPassword — middleware для POST /forgot-password.
func RateLimitForgotPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retryAfter := ForgotPasswordLimiter.Check(c.ClientIP())
		if !allowed {
			Log(EventForgotPasswordBlocked, c.ClientIP(), "")
			c.HTML(http.StatusTooManyRequests, "forgot_password.html", gin.H{
				"title": "Восстановление пароля",
				"error": forgotPasswordBlockedMsg(retryAfter),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
