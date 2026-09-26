package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"inspection-app/internal/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Integration: Generate() creates a valid PDF ---

func TestGenerate_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir() // автоматически очищается

	inspection := &models.Inspection{
		ActNumber:        "42-310326",
		Date:             time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
		InspectionTime:   "10:00",
		Address:          "ул. Тестовая, д. 1, кв. 5",
		RoomsCount:       2,
		Floor:            3,
		TotalArea:        55.5,
		TempOutside:      -5,
		TempInside:       22,
		Humidity:         45.3,
		OwnerName:        "Иванов И.И.",
		DeveloperRepName: "Петров П.П.",
		User:             models.User{FullName: "Сидоров С.С.", Initials: "Сидоров С.С."},
		Rooms: []models.InspectionRoom{
			{
				RoomNumber: 1,
				RoomName:   "Кухня",
				Length:     4.5,
				Width:      3.2,
				Height:     2.7,
				WindowType: "pvc",
				WallType:   "paint,tile",
				Defects: []models.RoomDefect{
					{
						Section: "window",
						Value:   "2 мм",
						DefectTemplate: models.DefectTemplate{
							Name:      "Отклонение от прямолинейности",
							Threshold: "1",
							Unit:      "мм",
						},
					},
					{
						Section: "ceiling",
						Notes:   "Небольшие пятна в углу",
					},
				},
			},
			{
				RoomNumber: 2,
				RoomName:   "Комната",
				Length:     5.0,
				Width:      4.0,
				Height:     2.7,
			},
		},
	}

	path, err := Generate(inspection, tmpDir)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Файл существует
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file not found: %v", err)
	}

	// Файл не пустой (минимум 1KB для реального PDF)
	if info.Size() < 1024 {
		t.Errorf("PDF too small: %d bytes", info.Size())
	}

	// Имя файла содержит номер акта
	if !strings.Contains(path, "act_42-310326.pdf") {
		t.Errorf("unexpected filename: %s", path)
	}

	t.Logf("PDF created: %s (%d bytes)", path, info.Size())
}

func TestGenerate_EmptyInspection(t *testing.T) {
	tmpDir := t.TempDir()

	inspection := &models.Inspection{
		ActNumber: "1-010126",
		Date:      time.Now(),
		User:      models.User{Initials: "Тест Т.Т."},
	}

	path, err := Generate(inspection, tmpDir)
	if err != nil {
		t.Fatalf("Generate() with empty inspection error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PDF file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("PDF is empty")
	}
}

func TestGenerate_WithWallDefects(t *testing.T) {
	tmpDir := t.TempDir()

	tid := uint(1)
	inspection := &models.Inspection{
		ActNumber: "99-310326",
		Date:      time.Now(),
		User:      models.User{Initials: "Тест Т."},
		Rooms: []models.InspectionRoom{
			{
				RoomNumber: 1,
				RoomName:   "Зал",
				WallType:   "paint",
				Defects: []models.RoomDefect{
					{
						Section:          "wall",
						Value:            "5 мм",
						WallNumber:       1,
						DefectTemplateID: &tid,
						DefectTemplate:   models.DefectTemplate{Name: "Отклонение от вертикали"},
					},
					{
						Section:          "wall",
						Value:            "3 мм",
						WallNumber:       2,
						DefectTemplateID: &tid,
						DefectTemplate:   models.DefectTemplate{Name: "Отклонение от вертикали"},
					},
				},
			},
		},
	}

	path, err := Generate(inspection, tmpDir)
	if err != nil {
		t.Fatalf("Generate() with wall defects error: %v", err)
	}

	info, _ := os.Stat(path)
	t.Logf("PDF with walls: %d bytes", info.Size())
}

// --- Unit tests ---

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

func TestSplitByWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"слово", 1},
		{"два слова", 2},
		{"три слова тут", 3},
		{"   пробелы   ", 1},
	}
	for _, tt := range tests {
		result := splitByWords(tt.input)
		if len(result) != tt.expected {
			t.Errorf("splitByWords(%q) returned %d parts, want %d", tt.input, len(result), tt.expected)
		}
	}
}

func TestRoomHasAnyDefects(t *testing.T) {
	room1 := &models.InspectionRoom{
		Defects: []models.RoomDefect{
			{Value: "трещина"},
		},
	}
	if !roomHasAnyDefects(room1) {
		t.Error("roomHasAnyDefects должен вернуть true для комнаты с дефектом")
	}

	room2 := &models.InspectionRoom{
		Defects: []models.RoomDefect{
			{Value: "", Notes: ""},
		},
	}
	if roomHasAnyDefects(room2) {
		t.Error("roomHasAnyDefects должен вернуть false для комнаты без данных")
	}

	room3 := &models.InspectionRoom{}
	if roomHasAnyDefects(room3) {
		t.Error("roomHasAnyDefects должен вернуть false для пустой комнаты")
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
		{"значение 0,6 мм", 1},         // десятичная запятая НЕ разбивается
		{"0,6 мм, трещина, 1,2 мм", 3}, // разбивается только между словами
	}
	for _, tt := range tests {
		result := splitByCommas(tt.input)
		if len(result) != tt.expected {
			t.Errorf("splitByCommas(%q) returned %d parts %v, want %d", tt.input, len(result), result, tt.expected)
		}
	}
}

// Заготовки дефектов (picked без значения) не должны давать ни строк, ни разделов:
// PDF с такими дефектами совпадает по размеру с PDF без дефектов вовсе.
func TestGenerate_EmptyDefectsNotRendered(t *testing.T) {
	tmpDir := t.TempDir()
	tid := uint(1)
	wallTid := uint(2)
	date := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)

	build := func(defects []models.RoomDefect) *models.Inspection {
		return &models.Inspection{
			ActNumber: "7-310326",
			Date:      date,
			User:      models.User{Initials: "Тест Т."},
			Rooms: []models.InspectionRoom{
				{RoomNumber: 1, RoomName: "Зал", WallType: "paint", Defects: defects},
			},
		}
	}
	empty := []models.RoomDefect{
		{Section: "window", DefectTemplateID: &tid, DefectTemplate: models.DefectTemplate{Name: "Царапина"}},
		{Section: "wall", WallNumber: 2, DefectTemplateID: &wallTid, DefectTemplate: models.DefectTemplate{Name: "Отклонение"}},
		{Section: "ceiling"},
	}

	withEmpty, err := Generate(build(empty), tmpDir)
	if err != nil {
		t.Fatalf("Generate() с пустыми дефектами: %v", err)
	}
	without, err := Generate(build(nil), filepath.Join(tmpDir, "b"))
	if err != nil {
		t.Fatalf("Generate() без дефектов: %v", err)
	}
	a, _ := os.Stat(withEmpty)
	b, _ := os.Stat(without)
	if a.Size() != b.Size() {
		t.Errorf("пустые дефекты попали в PDF: %d байт против %d без дефектов", a.Size(), b.Size())
	}
}

// Подпись собственника рисуется картинкой над линией, температура скрыта.
func TestGenerate_HideClimateAndSignature(t *testing.T) {
	tmp := t.TempDir()
	sigDir := filepath.Join(tmp, "web", "static", "uploads", "signatures", "1")
	if err := os.MkdirAll(sigDir, 0755); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 120, 40))
	for x := 10; x < 110; x++ {
		img.Set(x, 20, color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sigDir, "owner-test.png"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	inspection := &models.Inspection{
		ActNumber: "7-260926", Date: time.Now(), Address: "ул. Подписная, 1",
		OwnerName: "Иванов И.И.", HideClimate: true,
		User:       models.User{Initials: "Сидоров С.С."},
		Signatures: []models.Signature{{Role: models.SignatureRoleOwner, FilePath: "signatures/1/owner-test.png", SignedAt: time.Now()}},
	}
	out := filepath.Join(tmp, "out")
	path, err := Generate(inspection, out)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("PDF не создан: %v", err)
	}
}
