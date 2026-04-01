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

// --- Retry loop для failed фото (работает без Redis) ---

// StartFailedRetryLoop запускает фоновую горутину, которая каждые 60 секунд
// ищет failed фото (с retry_count < maxFailRetries), сбрасывает в pending и запускает загрузку.
// Работает независимо от Redis — всегда запускается в main.go.
func StartFailedRetryLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				retryFailedPhotos()
			}
		}
	}()
	logger.Info("failed photo retry loop started", "interval", "60s", "max_retries", maxFailRetries)
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
