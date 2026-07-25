// Package locker предоставляет интерфейс блокировок для предотвращения
// параллельной обработки одного ресурса несколькими воркерами.
//
// Текущая реализация — MemoryLocker (sync.Mutex per key).
// Для масштабирования на несколько инстансов замените на RedisLocker:
//
//	type RedisLocker struct{ client *redis.Client }
//	func (r *RedisLocker) Lock(key string) (func(), error) {
//	    // SETNX key value EX ttl
//	    // unlock = DEL key
//	}
//
// Интерфейс тот же — подмена через SetUploadLocker() в main.go.
package locker

import "sync"

// Locker — интерфейс блокировки по строковому ключу.
// Lock возвращает функцию unlock и ошибку.
// Если ресурс уже заблокирован (для неблокирующих реализаций), вернуть ErrLocked.
type Locker interface {
	Lock(key string) (unlock func(), err error)
}

// MemoryLocker — потокобезопасная in-memory реализация на sync.Mutex.
// Подходит для одного инстанса приложения. Записи удаляются из map,
// когда ключ никем не удерживается (refcount) — иначе map рос бы бесконечно.
type MemoryLocker struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	km   sync.Mutex
	refs int
}

// NewMemory создаёт MemoryLocker.
func NewMemory() *MemoryLocker {
	return &MemoryLocker{locks: make(map[string]*lockEntry)}
}

// Lock блокирует ключ. Если ключ уже заблокирован другим воркером — ждёт.
// Возвращает unlock, который ОБЯЗАТЕЛЬНО вызвать через defer.
func (m *MemoryLocker) Lock(key string) (func(), error) {
	m.mu.Lock()
	e, ok := m.locks[key]
	if !ok {
		e = &lockEntry{}
		m.locks[key] = e
	}
	e.refs++
	m.mu.Unlock()

	e.km.Lock()
	return func() {
		e.km.Unlock()
		m.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(m.locks, key)
		}
		m.mu.Unlock()
	}, nil
}
