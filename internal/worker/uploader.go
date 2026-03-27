package worker

import (
	"context"
	"inspection-app/internal/handlers"
	"inspection-app/internal/models"
	"inspection-app/internal/queue"
	"inspection-app/internal/storage"
	"log"
	"sync"
	"time"
)

const (
	requeueInterval = 60 * time.Second // как часто повторять failed-задачи
	maxFailRetries  = 5                // максимум попыток для одного фото
)

// Uploader — фоновый воркер, обрабатывающий задачи из Redis-очереди.
type Uploader struct {
	q    *queue.RedisQueue
	wg   sync.WaitGroup
	done chan struct{}
}

// New создаёт воркер. q может быть nil — тогда Start() немедленно возвращается.
func New(q *queue.RedisQueue) *Uploader {
	return &Uploader{q: q, done: make(chan struct{})}
}

// Start запускает n горутин-воркеров и 1 горутину повторной постановки задач.
// Возвращается сразу; для остановки — вызвать Stop().
func (u *Uploader) Start(ctx context.Context, n int) {
	if u.q == nil {
		return
	}
	// Восстанавливаем задачи после рестарта сервера
	u.recoverOnStartup(ctx)

	for i := 0; i < n; i++ {
		u.wg.Add(1)
		go u.loop(ctx)
	}

	// Горутина повторной постановки "failed" задач в очередь
	u.wg.Add(1)
	go u.requeueFailed(ctx)
}

// Stop ожидает завершения всех горутин воркера.
func (u *Uploader) Stop() {
	close(u.done)
	u.wg.Wait()
}

func (u *Uploader) loop(ctx context.Context) {
	defer u.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-u.done:
			return
		default:
		}

		inspectionID, err := u.q.Pop(ctx)
		if err != nil {
			log.Printf("worker: Pop error: %v", err)
			continue
		}
		if inspectionID == 0 {
			// Timeout от BLPOP — снова ждём
			continue
		}
		u.processJob(inspectionID)
	}
}

func (u *Uploader) processJob(inspectionID uint) {
	log.Printf("worker: processing inspectionID=%d", inspectionID)

	// Проверяем, есть ли pending-фото — если нет, пропускаем
	var count int64
	storage.DB.Model(&models.Photo{}).
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'pending'", inspectionID).
		Count(&count)

	if count == 0 {
		log.Printf("worker: no pending photos for inspectionID=%d, skipping", inspectionID)
		return
	}

	handlers.UploadInspectionPhotos(inspectionID)
	log.Printf("worker: done inspectionID=%d", inspectionID)
}

// recoverOnStartup находит незавершённые задачи после рестарта сервера и возобновляет их.
func (u *Uploader) recoverOnStartup(ctx context.Context) {
	// uploading → pending (воркер умер на полпути)
	storage.DB.Model(&models.Photo{}).
		Where("upload_status = 'uploading'").
		Update("upload_status", "pending")

	// Найти осмотры с pending-фото и поставить их в очередь
	var inspectionIDs []uint
	storage.DB.Model(&models.Photo{}).
		Select("DISTINCT inspection_rooms.inspection_id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("photos.upload_status = 'pending'").
		Scan(&inspectionIDs)

	for _, id := range inspectionIDs {
		if err := u.q.Push(ctx, id); err != nil {
			log.Printf("worker: recover Push inspectionID=%d: %v", id, err)
		} else {
			log.Printf("worker: recovered inspectionID=%d", id)
		}
	}
}

// requeueFailed периодически переставляет failed-фото (до maxFailRetries) обратно в pending.
func (u *Uploader) requeueFailed(ctx context.Context) {
	defer u.wg.Done()
	ticker := time.NewTicker(requeueInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-u.done:
			return
		case <-ticker.C:
			u.retryFailed(ctx)
		}
	}
}

func (u *Uploader) retryFailed(ctx context.Context) {
	var inspectionIDs []uint
	storage.DB.Model(&models.Photo{}).
		Select("DISTINCT inspection_rooms.inspection_id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("photos.upload_status = 'failed'").
		Scan(&inspectionIDs)

	for _, id := range inspectionIDs {
		// Сбрасываем failed → pending через подзапрос (PostgreSQL не поддерживает JOIN в UPDATE через GORM)
		var failedIDs []uint
		storage.DB.Table("photos").Select("photos.id").
			Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
			Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
			Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'failed' AND photos.deleted_at IS NULL", id).
			Pluck("photos.id", &failedIDs)
		if len(failedIDs) > 0 {
			storage.DB.Model(&models.Photo{}).Where("id IN ?", failedIDs).Update("upload_status", "pending")
		}

		if err := u.q.Push(ctx, id); err != nil {
			log.Printf("worker: retryFailed Push inspectionID=%d: %v", id, err)
		}
	}
}
