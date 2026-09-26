package handlers

import (
	"fmt"
	"strings"
	"unicode"

	"inspection-app/internal/models"
)

const maxDocNameLen = 150

// documentFileName — имя файла при скачивании: «Акт <номер> - <адрес>.<ext>»
// (адрес нужен, чтобы файлы разных объектов не путались в загрузках) и
// ASCII-запасной вариант для старых клиентов, не понимающих filename*.
func documentFileName(insp models.Inspection, format string) (utf8Name, asciiName string) {
	number := cleanFileNamePart(insp.ActNumber)
	address := cleanFileNamePart(insp.Address)
	if number == "" {
		number = fmt.Sprintf("%d", insp.ID)
	}
	utf8Name = "Акт " + number
	if address != "" {
		utf8Name += " - " + address
	}
	if r := []rune(utf8Name); len(r) > maxDocNameLen {
		utf8Name = strings.TrimSpace(string(r[:maxDocNameLen]))
	}
	utf8Name += "." + format

	ascii := strings.Map(func(r rune) rune {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.') {
			return r
		}
		return '_'
	}, number)
	asciiName = "act_" + ascii + "." + format
	return
}

// cleanFileNamePart убирает символы, недопустимые в именах файлов, и лишние пробелы.
func cleanFileNamePart(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return ' '
		}
		if r < 32 {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// contentDisposition собирает заголовок по RFC 6266: ASCII-имя плюс UTF-8 в filename*.
func contentDisposition(utf8Name, asciiName string) string {
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, asciiName, rfc5987Encode(utf8Name))
}

// rfc5987Encode кодирует всё, кроме attr-char из RFC 5987, как %XX.
func rfc5987Encode(s string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(safe, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
