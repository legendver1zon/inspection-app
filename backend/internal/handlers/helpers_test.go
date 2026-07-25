package handlers

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with%percent", "with\\%percent"},
		{"with_underscore", "with\\_underscore"},
		{"100%", "100\\%"},
		{"%admin%", "\\%admin\\%"},
		{"back\\slash", "back\\\\slash"},
		{"mixed%_\\all", "mixed\\%\\_\\\\all"},
		{"", ""},
		{"нормальный текст", "нормальный текст"},
	}

	for _, tt := range tests {
		result := escapeLike(tt.input)
		if result != tt.expected {
			t.Errorf("escapeLike(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
