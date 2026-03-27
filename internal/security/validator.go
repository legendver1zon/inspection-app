package security

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	MaxAvatarSize = 5 * 1024 * 1024  // 5 MB
	MaxPlanSize   = 20 * 1024 * 1024 // 20 MB
)

var allowedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

var allowedImageMIMEs = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// ValidatePassword проверяет сложность пароля:
// — минимум 6 символов
// — хотя бы одна заглавная буква
// — хотя бы одна цифра
// — хотя бы один спецсимвол
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return errors.New("пароль должен содержать минимум 6 символов")
	}
	var hasUpper, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errors.New("пароль должен содержать хотя бы одну заглавную букву")
	}
	if !hasDigit {
		return errors.New("пароль должен содержать хотя бы одну цифру")
	}
	if !hasSpecial {
		return errors.New("пароль должен содержать хотя бы один спецсимвол (!, @, #, $ и т.д.)")
	}
	return nil
}

// ValidateImage проверяет загружаемый файл:
// — расширение (.jpg / .jpeg / .png / .webp)
// — размер (≤ maxBytes)
// — реальный MIME-тип по содержимому (первые 512 байт)
func ValidateImage(header *multipart.FileHeader, maxBytes int64) error {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExts[ext] {
		return fmt.Errorf("недопустимый формат файла (разрешены: jpg, jpeg, png, webp)")
	}

	if header.Size > maxBytes {
		return fmt.Errorf("файл слишком большой (максимум %d МБ)", maxBytes/1024/1024)
	}

	f, err := header.Open()
	if err != nil {
		return fmt.Errorf("ошибка открытия файла")
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	detected := http.DetectContentType(buf[:n])
	// http.DetectContentType может вернуть "image/jpeg; ..." — берём только базовый тип
	mimeBase := strings.SplitN(detected, ";", 2)[0]
	if !allowedImageMIMEs[mimeBase] {
		return fmt.Errorf("файл не является изображением (обнаружен тип: %s)", mimeBase)
	}

	return nil
}
