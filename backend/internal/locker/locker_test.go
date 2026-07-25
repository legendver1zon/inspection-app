package locker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryLocker_Interface(t *testing.T) {
	var _ Locker = NewMemory()
}

func TestMemoryLocker_LockUnlock(t *testing.T) {
	m := NewMemory()
	unlock, err := m.Lock("key1")
	if err != nil {
		t.Fatalf("Lock() error: %v", err)
	}
	unlock()
}

func TestMemoryLocker_SameKeyBlocks(t *testing.T) {
	m := NewMemory()
	unlock1, _ := m.Lock("key1")

	var started atomic.Int32
	done := make(chan struct{})
	go func() {
		started.Store(1)
		unlock2, _ := m.Lock("key1")
		defer unlock2()
		close(done)
	}()

	// Даём горутине время попытаться взять лок
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("вторая горутина не должна была получить лок")
	default:
		// ОК — заблокирована
	}

	unlock1()

	select {
	case <-done:
		// ОК — разблокировалась
	case <-time.After(time.Second):
		t.Fatal("вторая горутина не разблокировалась после unlock")
	}
}

func TestMemoryLocker_DifferentKeysIndependent(t *testing.T) {
	m := NewMemory()
	unlock1, _ := m.Lock("key1")
	defer unlock1()

	done := make(chan struct{})
	go func() {
		unlock2, _ := m.Lock("key2")
		defer unlock2()
		close(done)
	}()

	select {
	case <-done:
		// ОК — разные ключи не блокируют друг друга
	case <-time.After(time.Second):
		t.Fatal("key2 не должен блокироваться при занятом key1")
	}
}

func TestMemoryLocker_ConcurrentSafety(t *testing.T) {
	m := NewMemory()
	var counter int64
	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := m.Lock("shared")
			if err != nil {
				t.Errorf("Lock error: %v", err)
				return
			}
			defer unlock()
			// Без лока тут был бы data race
			atomic.AddInt64(&counter, 1)
		}()
	}
	wg.Wait()

	if counter != goroutines {
		t.Errorf("counter = %d, ожидали %d", counter, goroutines)
	}
}

func TestMemoryLocker_ReusableAfterUnlock(t *testing.T) {
	m := NewMemory()
	for i := 0; i < 5; i++ {
		unlock, err := m.Lock("key")
		if err != nil {
			t.Fatalf("итерация %d: Lock() error: %v", i, err)
		}
		unlock()
	}
}

// TestMemoryLocker_MutualExclusion_NonAtomicCounter — обычная (неатомарная)
// переменная под локом: нарушение взаимного исключения ловится go test -race.
// После завершения все записи должны быть удалены из map (refcount-очистка).
func TestMemoryLocker_MutualExclusion_NonAtomicCounter(t *testing.T) {
	m := NewMemory()
	const goroutines = 50
	var wg sync.WaitGroup
	counter := 0

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := m.Lock("shared-key")
			if err != nil {
				t.Errorf("Lock() error: %v", err)
				return
			}
			counter++
			unlock()
		}()
	}
	wg.Wait()

	if counter != goroutines {
		t.Errorf("counter = %d, ожидали %d (нарушено взаимное исключение)", counter, goroutines)
	}

	m.mu.Lock()
	remaining := len(m.locks)
	m.mu.Unlock()
	if remaining != 0 {
		t.Errorf("map не очищена после освобождения всех ключей: %d записей", remaining)
	}
}

// TestMemoryLocker_CleanupPerKey — каждый уникальный ключ удаляется после unlock.
func TestMemoryLocker_CleanupPerKey(t *testing.T) {
	m := NewMemory()
	for i := 0; i < 100; i++ {
		unlock, err := m.Lock(string(rune('a'+i%26)) + "-key")
		if err != nil {
			t.Fatalf("Lock() error: %v", err)
		}
		unlock()
	}
	m.mu.Lock()
	remaining := len(m.locks)
	m.mu.Unlock()
	if remaining != 0 {
		t.Errorf("утечка записей: %d ключей осталось в map", remaining)
	}
}
