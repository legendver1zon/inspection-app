package cloudstorage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const yadiskAPI = "https://cloud-api.yandex.net/v1/disk"

// YandexDisk реализует FileStorage через REST API Яндекс Диска.
// Документация: https://yandex.ru/dev/disk-api/doc/ru/
type YandexDisk struct {
	token   string // OAuth-токен (YADISK_TOKEN)
	rootDir string // Корневая папка, например "disk:/inspection-app"
	client  *http.Client
}

// NewYandexDisk создаёт адаптер.
// token — OAuth-токен Яндекс Диска.
// rootDir — корневая папка (если пустой, используется "disk:/inspection-app").
func NewYandexDisk(token, rootDir string) *YandexDisk {
	if rootDir == "" {
		rootDir = "disk:/inspection-app"
	}
	return &YandexDisk{
		token:   token,
		rootDir: strings.TrimRight(rootDir, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// fullPath собирает полный путь Яндекс Диска из относительного.
func (y *YandexDisk) fullPath(relPath string) string {
	return y.rootDir + "/" + strings.TrimLeft(relPath, "/")
}

// EnsurePath создаёт все промежуточные папки в пути.
// Каждый сегмент создаётся отдельным запросом; ответ 409 (уже существует) игнорируется.
func (y *YandexDisk) EnsurePath(relPath string) error {
	segments := strings.Split(strings.Trim(relPath, "/"), "/")
	built := y.rootDir

	// Убеждаемся, что корневая папка существует
	if err := y.mkdir(built); err != nil {
		return fmt.Errorf("mkdir %q: %w", built, err)
	}

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		built += "/" + seg
		if err := y.mkdir(built); err != nil {
			return fmt.Errorf("mkdir %q: %w", built, err)
		}
	}
	return nil
}

// mkdir создаёт одну папку. Ответ 409 (ConflictError — уже существует) не является ошибкой.
func (y *YandexDisk) mkdir(fullPath string) error {
	endpoint := yadiskAPI + "/resources?path=" + url.QueryEscape(fullPath)
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "OAuth "+y.token)

	resp, err := y.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 201 Created — успешно, 409 Conflict — уже существует (ok)
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("mkdir %q: HTTP %d: %s", fullPath, resp.StatusCode, body)
}

// UploadFile загружает файл по относительному пути.
func (y *YandexDisk) UploadFile(relPath string, r io.Reader) error {
	path := y.fullPath(relPath)

	// Шаг 1: получаем URL для загрузки
	uploadURL, err := y.getUploadURL(path)
	if err != nil {
		return fmt.Errorf("get upload URL: %w", err)
	}

	// Шаг 2: загружаем файл методом PUT
	req, err := http.NewRequest(http.MethodPut, uploadURL, r)
	if err != nil {
		return err
	}

	resp, err := y.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Яндекс Диск возвращает 201 Created при успешной загрузке
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload %q: HTTP %d: %s", path, resp.StatusCode, body)
	}
	return nil
}

// getUploadURL запрашивает у Яндекс Диска временный URL для загрузки файла.
func (y *YandexDisk) getUploadURL(fullPath string) (string, error) {
	endpoint := yadiskAPI + "/resources/upload?path=" + url.QueryEscape(fullPath) + "&overwrite=true"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "OAuth "+y.token)

	resp, err := y.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get upload URL for %q: HTTP %d: %s", fullPath, resp.StatusCode, body)
	}

	var result struct {
		Href string `json:"href"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload URL response: %w", err)
	}
	if result.Href == "" {
		return "", fmt.Errorf("empty upload URL for %q", fullPath)
	}
	return result.Href, nil
}

// PublishFolder публикует папку и возвращает публичную ссылку.
func (y *YandexDisk) PublishFolder(relPath string) (string, error) {
	return y.publish(y.fullPath(relPath))
}

// PublishFile публикует файл и возвращает публичную ссылку.
func (y *YandexDisk) PublishFile(relPath string) (string, error) {
	return y.publish(y.fullPath(relPath))
}

// publish публикует ресурс (файл или папку) и возвращает его public_url.
func (y *YandexDisk) publish(fullPath string) (string, error) {
	// Шаг 1: публикуем ресурс
	endpoint := yadiskAPI + "/resources/publish?path=" + url.QueryEscape(fullPath)
	req, err := http.NewRequest(http.MethodPut, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "OAuth "+y.token)

	resp, err := y.client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("publish %q: HTTP %d: %s", fullPath, resp.StatusCode, body)
	}

	// Шаг 2: получаем метаданные ресурса с public_url
	return y.getPublicURL(fullPath)
}

// getPublicURL запрашивает метаданные ресурса и возвращает поле public_url.
func (y *YandexDisk) getPublicURL(fullPath string) (string, error) {
	endpoint := yadiskAPI + "/resources?path=" + url.QueryEscape(fullPath) + "&fields=public_url"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "OAuth "+y.token)

	resp, err := y.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get resource %q: HTTP %d: %s", fullPath, resp.StatusCode, body)
	}

	var result struct {
		PublicURL string `json:"public_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode resource response: %w", err)
	}
	return result.PublicURL, nil
}
