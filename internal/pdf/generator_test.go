package pdf

import "testing"

func TestFmtFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, ""},
		{1.5, "1.5"},
		{10, "10"},
		{3.14159, "3.14159"},
		{-5, "-5"},
		{0.001, "0.001"},
	}
	for _, tt := range tests {
		result := fmtFloat(tt.input)
		if result != tt.expected {
			t.Errorf("fmtFloat(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFmtTempOutside(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, ""},
		{-5, "-5°C"},
		{-10.5, "-10.5°C"},
		{5, "-5°C"}, // Намеренное поведение: всегда минус (для тепловизии)
	}
	for _, tt := range tests {
		result := fmtTempOutside(tt.input)
		if result != tt.expected {
			t.Errorf("fmtTempOutside(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFmtTempInside(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, ""},
		{22, "+22°C"},
		{-3, "+3°C"}, // Намеренное поведение: всегда плюс (внутренняя температура)
	}
	for _, tt := range tests {
		result := fmtTempInside(tt.input)
		if result != tt.expected {
			t.Errorf("fmtTempInside(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFmtHumidity(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, ""},
		{45.3, "45.3%"},
		{100, "100.0%"},
	}
	for _, tt := range tests {
		result := fmtHumidity(tt.input)
		if result != tt.expected {
			t.Errorf("fmtHumidity(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestWindowTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"pvc", "ПВХ"},
		{"al", "Al"},
		{"wood", "Дерево"},
		{"", "—"},
		{"unknown", "—"},
	}
	for _, tt := range tests {
		result := windowTypeName(tt.input)
		if result != tt.expected {
			t.Errorf("windowTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestWallTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"paint", "Окраска"},
		{"tile", "Плитка"},
		{"gkl", "ГКЛ"},
		{"paint,tile", "Окраска/Плитка"},
		{"paint,tile,gkl", "Окраска/Плитка/ГКЛ"},
		{"", "—"},
		{"unknown", "—"},
	}
	for _, tt := range tests {
		result := wallTypeName(tt.input)
		if result != tt.expected {
			t.Errorf("wallTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSplitByCommas(t *testing.T) {
	tests := []struct {
		input    string
		expected int // количество частей
	}{
		{"", 1},
		{"простой текст", 1},
		{"раз, два, три", 3},
		{"значение 0,6 мм", 1},        // десятичная запятая НЕ разбивается
		{"0,6 мм, трещина, 1,2 мм", 3}, // разбивается только между словами
	}
	for _, tt := range tests {
		result := splitByCommas(tt.input)
		if len(result) != tt.expected {
			t.Errorf("splitByCommas(%q) returned %d parts %v, want %d", tt.input, len(result), result, tt.expected)
		}
	}
}
