package cloudstorage

import (
	"fmt"
	"io"
)

// UploadError оборачивает ошибку загрузки с HTTP-статусом.
// Позволяет отличить retryable-ошибки (5xx, таймаут) от permanent (4xx).
type UploadError struct {
	StatusCode int    // HTTP-статус (0 = сетевая ошибка / таймаут)
	Message    string // тело ответа или описание ошибки
	Err        error  // оригинальная ошибка
}

func (e *UploadError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	return e.Err.Error()
}

func (e *UploadError) Unwrap() error { return e.Err }

// Retryable возвращает true, если ошибку имеет смысл повторить.
// Retryable: сетевые ошибки (StatusCode=0), 5xx, 429 (rate limit).
// Permanent: 4xx (кроме 429) — повтор не поможет.
func (e *UploadError) Retryable() bool {
	if e.StatusCode == 0 {
		return true // сетевая ошибка / таймаут
	}
	if e.StatusCode == 429 {
		return true // rate limit
	}
	return e.StatusCode >= 500
}

// IsRetryable проверяет, является ли ошибка retryable.
// Для обычных ошибок (не *UploadError) считаем retryable (на всякий случай).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ue *UploadError
	if ok := errorAs(err, &ue); ok {
		return ue.Retryable()
	}
	return true // неизвестная ошибка — попробуем повторить
}

// errorAs — обёртка над errors.As для удобства (избегаем import cycle).
func errorAs(err error, target interface{}) bool {
	// Используем стандартный механизм Go: проверяем цепочку Unwrap.
	type unwrapper interface{ Unwrap() error }
	for {
		if ue, ok := err.(*UploadError); ok {
			*target.(**UploadError) = ue
			return true
		}
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
}

// FileStorage — интерфейс облачного хранилища файлов.
// Реализуется адаптерами для конкретных провайдеров (Яндекс Диск, Google Drive и др.)
type FileStorage interface {
	// EnsurePath создаёт цепочку папок по относительному пути.
	// Если папка уже существует — не возвращает ошибку.
	EnsurePath(relPath string) error

	// UploadFile загружает файл по относительному пути.
	// Путь включает имя файла, например: "inspections/25/Кухня/10/photo_1.jpg"
	UploadFile(relPath string, r io.Reader) error

	// PublishFolder публикует папку (делает её публично доступной)
	// и возвращает публичную ссылку на неё.
	PublishFolder(relPath string) (publicURL string, err error)

	// PublishFile публикует файл и возвращает публичную ссылку.
	PublishFile(relPath string) (publicURL string, err error)

	// FolderExists проверяет существование папки по относительному пути.
	FolderExists(relPath string) (bool, error)

	// MoveFolder переименовывает (перемещает) папку на облаке.
	MoveFolder(oldRelPath, newRelPath string) error

	// GetDownloadURL возвращает временный URL для скачивания файла.
	GetDownloadURL(relPath string) (string, error)
}
