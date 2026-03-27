package security

import (
	"strings"
	"testing"
	"time"
)

// --- ValidatePassword ---

func TestValidatePassword_Valid(t *testing.T) {
	cases := []string{
		"Secret1!",
		"Aa1@abcd",
		"Hello1#World",
		"Пароль1!",
	}
	for _, p := range cases {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("пароль %q должен быть валидным, получили ошибку: %v", p, err)
		}
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := ValidatePassword("Ab1!")
	if err == nil {
		t.Error("ожидали ошибку для пароля < 6 символов")
	}
	if err != nil && !strings.Contains(err.Error(), "6") {
		t.Errorf("ожидали упоминание '6' в ошибке, получили: %v", err)
	}
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	err := ValidatePassword("secret1!")
	if err == nil {
		t.Error("ожидали ошибку: нет заглавной буквы")
	}
	if err != nil && !strings.Contains(err.Error(), "заглавн") {
		t.Errorf("ожидали упоминание заглавной буквы, получили: %v", err)
	}
}

func TestValidatePassword_NoDigit(t *testing.T) {
	err := ValidatePassword("Secret!")
	if err == nil {
		t.Error("ожидали ошибку: нет цифры")
	}
	if err != nil && !strings.Contains(err.Error(), "цифр") {
		t.Errorf("ожидали упоминание цифры, получили: %v", err)
	}
}

func TestValidatePassword_NoSpecial(t *testing.T) {
	err := ValidatePassword("Secret1")
	if err == nil {
		t.Error("ожидали ошибку: нет спецсимвола")
	}
	if err != nil && !strings.Contains(err.Error(), "спецсимвол") {
		t.Errorf("ожидали упоминание спецсимвола, получили: %v", err)
	}
}

// --- MemoryRateLimiter ---

func newTestLimiter(max int) *MemoryRateLimiter {
	return NewMemoryRateLimiter(max, time.Hour)
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := newTestLimiter(3)
	for i := 0; i < 3; i++ {
		allowed, _ := rl.Check("ip1")
		if !allowed {
			t.Errorf("попытка %d должна быть разрешена", i+1)
		}
		rl.Increment("ip1")
	}
}

func TestRateLimiter_BlocksWhenLimitReached(t *testing.T) {
	rl := newTestLimiter(3)
	for i := 0; i < 3; i++ {
		rl.Increment("ip1")
	}
	allowed, retryAfter := rl.Check("ip1")
	if allowed {
		t.Error("ожидали блокировку после 3 попыток")
	}
	if retryAfter <= 0 {
		t.Error("retryAfter должен быть > 0 при блокировке")
	}
}

func TestRateLimiter_ResetClearsCounter(t *testing.T) {
	rl := newTestLimiter(3)
	for i := 0; i < 3; i++ {
		rl.Increment("ip1")
	}
	rl.Reset("ip1")
	allowed, _ := rl.Check("ip1")
	if !allowed {
		t.Error("после Reset() попытки должны быть разрешены")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewMemoryRateLimiter(2, 50*time.Millisecond) // очень короткое окно для теста
	rl.Increment("ip2")
	rl.Increment("ip2")
	allowed, _ := rl.Check("ip2")
	if allowed {
		t.Fatal("должно быть заблокировано до истечения окна")
	}
	time.Sleep(60 * time.Millisecond) // ждём истечения окна
	allowed, _ = rl.Check("ip2")
	if !allowed {
		t.Error("после истечения окна должны быть снова разрешены")
	}
}

func TestRateLimiter_CheckAndIncrement_Atomic(t *testing.T) {
	rl := newTestLimiter(5)
	// Первые 5 — разрешены
	for i := 0; i < 5; i++ {
		allowed, _ := rl.CheckAndIncrement("ip3")
		if !allowed {
			t.Errorf("попытка %d/%d должна быть разрешена", i+1, 5)
		}
	}
	// 6-я — заблокирована
	allowed, retryAfter := rl.CheckAndIncrement("ip3")
	if allowed {
		t.Error("6-я попытка должна быть заблокирована")
	}
	if retryAfter <= 0 {
		t.Error("retryAfter должен быть > 0")
	}
}

func TestRateLimiter_IndependentKeys(t *testing.T) {
	rl := newTestLimiter(1)
	rl.Increment("userA")
	rl.Increment("userA")

	// userA заблокирован
	allowedA, _ := rl.Check("userA")
	if allowedA {
		t.Error("userA должен быть заблокирован")
	}

	// userB независим — не заблокирован
	allowedB, _ := rl.Check("userB")
	if !allowedB {
		t.Error("userB не должен быть заблокирован")
	}
}

// --- CheckInspectionLimit ---

func TestCheckInspectionLimit_AdminBypass(t *testing.T) {
	InspectionLimiter = NewMemoryRateLimiter(1, time.Hour) // лимит 1, но admin проходит
	for i := 0; i < 5; i++ {
		allowed, msg := CheckInspectionLimit(1, "admin")
		if !allowed {
			t.Errorf("admin не должен блокироваться; сообщение: %s", msg)
		}
	}
}

func TestCheckInspectionLimit_UserBlockedAfterLimit(t *testing.T) {
	InspectionLimiter = NewMemoryRateLimiter(3, time.Hour)
	for i := 0; i < 3; i++ {
		allowed, _ := CheckInspectionLimit(42, "inspector")
		if !allowed {
			t.Fatalf("попытка %d должна быть разрешена", i+1)
		}
	}
	allowed, msg := CheckInspectionLimit(42, "inspector")
	if allowed {
		t.Error("4-я попытка должна быть заблокирована")
	}
	if !strings.Contains(msg, "мин") {
		t.Errorf("сообщение должно содержать время ожидания, получили: %q", msg)
	}
}
