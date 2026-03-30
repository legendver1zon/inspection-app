package handlers

import "testing"

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"простое имя", "простое имя"},
		{"с/слешем", "с_слешем"},
		{"с\\обратным", "с_обратным"},
		{"с:двоеточием", "с_двоеточием"},
		{"звёзд*очка", "звёзд_очка"},
		{"вопрос?знак", "вопрос_знак"},
		{`кавыч"ки`, "кавыч_ки"},
		{"угловые<>скобки", "угловые__скобки"},
		{"пайп|символ", "пайп_символ"},
		{"  пробелы  ", "пробелы"},
		{"", ""},
		{"нормальное", "нормальное"},
		{"multiple///slashes", "multiple___slashes"},
	}

	for _, tt := range tests {
		result := sanitizeFolderName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFolderName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSectionFolderName(t *testing.T) {
	tests := []struct {
		section    string
		wallNumber int
		expected   string
	}{
		{"window", 0, "Окна"},
		{"ceiling", 0, "Потолок"},
		{"wall", 0, "Стены"},
		{"wall", 1, "Стены/Стена_1"},
		{"wall", 4, "Стены/Стена_4"},
		{"floor", 0, "Пол"},
		{"door", 0, "Двери"},
		{"plumbing", 0, "Сантехника"},
		{"unknown", 0, "unknown"},
	}

	for _, tt := range tests {
		result := sectionFolderName(tt.section, tt.wallNumber)
		if result != tt.expected {
			t.Errorf("sectionFolderName(%q, %d) = %q, want %q", tt.section, tt.wallNumber, result, tt.expected)
		}
	}
}
