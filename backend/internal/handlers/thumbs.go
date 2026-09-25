package handlers

import (
	"context"
	"errors"
	"fmt"
	"inspection-app/internal/logger"
	"inspection-app/internal/models"
	"inspection-app/internal/thumbs"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
)

const (
	thumbDownloadTimeout = 30 * time.Second
	thumbMaxSourceSize   = 30 << 20
)

var (
	thumbClient = &http.Client{Timeout: thumbDownloadTimeout}
	thumbGroup  singleflight.Group
)

// GetPhotoThumb — GET /photos/:id/thumb
// Отдаёт миниатюру фото; при отсутствии строит её лениво, при ошибке — редирект на оригинал.
func GetPhotoThumb(c *gin.Context) {
	photo, ok := authorizePhoto(c)
	if !ok {
		return
	}

	dst := thumbs.Path(photo.ID)
	if !fileExists(dst) {
		if err := ensureThumb(c.Request.Context(), &photo); err != nil {
			logger.Ctx(c.Request.Context()).Warn("thumb fallback to original", "photo_id", photo.ID, "error", err)
			c.Header("Cache-Control", "no-store")
			c.Redirect(http.StatusTemporaryRedirect, "/photos/"+strconv.FormatUint(uint64(photo.ID), 10)+"/download")
			return
		}
	}

	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.File(dst)
}

// ensureThumb строит миниатюру, если её ещё нет. Одинаковые запросы схлопываются
// в одну генерацию, число одновременных генераций ограничено семафором thumbs.
func ensureThumb(ctx context.Context, photo *models.Photo) error {
	key := strconv.FormatUint(uint64(photo.ID), 10)
	_, err, _ := thumbGroup.Do(key, func() (any, error) {
		dst := thumbs.Path(photo.ID)
		if fileExists(dst) {
			return nil, nil
		}
		thumbs.Acquire()
		defer thumbs.Release()

		src, err := openThumbSource(ctx, photo)
		if err != nil {
			return nil, err
		}
		defer src.Close()
		return nil, thumbs.Make(src, dst)
	})
	return err
}

// makeThumbAsync строит миниатюру в фоне после загрузки фото.
func makeThumbAsync(photo models.Photo) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("thumb panic", "photo_id", photo.ID, "error", r)
		}
	}()
	if err := ensureThumb(context.Background(), &photo); err != nil {
		logger.Warn("thumb after upload", "photo_id", photo.ID, "error", err)
	}
}

// openThumbSource открывает оригинал фото в том же порядке, что GetPhotoDownload:
// локальный файл, http-ссылка, облако, legacy /static/.
func openThumbSource(ctx context.Context, photo *models.Photo) (io.ReadCloser, error) {
	if p, ok := localPhotoPath(ctx, photo); ok {
		return os.Open(p)
	}
	switch {
	case strings.HasPrefix(photo.FileURL, "http"):
		return downloadThumbSource(photo.FileURL)
	case strings.HasPrefix(photo.FileURL, "/static/"):
		clean := path.Clean(photo.FileURL)
		if !strings.HasPrefix(clean, "/static/") {
			return nil, fmt.Errorf("недопустимый путь %q", photo.FileURL)
		}
		return os.Open(filepath.Join("web", filepath.FromSlash(clean)))
	case cloudStore != nil && photo.FileURL != "":
		u, err := cloudStore.GetDownloadURL(photo.FileURL)
		if err != nil {
			return nil, fmt.Errorf("cloud download url: %w", err)
		}
		return downloadThumbSource(u)
	}
	return nil, errors.New("источник фото недоступен")
}

func downloadThumbSource(url string) (io.ReadCloser, error) {
	resp, err := thumbClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download source: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download source: HTTP %d", resp.StatusCode)
	}
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, thumbMaxSourceSize), resp.Body}, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
