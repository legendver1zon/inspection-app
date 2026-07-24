package seed

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"inspection-app/internal/textutil"
	"log"
	"os"
)

// TestUserEmail — постоянный тестовый аккаунт для ручного и автоматизированного тестирования.
// Пароль: Test1234!
const TestUserEmail = "test@example.com"
const TestUserPassword = "Test1234!"

// SeedTestUser создаёт тестового пользователя, если он не существует.
// В production (GIN_MODE=release) тестовый пользователь НЕ создаётся.
func SeedTestUser() {
	if os.Getenv("GIN_MODE") == "release" {
		return
	}

	var existing models.User
	if err := storage.DB.Where("email = ?", TestUserEmail).First(&existing).Error; err == nil {
		// Пользователь уже существует — ничего не делать
		return
	}

	hash, err := auth.HashPassword(TestUserPassword)
	if err != nil {
		log.Printf("SeedTestUser: ошибка хэширования пароля: %v", err)
		return
	}

	fullName := "Тестов Тест Тестович"
	user := models.User{
		Email:        TestUserEmail,
		PasswordHash: hash,
		FullName:     fullName,
		Initials:     textutil.Initials(fullName),
		Role:         models.RoleAdmin,
	}

	if err := storage.DB.Create(&user).Error; err != nil {
		log.Printf("SeedTestUser: ошибка создания пользователя: %v", err)
		return
	}

	log.Printf("SeedTestUser: создан тестовый аккаунт %s (role: admin)", TestUserEmail)
}
