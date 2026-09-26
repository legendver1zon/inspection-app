package auth

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(getSecret())

func getSecret() string {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		if os.Getenv("GIN_MODE") == "release" {
			log.Fatal("JWT_SECRET обязателен в production (GIN_MODE=release)")
		}
		return "dev-only-secret-do-not-use-in-production"
	}
	return s
}

type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// HashPassword — хэширует пароль через bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword — проверяет пароль против хэша
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Сессия живёт 7 дней и продлевается при активности: истечение ровно
// через сутки после входа посреди осмотра выглядело как «фото не загружаются».
const (
	SessionTTL = 7 * 24 * time.Hour
	RenewAfter = 24 * time.Hour
)

func GenerateToken(userID uint, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(SessionTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken — парсит и валидирует JWT токен
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("неверный метод подписи")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("невалидный токен")
	}
	return claims, nil
}

// ShouldRenew — токен старше суток пора перевыпустить, чтобы срок сессии
// отсчитывался от последней активности, а не от входа.
func ShouldRenew(claims *Claims) bool {
	if claims == nil || claims.IssuedAt == nil {
		return true
	}
	return time.Since(claims.IssuedAt.Time) > RenewAfter
}
