package security

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter — rate limiter на базе Redis (INCR + EXPIRE).
// Работает корректно при нескольких инстансах приложения.
type RedisRateLimiter struct {
	client *redis.Client
	prefix string
	max    int
	window time.Duration
}

// NewRedisRateLimiter создаёт лимитер на базе Redis.
func NewRedisRateLimiter(client *redis.Client, prefix string, max int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		prefix: prefix,
		max:    max,
		window: window,
	}
}

func (rl *RedisRateLimiter) key(id string) string {
	return fmt.Sprintf("ratelimit:%s:%s", rl.prefix, id)
}

// Check проверяет лимит без инкремента.
func (rl *RedisRateLimiter) Check(id string) (bool, time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	val, err := rl.client.Get(ctx, rl.key(id)).Int()
	if err == redis.Nil {
		return true, 0
	}
	if err != nil {
		return true, 0 // при ошибке Redis — пропускаем (fail open)
	}
	if val >= rl.max {
		ttl, _ := rl.client.TTL(ctx, rl.key(id)).Result()
		return false, ttl
	}
	return true, 0
}

// Increment увеличивает счётчик.
func (rl *RedisRateLimiter) Increment(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	k := rl.key(id)
	pipe := rl.client.Pipeline()
	pipe.Incr(ctx, k)
	pipe.Expire(ctx, k, rl.window)
	pipe.Exec(ctx) //nolint:errcheck
}

// CheckAndIncrement атомарно проверяет и инкрементирует.
func (rl *RedisRateLimiter) CheckAndIncrement(id string) (bool, time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	k := rl.key(id)
	val, err := rl.client.Incr(ctx, k).Result()
	if err != nil {
		return true, 0 // fail open
	}
	if val == 1 {
		rl.client.Expire(ctx, k, rl.window)
	}
	if int(val) > rl.max {
		ttl, _ := rl.client.TTL(ctx, k).Result()
		return false, ttl
	}
	return true, 0
}

// Reset сбрасывает счётчик.
func (rl *RedisRateLimiter) Reset(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rl.client.Del(ctx, rl.key(id)) //nolint:errcheck
}
