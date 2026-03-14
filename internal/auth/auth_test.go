package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// --- HashPassword / CheckPassword ---

func TestHashPassword_ReturnsNonEmptyHash(t *testing.T) {
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "mypassword" {
		t.Fatal("hash must differ from original password")
	}
}

func TestHashPassword_DifferentCallsDifferentHashes(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("bcrypt should produce different salts each call")
	}
}

func TestCheckPassword_Correct(t *testing.T) {
	hash, _ := HashPassword("secret123")
	if !CheckPassword("secret123", hash) {
		t.Fatal("correct password should match hash")
	}
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, _ := HashPassword("secret123")
	if CheckPassword("wrongpassword", hash) {
		t.Fatal("wrong password should not match hash")
	}
}

func TestCheckPassword_Empty(t *testing.T) {
	hash, _ := HashPassword("secret123")
	if CheckPassword("", hash) {
		t.Fatal("empty password should not match hash")
	}
}

// --- GenerateToken / ParseToken ---

func TestGenerateToken_ReturnsNonEmpty(t *testing.T) {
	tok, err := GenerateToken(42, "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tok == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestParseToken_Valid(t *testing.T) {
	tok, _ := GenerateToken(99, "inspector")
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 99 {
		t.Errorf("UserID: got %d, want 99", claims.UserID)
	}
	if claims.Role != "inspector" {
		t.Errorf("Role: got %q, want %q", claims.Role, "inspector")
	}
}

func TestParseToken_AdminRole(t *testing.T) {
	tok, _ := GenerateToken(1, "admin")
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q, want %q", claims.Role, "admin")
	}
}

func TestParseToken_Invalid(t *testing.T) {
	_, err := ParseToken("not.a.valid.token")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}

func TestParseToken_Empty(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestParseToken_Tampered(t *testing.T) {
	tok, _ := GenerateToken(1, "admin")
	_, err := ParseToken(tok + "tampered")
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestParseToken_Expired(t *testing.T) {
	claims := Claims{
		UserID: 1,
		Role:   "inspector",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := tok.SignedString(jwtSecret)

	_, err := ParseToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParseToken_WrongSigningMethod(t *testing.T) {
	// Попытка создать токен с другим алгоритмом
	claims := Claims{
		UserID: 1,
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := ParseToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}
