package handlers

import (
	"context"
	"inspection-app/internal/logger"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"sync"
	"time"

	"gorm.io/gorm"
)

// --- Debounced sync scheduler ---
// Когда пользователь загружает 10 фото быстро, каждый PostUploadPhoto вызывает ScheduleSync.
// Вместо 10 параллельных safeSync — один запуск через 500ms после последнего вызова.

var (
	syncTimers   = map[uint]*time.Timer{}
	syncTimersMu sync.Mutex
	syncDebounce = 500 * time.Millisecond
)

// ScheduleSync планирует загрузку фото на Яндекс Диск для осмотра.
// Дедупликация: если вызвано несколько раз за 500ms — выполнится один раз.
func ScheduleSync(inspectionID uint) {
	syncTimersMu.Lock()
	defer syncTimersMu.Unlock()

	if t, ok := syncTimers[inspectionID]; ok {
		t.Reset(syncDebounce)
		return
	}
	syncTimers[inspectionID] = time.AfterFunc(syncDebounce, func() {
		syncTimersMu.Lock()
		delete(syncTimers, inspectionID)
		syncTimersMu.Unlock()

		logger.Info("sync scheduled", "inspection_id", inspectionID)
		safeSync(inspectionID)
	})
}

// --- Фоновые циклы самовосстановления ---

const stuckTimeout = 10 * time.Minute // uploading старше этого — считается зависшим

// StartSelfHealLoop запускает фоновые циклы:
// 1. Retry failed фото (каждые 60 сек)
// 2. Восстановление зависших uploading (каждые 2 мин)
// Работает независимо от Redis — всегда запускается в main.go.
func StartSelfHealLoop(ctx context.Context) {
	go func() {
		retryTicker := time.NewTicker(60 * time.Second)
		stuckTicker := time.NewTicker(2 * time.Minute)
		defer retryTicker.Stop()
		defer stuckTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-retryTicker.C:
				retryFailedPhotos()
			case <-stuckTicker.C:
				requeueStuckUploads()
			}
		}
	}()
	logger.Info("self-heal loop started", "retry_interval", "60s", "stuck_interval", "2m", "max_retries", maxFailRetries)
}

// requeueStuckUploads переводит uploading → pending для фото, зависших дольше stuckTimeout.
// Это восстанавливает фото после падения воркера между пометкой uploading и завершением.
func requeueStuckUploads() {
	if cloudStore == nil {
		return
	}

	cutoff := time.Now().Add(-stuckTimeout)
	var stuckIDs []uint
	storage.DB.Model(&models.Photo{}).
		Where("upload_status = 'uploading' AND updated_at < ? AND deleted_at IS NULL", cutoff).
		Pluck("id", &stuckIDs)

	if len(stuckIDs) == 0 {
		return
	}

	now := time.Now()
	storage.DB.Model(&models.Photo{}).Where("id IN ?", stuckIDs).
		Updates(map[string]interface{}{
			"upload_status":   "pending",
			"last_error":      "stuck in uploading (self-heal)",
			"last_attempt_at": now,
		})
	logger.Warn("self-heal: reset stuck uploading→pending", "photos", len(stuckIDs))

	// Находим инспекции для переставленных фото и запускаем загрузку
	var inspectionIDs []uint
	storage.DB.Model(&models.Photo{}).
		Select("DISTINCT inspection_rooms.inspection_id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("photos.id IN ?", stuckIDs).
		Scan(&inspectionIDs)

	for _, id := range inspectionIDs {
		ScheduleSync(id)
	}
}

func retryFailedPhotos() {
	if cloudStore == nil {
		return
	}

	var inspectionIDs []uint
	storage.DB.Model(&models.Photo{}).
		Select("DISTINCT inspection_rooms.inspection_id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("photos.upload_status = 'failed' AND photos.retry_count < ? AND photos.deleted_at IS NULL", maxFailRetries).
		Scan(&inspectionIDs)

	if len(inspectionIDs) == 0 {
		return
	}

	logger.Info("retrying failed photos", "inspections", len(inspectionIDs))

	for _, id := range inspectionIDs {
		var failedIDs []uint
		storage.DB.Table("photos").Select("photos.id").
			Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
			Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
			Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'failed' AND photos.retry_count < ? AND photos.deleted_at IS NULL", id, maxFailRetries).
			Pluck("photos.id", &failedIDs)

		if len(failedIDs) > 0 {
			storage.DB.Model(&models.Photo{}).Where("id IN ?", failedIDs).
				Updates(map[string]interface{}{
					"upload_status": "pending",
					"retry_count":   gorm.Expr("retry_count + 1"),
				})
			logger.Info("retry: reset failed→pending", "inspection_id", id, "photos", len(failedIDs))
			ScheduleSync(id)
		}
	}
}

// TriggerRetryForInspection запускает retry для failed-фото конкретной инспекции.
// Вызывается при просмотре инспекции (soft retry). Не блокирует HTTP-запрос.
func TriggerRetryForInspection(inspectionID uint) {
	if cloudStore == nil {
		return
	}

	var failedCount int64
	storage.DB.Model(&models.Photo{}).
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'failed' AND photos.retry_count < ? AND photos.deleted_at IS NULL",
			inspectionID, maxFailRetries).
		Count(&failedCount)

	if failedCount == 0 {
		return
	}

	var failedIDs []uint
	storage.DB.Table("photos").Select("photos.id").
		Joins("JOIN room_defects ON room_defects.id = photos.defect_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("inspection_rooms.inspection_id = ? AND photos.upload_status = 'failed' AND photos.retry_count < ? AND photos.deleted_at IS NULL",
			inspectionID, maxFailRetries).
		Pluck("photos.id", &failedIDs)

	if len(failedIDs) == 0 {
		return
	}

	storage.DB.Model(&models.Photo{}).Where("id IN ?", failedIDs).
		Updates(map[string]interface{}{
			"upload_status": "pending",
			"retry_count":   gorm.Expr("retry_count + 1"),
		})
	logger.Info("soft retry: reset failed→pending on view", "inspection_id", inspectionID, "photos", len(failedIDs))
	go safeSync(inspectionID)
}
