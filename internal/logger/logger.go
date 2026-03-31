package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Context keys for request-scoped data
type ctxKey string

const (
	keyRequestID ctxKey = "request_id"
	keyUserID    ctxKey = "user_id"
	keyEndpoint  ctxKey = "endpoint"
)

var L *slog.Logger

// Init initializes the global logger.
// In production (GIN_MODE=release) — JSON output.
// In dev — human-readable text output.
// LOG_LEVEL env: debug, info, warn, error (default: info).
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Shorter timestamp format
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.DateTime))
			}
			return a
		},
	}

	var handler slog.Handler
	if os.Getenv("GIN_MODE") == "release" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	L = slog.New(handler)
	slog.SetDefault(L)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// --- Context helpers ---

// WithRequestID adds request_id to context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// WithUserID adds user_id to context.
func WithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// WithEndpoint adds endpoint to context.
func WithEndpoint(ctx context.Context, endpoint string) context.Context {
	return context.WithValue(ctx, keyEndpoint, endpoint)
}

// Ctx returns a logger enriched with request context (request_id, user_id, endpoint).
func Ctx(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return L
	}
	attrs := []any{}
	if v, ok := ctx.Value(keyRequestID).(string); ok {
		attrs = append(attrs, "request_id", v)
	}
	if v, ok := ctx.Value(keyUserID).(uint); ok && v > 0 {
		attrs = append(attrs, "user_id", v)
	}
	if v, ok := ctx.Value(keyEndpoint).(string); ok {
		attrs = append(attrs, "endpoint", v)
	}
	if len(attrs) == 0 {
		return L
	}
	return L.With(attrs...)
}

// --- Convenience functions (use global logger) ---

func Info(msg string, args ...any)  { L.Info(msg, args...) }
func Warn(msg string, args ...any)  { L.Warn(msg, args...) }
func Error(msg string, args ...any) { L.Error(msg, args...) }
func Debug(msg string, args ...any) { L.Debug(msg, args...) }
