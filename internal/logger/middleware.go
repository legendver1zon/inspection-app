package logger

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger — Gin middleware that:
// 1. Generates a unique request_id
// 2. Enriches context with request_id, user_id, endpoint
// 3. Logs every request with method, path, status, latency
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Generate short request ID (8 hex chars = 4 bytes)
		reqID := generateRequestID()
		ctx := WithRequestID(c.Request.Context(), reqID)
		ctx = WithEndpoint(ctx, fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()))
		c.Request = c.Request.WithContext(ctx)

		// Set header for tracing
		c.Header("X-Request-ID", reqID)

		c.Next()

		// After request: enrich with user_id if set by auth middleware
		userID := c.GetUint("userID")
		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []any{
			"request_id", reqID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"ip", c.ClientIP(),
		}
		if userID > 0 {
			attrs = append(attrs, "user_id", userID)
		}

		switch {
		case status >= 500:
			L.Error("request", attrs...)
		case status >= 400:
			L.Warn("request", attrs...)
		default:
			L.Info("request", attrs...)
		}
	}
}

// PanicRecovery — Gin middleware that catches panics,
// logs the stack trace, and returns 500 without crashing the server.
func PanicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				reqID, _ := c.Request.Context().Value(keyRequestID).(string)
				L.Error("panic recovered",
					"request_id", reqID,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"error", fmt.Sprintf("%v", r),
					"ip", c.ClientIP(),
				)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
