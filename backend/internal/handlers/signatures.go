package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"inspection-app/internal/models"
	"inspection-app/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Рукописные подписи. PNG с экрана телефона приходит data-URL'ом: в акт —
// полями signature_* формы сохранения (так подпись уезжает вместе с
// автосохранением и без сети), в профиль — JSON'ом. Файлы лежат в
// uploads/signatures с непредсказуемыми именами и отдаются только владельцу
// акта и администратору.

const (
	maxSignatureBytes = 300 * 1024
	maxSignatureSide  = 2000
	errSignedByOwner  = "Акт подписан собственником. Снимите подпись, чтобы вносить правки."
)

var signatureRoles = []string{models.SignatureRoleInspector, models.SignatureRoleOwner}

func uploadsPath(rel string) string {
	return filepath.Join("web", "static", "uploads", filepath.FromSlash(rel))
}

func decodeSignatureDataURL(s string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(s, prefix) {
		return nil, errors.New("ожидается PNG в виде data-URL")
	}
	raw, err := base64.StdEncoding.DecodeString(s[len(prefix):])
	if err != nil {
		return nil, errors.New("повреждённые данные подписи")
	}
	if len(raw) > maxSignatureBytes {
		return nil, fmt.Errorf("подпись больше %d КБ", maxSignatureBytes/1024)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("файл подписи не PNG")
	}
	if cfg.Width < 10 || cfg.Height < 10 || cfg.Width > maxSignatureSide || cfg.Height > maxSignatureSide {
		return nil, errors.New("недопустимый размер подписи")
	}
	return raw, nil
}

func randomSuffix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

// writeSignatureFile кладёт PNG в uploads/signatures/{dir}/{name}-{случайное}.png
// и возвращает путь относительно каталога uploads.
func writeSignatureFile(dir, name string, data []byte) (string, error) {
	rel := path.Join("signatures", dir, name+"-"+randomSuffix()+".png")
	abs := uploadsPath(rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0644); err != nil {
		return "", err
	}
	return rel, nil
}

func removeUploadsFile(rel string) {
	if rel != "" {
		os.Remove(uploadsPath(rel))
	}
}

// parseSignedAt — время подписания с телефона (RFC3339 со смещением зоны);
// при ошибке, из будущего или старше года — сейчас. Смещение возвращается
// отдельно: база хранит UTC, а в акте печатается местное время.
func parseSignedAt(s string) (time.Time, int) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	now := time.Now()
	if err != nil || t.After(now.Add(5*time.Minute)) || t.Before(now.Add(-365*24*time.Hour)) {
		_, off := now.Zone()
		return now, off / 60
	}
	_, off := t.Zone()
	return t, off / 60
}

func setSignature(inspectionID uint, role string, data []byte, signedAt time.Time, tzOffsetMin int) error {
	rel, err := writeSignatureFile(strconv.FormatUint(uint64(inspectionID), 10), role, data)
	if err != nil {
		return err
	}
	var old models.Signature
	found := storage.DB.Where("inspection_id = ? AND role = ?", inspectionID, role).First(&old).Error == nil
	err = storage.DB.Transaction(func(tx *gorm.DB) error {
		if found {
			if err := tx.Delete(&old).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.Signature{InspectionID: inspectionID, Role: role, FilePath: rel, SignedAt: signedAt, TZOffsetMin: tzOffsetMin}).Error
	})
	if err != nil {
		removeUploadsFile(rel)
		return err
	}
	if found {
		removeUploadsFile(old.FilePath)
	}
	return nil
}

func clearSignature(inspectionID uint, role string) {
	var old models.Signature
	if storage.DB.Where("inspection_id = ? AND role = ?", inspectionID, role).First(&old).Error != nil {
		return
	}
	if storage.DB.Delete(&old).Error == nil {
		removeUploadsFile(old.FilePath)
	}
}

// ownerSigned — акт подписан собственником: правки формы, удаление фото и
// самого акта закрыты, пока подпись не снята.
func ownerSigned(inspectionID uint) bool {
	var n int64
	storage.DB.Model(&models.Signature{}).Where("inspection_id = ? AND role = ?", inspectionID, models.SignatureRoleOwner).Count(&n)
	return n > 0
}

// applyFormSignatures обрабатывает поля signature_* формы сохранения акта:
// signature_{role} (data-URL) + signature_{role}_at, signature_{role}_clear,
// signature_inspector_from_profile.
func applyFormSignatures(c *gin.Context, inspection *models.Inspection) error {
	for _, role := range signatureRoles {
		if formFlag(c, "signature_"+role+"_clear") {
			clearSignature(inspection.ID, role)
		}
		if role == models.SignatureRoleInspector && formFlag(c, "signature_inspector_from_profile") {
			user := CurrentUser(c)
			if user.SignaturePath == "" {
				return errors.New("в профиле нет подписи")
			}
			data, err := os.ReadFile(uploadsPath(user.SignaturePath))
			if err != nil {
				return errors.New("подпись профиля недоступна")
			}
			at, off := parseSignedAt(c.PostForm("signature_inspector_at"))
			if err := setSignature(inspection.ID, role, data, at, off); err != nil {
				return err
			}
			continue
		}
		if s := c.PostForm("signature_" + role); s != "" {
			data, err := decodeSignatureDataURL(s)
			if err != nil {
				return err
			}
			at, off := parseSignedAt(c.PostForm("signature_" + role + "_at"))
			if err := setSignature(inspection.ID, role, data, at, off); err != nil {
				return err
			}
		}
	}
	return nil
}

// signaturesJSON — блок подписей для API: inspector/owner → {signed_at, url} или null.
func signaturesJSON(inspectionID uint) gin.H {
	var sigs []models.Signature
	storage.DB.Where("inspection_id = ?", inspectionID).Find(&sigs)
	out := gin.H{models.SignatureRoleInspector: nil, models.SignatureRoleOwner: nil}
	for _, s := range sigs {
		out[s.Role] = gin.H{
			"signed_at": s.SignedLocal().Format(time.RFC3339),
			"url":       fmt.Sprintf("/api/inspections/%d/signature/%s?v=%d", inspectionID, s.Role, s.ID),
		}
	}
	return out
}

// APIGetInspectionSignature — GET /api/inspections/:id/signature/:role (PNG)
func APIGetInspectionSignature(c *gin.Context) {
	inspection, ok := loadInspection(c)
	if !ok {
		return
	}
	var s models.Signature
	if err := storage.DB.Where("inspection_id = ? AND role = ?", inspection.ID, c.Param("role")).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Подписи нет"})
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(uploadsPath(s.FilePath))
}

// APIGetProfileSignature — GET /api/profile/signature (PNG своей подписи)
func APIGetProfileSignature(c *gin.Context) {
	user := CurrentUser(c)
	if user.SignaturePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Подпись не задана"})
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(uploadsPath(user.SignaturePath))
}

// APISetProfileSignature — POST /api/profile/signature {data_url}
func APISetProfileSignature(c *gin.Context) {
	var req struct {
		DataURL string `json:"data_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	data, err := decodeSignatureDataURL(req.DataURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user := CurrentUser(c)
	rel, err := writeSignatureFile("users", "u"+strconv.FormatUint(uint64(user.ID), 10), data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить файл"})
		return
	}
	if err := storage.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("signature_path", rel).Error; err != nil {
		removeUploadsFile(rel)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
		return
	}
	removeUploadsFile(user.SignaturePath)
	var fresh models.User
	storage.DB.First(&fresh, user.ID)
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(fresh)})
}

// APIDeleteProfileSignature — POST /api/profile/signature/delete
func APIDeleteProfileSignature(c *gin.Context) {
	user := CurrentUser(c)
	if err := storage.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("signature_path", "").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения"})
		return
	}
	removeUploadsFile(user.SignaturePath)
	var fresh models.User
	storage.DB.First(&fresh, user.ID)
	c.JSON(http.StatusOK, gin.H{"user": toAPIUser(fresh)})
}
