package seed

import (
	"inspection-app/internal/auth"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"log"
	"strings"
)

// TestUserEmail — постоянный тестовый аккаунт для ручного и автоматизированного тестирования.
// Пароль: Test1234!
const TestUserEmail = "test@example.com"
const TestUserPassword = "Test1234!"

// SeedTestUser создаёт тестового пользователя, если он не существует.
// Безопасно вызывать при каждом старте приложения.
func SeedTestUser() {
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
		Initials:     buildInitialsLocal(fullName),
		Role:         models.RoleAdmin,
	}

	if err := storage.DB.Create(&user).Error; err != nil {
		log.Printf("SeedTestUser: ошибка создания пользователя: %v", err)
		return
	}

	log.Printf("SeedTestUser: создан тестовый аккаунт %s (role: admin)", TestUserEmail)
}

// buildInitialsLocal — локальная копия логики из handlers/auth.go
func buildInitialsLocal(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return fullName
	}
	result := parts[0]
	for i := 1; i < len(parts) && i <= 2; i++ {
		runes := []rune(parts[i])
		if len(runes) > 0 {
			result += " " + string(runes[0]) + "."
		}
	}
	return result
}
