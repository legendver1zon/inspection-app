package auth

import (
	"os"
	"testing"
)

func TestGetSecret_FromEnv(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-custom-secret-key-12345")
	defer os.Unsetenv("JWT_SECRET")

	s := getSecret()
	if s != "my-custom-secret-key-12345" {
		t.Errorf("getSecret() = %q, want 'my-custom-secret-key-12345'", s)
	}
}

func TestGetSecret_DevFallback(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("GIN_MODE")

	s := getSecret()
	if s != "dev-only-secret-do-not-use-in-production" {
		t.Errorf("getSecret() = %q, want dev fallback", s)
	}
}

func TestGetSecret_TokenRoundtrip(t *testing.T) {
	// Проверяем что токен, созданный с текущим секретом, валидируется
	token, err := GenerateToken(42, "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("UserID = %d, want 42", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, want 'admin'", claims.Role)
	}
}
