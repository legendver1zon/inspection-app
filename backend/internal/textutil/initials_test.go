package textutil

import "testing"

func TestInitials(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Иванов Иван Иванович", "Иванов И. И."},
		{"Петров Пётр", "Петров П."},
		{"Сидоров", "Сидоров"},
		{"", ""},
	}
	for _, tc := range cases {
		got := Initials(tc.input)
		if got != tc.want {
			t.Errorf("Initials(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
