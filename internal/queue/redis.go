package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const queueKey = "inspection_app:upload_jobs"

// Job описывает задачу загрузки фото на облако.
type Job struct {
	InspectionID uint      `json:"inspection_id"`
	EnqueuedAt   time.Time `json:"enqueued_at"`
}

// RedisQueue — очередь задач на базе Redis List (RPUSH / BLPOP).
type RedisQueue struct {
	client *redis.Client
}

// New создаёт RedisQueue из REDIS_URL (redis://host:port) или адреса напрямую.
func New(redisURL string) (*RedisQueue, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("queue: invalid REDIS_URL %q: %w", redisURL, err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("queue: redis ping failed: %w", err)
	}
	return &RedisQueue{client: client}, nil
}

// NewFromEnv создаёт очередь из переменной окружения REDIS_URL.
// Если переменная не задана — возвращает nil, nil (Redis опционален).
func NewFromEnv() (*RedisQueue, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return nil, nil
	}
	return New(url)
}

// Push добавляет задачу загрузки фото в конец очереди.
func (q *RedisQueue) Push(ctx context.Context, inspectionID uint) error {
	job := Job{InspectionID: inspectionID, EnqueuedAt: time.Now()}
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.RPush(ctx, queueKey, data).Err()
}

// Pop ждёт и извлекает задачу из начала очереди (блокирующий вызов).
// Возвращает (0, nil) при истечении timeout или отмене context.
func (q *RedisQueue) Pop(ctx context.Context) (uint, error) {
	result, err := q.client.BLPop(ctx, 5*time.Second, queueKey).Result()
	if err == redis.Nil || err == context.DeadlineExceeded || err == context.Canceled {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(result) < 2 {
		return 0, nil
	}
	var job Job
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return 0, err
	}
	return job.InspectionID, nil
}

// Len возвращает длину очереди.
func (q *RedisQueue) Len(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, queueKey).Result()
}

// Close закрывает соединение с Redis.
func (q *RedisQueue) Close() error {
	return q.client.Close()
}
