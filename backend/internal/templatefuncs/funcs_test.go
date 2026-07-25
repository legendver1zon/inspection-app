package templatefuncs

import (
	"inspection-app/internal/models"
	"testing"
)

func TestInitials2(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Иванов Иван Иванович", "ИИ"},
		{"Петров", "П"},
		{"", "?"},
		{"A B C D", "AB"},
		{"  пробелы  ", "п"},
	}
	fm := FuncMap()
	fn := fm["initials2"].(func(string) string)
	for _, tt := range tests {
		if got := fn(tt.input); got != tt.want {
			t.Errorf("initials2(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRoomField(t *testing.T) {
	room := &models.InspectionRoom{
		RoomName:   "Кухня",
		WindowType: "pvc",
		WallType:   "paint",
		Length:      4.5,
		Width:      3.2,
		Height:     0, // нулевое значение → пустая строка
		DoorHeight: 2.1,
	}
	roomMap := map[int]*models.InspectionRoom{1: room}

	tests := []struct {
		roomNum int
		field   string
		want    string
	}{
		{1, "name", "Кухня"},
		{1, "window_type", "pvc"},
		{1, "length", "4.5"},
		{1, "width", "3.2"},
		{1, "height", ""},
		{1, "dh", "2.1"},
		{1, "dw", ""},
		{2, "name", ""},     // несуществующая комната
		{1, "unknown", ""},  // неизвестное поле
	}
	for _, tt := range tests {
		got := roomField(roomMap, tt.roomNum, tt.field)
		if got != tt.want {
			t.Errorf("roomField(%d, %q) = %q, want %q", tt.roomNum, tt.field, got, tt.want)
		}
	}
}

func TestRoomExists(t *testing.T) {
	roomMap := map[int]*models.InspectionRoom{1: {}}
	if !roomExists(roomMap, 1) {
		t.Error("roomExists(1) should be true")
	}
	if roomExists(roomMap, 2) {
		t.Error("roomExists(2) should be false")
	}
}

func TestRoomHasDefects(t *testing.T) {
	room1 := models.InspectionRoom{Defects: []models.RoomDefect{{Value: "5мм"}}}
	room2 := models.InspectionRoom{Defects: []models.RoomDefect{{Notes: "трещина"}}}
	room3 := models.InspectionRoom{Defects: []models.RoomDefect{{Value: "", Notes: ""}}}
	room4 := models.InspectionRoom{}

	if !roomHasDefects(room1) {
		t.Error("room1 with Value should have defects")
	}
	if !roomHasDefects(room2) {
		t.Error("room2 with Notes should have defects")
	}
	if roomHasDefects(room3) {
		t.Error("room3 with empty defect should not have defects")
	}
	if roomHasDefects(room4) {
		t.Error("room4 empty should not have defects")
	}
}

func TestSectionDefects(t *testing.T) {
	room := models.InspectionRoom{
		Defects: []models.RoomDefect{
			{Section: "window", Value: "2мм", Notes: ""},
			{Section: "window", Value: "", Notes: "прочее"},
			{Section: "ceiling", Value: "1мм"},
			{Section: "window", Value: "3мм"},
		},
	}
	result := sectionDefects(room, "window")
	if len(result) != 2 {
		t.Errorf("sectionDefects(window) returned %d, want 2", len(result))
	}
}

func TestSectionNotes(t *testing.T) {
	room := models.InspectionRoom{
		Defects: []models.RoomDefect{
			{Section: "window", Value: "2мм"},
			{Section: "window", Notes: "пятна на стекле"},
		},
	}
	got := sectionNotes(room, "window")
	if got != "пятна на стекле" {
		t.Errorf("sectionNotes = %q, want 'пятна на стекле'", got)
	}
	if sectionNotes(room, "floor") != "" {
		t.Error("sectionNotes for missing section should be empty")
	}
}

func TestWallRows(t *testing.T) {
	tid := uint(1)
	tid2 := uint(2)
	room := models.InspectionRoom{
		Defects: []models.RoomDefect{
			{Section: "wall", WallNumber: 1, DefectTemplateID: &tid, DefectTemplate: models.DefectTemplate{Name: "Отклонение"}, Value: "5мм"},
			{Section: "wall", WallNumber: 2, DefectTemplateID: &tid, DefectTemplate: models.DefectTemplate{Name: "Отклонение"}, Value: "3мм"},
			{Section: "wall", WallNumber: 1, DefectTemplateID: &tid2, DefectTemplate: models.DefectTemplate{Name: "Трещина"}, Value: "0.5мм"},
			{Section: "window", Value: "1мм"}, // не wall — игнорируется
		},
	}
	rows := wallRows(room)
	if len(rows) != 2 {
		t.Fatalf("wallRows returned %d rows, want 2", len(rows))
	}
	if rows[0].Name != "Отклонение" || rows[0].W1 != "5мм" || rows[0].W2 != "3мм" {
		t.Errorf("row0: %+v", rows[0])
	}
	if rows[1].Name != "Трещина" || rows[1].W1 != "0.5мм" {
		t.Errorf("row1: %+v", rows[1])
	}
}

func TestWindowTypeName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"pvc", "ПВХ"}, {"al", "Al"}, {"wood", "Дерево"}, {"", ""}, {"other", ""},
	}
	for _, tt := range tests {
		if got := WindowTypeName(tt.in); got != tt.want {
			t.Errorf("WindowTypeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWallTypeName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"paint", "Окраска"},
		{"paint,tile", "Окраска/Плитка"},
		{"paint,tile,gkl", "Окраска/Плитка/ГКЛ"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := WallTypeName(tt.in); got != tt.want {
			t.Errorf("WallTypeName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHasWallType(t *testing.T) {
	if !hasWallType("paint,tile", "tile") {
		t.Error("hasWallType should find tile")
	}
	if hasWallType("paint,tile", "gkl") {
		t.Error("hasWallType should not find gkl")
	}
}

func TestDefectVal(t *testing.T) {
	tid := uint(10)
	room := &models.InspectionRoom{
		Defects: []models.RoomDefect{
			{DefectTemplateID: &tid, WallNumber: 0, Value: "2мм"},
			{DefectTemplateID: &tid, WallNumber: 1, Value: "5мм"},
		},
	}
	roomMap := map[int]*models.InspectionRoom{1: room}

	if got := defectVal(roomMap, 1, 10, 0); got != "2мм" {
		t.Errorf("defectVal wall=0: got %q, want '2мм'", got)
	}
	if got := defectVal(roomMap, 1, 10, 1); got != "5мм" {
		t.Errorf("defectVal wall=1: got %q, want '5мм'", got)
	}
	if got := defectVal(roomMap, 1, 99, 0); got != "" {
		t.Errorf("defectVal missing template: got %q, want empty", got)
	}
	if got := defectVal(roomMap, 2, 10, 0); got != "" {
		t.Errorf("defectVal missing room: got %q, want empty", got)
	}
}

func TestFuncMap_AllKeysPresent(t *testing.T) {
	fm := FuncMap()
	expected := []string{
		"string", "initials2", "defectVal", "notesVal", "roomField",
		"roomExists", "add", "roomHasDefects", "roomHasSection",
		"sectionDefects", "sectionNotes", "sectionNotesDefect",
		"wallRows", "windowTypeName", "wallTypeName", "hasWallType",
	}
	for _, name := range expected {
		if _, ok := fm[name]; !ok {
			t.Errorf("FuncMap missing key %q", name)
		}
	}
}
