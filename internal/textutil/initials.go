// Package textutil — общие текстовые утилиты, используемые в нескольких пакетах.
package textutil

import "strings"

// Initials генерирует «Фамилия И. О.» из полного ФИО
func Initials(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return fullName
	}
	result := parts[0]
	for i := 1; i < len(parts) && i <= 2; i++ {
		runes := []rune(parts[i])
		if len(runes) > 0 {
			result += " " + string(runes[0]) + "."
		}
	}
	return result
}
