package cloudstorage

import "io"

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
}
