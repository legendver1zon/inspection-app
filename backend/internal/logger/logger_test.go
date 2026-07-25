package logger

import (
	"context"
	"os"
	"testing"
)

func TestInit_DevMode(t *testing.T) {
	os.Unsetenv("GIN_MODE")
	os.Unsetenv("LOG_LEVEL")
	Init()

	if L == nil {
		t.Fatal("logger not initialized")
	}
	// Should not panic
	Info("test info", "key", "value")
	Warn("test warn")
	Error("test error", "err", "something")
	Debug("test debug")
}

func TestInit_ProductionJSON(t *testing.T) {
	os.Setenv("GIN_MODE", "release")
	defer os.Unsetenv("GIN_MODE")
	Init()

	if L == nil {
		t.Fatal("logger not initialized in production mode")
	}
	Info("json test", "mode", "release")
}

func TestInit_LogLevel(t *testing.T) {
	tests := []string{"debug", "info", "warn", "error", ""}
	for _, level := range tests {
		os.Setenv("LOG_LEVEL", level)
		Init()
		if L == nil {
			t.Fatalf("logger nil for LOG_LEVEL=%q", level)
		}
	}
	os.Unsetenv("LOG_LEVEL")
}

func TestCtx_WithRequestID(t *testing.T) {
	Init()
	ctx := WithRequestID(context.Background(), "abc123")
	l := Ctx(ctx)
	if l == nil {
		t.Fatal("Ctx returned nil")
	}
	// Should not panic
	l.Info("test with request_id")
}

func TestCtx_WithUserID(t *testing.T) {
	Init()
	ctx := WithUserID(context.Background(), 42)
	l := Ctx(ctx)
	if l == nil {
		t.Fatal("Ctx returned nil")
	}
	l.Info("test with user_id")
}

func TestCtx_WithEndpoint(t *testing.T) {
	Init()
	ctx := WithEndpoint(context.Background(), "GET /inspections")
	l := Ctx(ctx)
	if l == nil {
		t.Fatal("Ctx returned nil")
	}
	l.Info("test with endpoint")
}

func TestCtx_FullContext(t *testing.T) {
	Init()
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-001")
	ctx = WithUserID(ctx, 7)
	ctx = WithEndpoint(ctx, "POST /login")

	l := Ctx(ctx)
	if l == nil {
		t.Fatal("Ctx returned nil")
	}
	l.Info("full context test")
}

func TestCtx_NilContext(t *testing.T) {
	Init()
	l := Ctx(nil)
	if l == nil {
		t.Fatal("Ctx(nil) should return global logger, not nil")
	}
	l.Info("nil context test")
}

func TestCtx_EmptyContext(t *testing.T) {
	Init()
	l := Ctx(context.Background())
	if l == nil {
		t.Fatal("Ctx with empty context should return global logger")
	}
	l.Info("empty context test")
}
