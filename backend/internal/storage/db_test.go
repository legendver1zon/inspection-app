package storage

import "testing"

func TestMaskDSN_HidesPassword(t *testing.T) {
	dsn := "postgres://inspection:secret@localhost:5432/inspection_db?sslmode=disable"
	masked := maskDSN(dsn)

	if masked == dsn {
		t.Error("maskDSN не должен возвращать исходный DSN с паролем")
	}
	if contains(masked, "secret") {
		t.Errorf("maskDSN всё ещё содержит пароль: %s", masked)
	}
	if !contains(masked, "inspection") {
		t.Errorf("maskDSN должен сохранять username: %s", masked)
	}
	// url.UserPassword URL-кодирует *** в %2A%2A%2A — это нормально
	if !contains(masked, "%2A%2A%2A") && !contains(masked, "***") {
		t.Errorf("maskDSN должен содержать замаскированный пароль: %s", masked)
	}
	if !contains(masked, "localhost:5432") {
		t.Errorf("maskDSN должен сохранять хост: %s", masked)
	}
}

func TestMaskDSN_InvalidURL(t *testing.T) {
	masked := maskDSN("not-a-url-:::::@@@")
	if masked == "" {
		t.Error("maskDSN не должен возвращать пустую строку")
	}
}

func TestMaskDSN_NoPassword(t *testing.T) {
	dsn := "postgres://inspection@localhost:5432/inspection_db"
	masked := maskDSN(dsn)
	if contains(masked, "secret") {
		t.Errorf("maskDSN не должен добавлять пароль: %s", masked)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchIn(s, substr)
}

func searchIn(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
