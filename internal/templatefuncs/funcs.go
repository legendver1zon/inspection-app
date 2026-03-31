package templatefuncs

import (
	"fmt"
	"html/template"
	"inspection-app/internal/models"
	"strings"
)

// WallRow — строка таблицы стеновых дефектов (ст1-ст4)
type WallRow struct {
	Name, W1, W2, W3, W4 string
}

// FuncMap возвращает все template-функции для HTML-шаблонов.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"string": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},

		"initials2": initials2,

		"defectVal":  defectVal,
		"notesVal":   notesVal,
		"roomField":  roomField,
		"roomExists": roomExists,
		"add":        func(a, b int) int { return a + b },

		"roomHasDefects":    roomHasDefects,
		"roomHasSection":    roomHasSection,
		"sectionDefects":    sectionDefects,
		"sectionNotes":      sectionNotes,
		"sectionNotesDefect": sectionNotesDefect,
		"wallRows":          wallRows,

		"windowTypeName": WindowTypeName,
		"wallTypeName":   WallTypeName,
		"hasWallType":    hasWallType,
	}
}

// initials2 — первые буквы первых двух слов (аббревиатура для аватара)
func initials2(s string) string {
	parts := strings.Fields(s)
	result := ""
	for i, p := range parts {
		if i >= 2 {
			break
		}
		r := []rune(p)
		if len(r) > 0 {
			result += string(r[0])
		}
	}
	if result == "" {
		return "?"
	}
	return result
}

// defectVal — значение дефекта для комнаты из roomMap
func defectVal(roomMap map[int]*models.InspectionRoom, roomNum int, templateID uint, wallNum int) string {
	if room, ok := roomMap[roomNum]; ok && room != nil {
		for _, d := range room.Defects {
			if d.DefectTemplateID != nil && *d.DefectTemplateID == templateID && d.WallNumber == wallNum {
				return d.Value
			}
		}
	}
	return ""
}

// notesVal — текст "Прочее" для секции комнаты
func notesVal(roomMap map[int]*models.InspectionRoom, roomNum int, section string) string {
	if room, ok := roomMap[roomNum]; ok && room != nil {
		for _, d := range room.Defects {
			if d.DefectTemplateID == nil && d.Section == section {
				return d.Notes
			}
		}
	}
	return ""
}

// roomField — поле замеров/типов комнаты
func roomField(roomMap map[int]*models.InspectionRoom, roomNum int, field string) string {
	room, ok := roomMap[roomNum]
	if !ok || room == nil {
		return ""
	}
	switch field {
	case "name":
		return room.RoomName
	case "window_type":
		return room.WindowType
	case "wall_type":
		return room.WallType
	case "length":
		return fmtNonZero(room.Length)
	case "width":
		return fmtNonZero(room.Width)
	case "height":
		return fmtNonZero(room.Height)
	case "w1h":
		return fmtNonZero(room.Window1Height)
	case "w1w":
		return fmtNonZero(room.Window1Width)
	case "w2h":
		return fmtNonZero(room.Window2Height)
	case "w2w":
		return fmtNonZero(room.Window2Width)
	case "w3h":
		return fmtNonZero(room.Window3Height)
	case "w3w":
		return fmtNonZero(room.Window3Width)
	case "w4h":
		return fmtNonZero(room.Window4Height)
	case "w4w":
		return fmtNonZero(room.Window4Width)
	case "w5h":
		return fmtNonZero(room.Window5Height)
	case "w5w":
		return fmtNonZero(room.Window5Width)
	case "dh":
		return fmtNonZero(room.DoorHeight)
	case "dw":
		return fmtNonZero(room.DoorWidth)
	}
	return ""
}

func fmtNonZero(v float64) string {
	if v != 0 {
		return fmt.Sprintf("%g", v)
	}
	return ""
}

// roomExists — есть ли комната в roomMap
func roomExists(roomMap map[int]*models.InspectionRoom, roomNum int) bool {
	room, ok := roomMap[roomNum]
	return ok && room != nil
}

// roomHasDefects — есть ли хоть один дефект в помещении
func roomHasDefects(room models.InspectionRoom) bool {
	for _, d := range room.Defects {
		if d.Value != "" || d.Notes != "" {
			return true
		}
	}
	return false
}

// roomHasSection — есть ли данные в секции помещения
func roomHasSection(room models.InspectionRoom, section string) bool {
	for _, d := range room.Defects {
		if d.Section == section && (d.Value != "" || d.Notes != "") {
			return true
		}
	}
	return false
}

// sectionDefects — дефекты секции (только с Value, без Notes)
func sectionDefects(room models.InspectionRoom, section string) []models.RoomDefect {
	var result []models.RoomDefect
	for _, d := range room.Defects {
		if d.Section == section && d.Notes == "" && d.Value != "" {
			result = append(result, d)
		}
	}
	return result
}

// sectionNotes — текст "Прочее" для секции
func sectionNotes(room models.InspectionRoom, section string) string {
	for _, d := range room.Defects {
		if d.Section == section && d.Notes != "" {
			return d.Notes
		}
	}
	return ""
}

// sectionNotesDefect — полный объект RoomDefect для записи "Прочее" секции (nil если нет)
func sectionNotesDefect(room models.InspectionRoom, section string) *models.RoomDefect {
	for i := range room.Defects {
		if room.Defects[i].DefectTemplateID == nil && room.Defects[i].Section == section && room.Defects[i].Notes != "" {
			return &room.Defects[i]
		}
	}
	return nil
}

// wallRows — дефекты стен, сгруппированные по шаблону (для таблицы ст1-ст4)
func wallRows(room models.InspectionRoom) []WallRow {
	type entry struct {
		name   string
		values [5]string
	}
	entries := make(map[uint]*entry)
	order := []uint{}
	for _, d := range room.Defects {
		if d.Section != "wall" || d.Notes != "" || d.DefectTemplateID == nil || d.WallNumber < 1 || d.WallNumber > 4 {
			continue
		}
		tid := *d.DefectTemplateID
		if _, ok := entries[tid]; !ok {
			entries[tid] = &entry{name: d.DefectTemplate.Name}
			order = append(order, tid)
		}
		entries[tid].values[d.WallNumber] = d.Value
	}
	rows := make([]WallRow, 0, len(order))
	for _, id := range order {
		e := entries[id]
		rows = append(rows, WallRow{Name: e.name, W1: e.values[1], W2: e.values[2], W3: e.values[3], W4: e.values[4]})
	}
	return rows
}

// WindowTypeName — отображаемое название типа окна
func WindowTypeName(t string) string {
	switch t {
	case "pvc":
		return "ПВХ"
	case "al":
		return "Al"
	case "wood":
		return "Дерево"
	}
	return ""
}

// WallTypeName — отображаемые названия типов стен (через запятую)
func WallTypeName(t string) string {
	var names []string
	for _, p := range strings.Split(t, ",") {
		switch strings.TrimSpace(p) {
		case "paint":
			names = append(names, "Окраска")
		case "tile":
			names = append(names, "Плитка")
		case "gkl":
			names = append(names, "ГКЛ")
		}
	}
	return strings.Join(names, "/")
}

// hasWallType — есть ли конкретный тип в строке wall_type
func hasWallType(wallType, checkType string) bool {
	for _, p := range strings.Split(wallType, ",") {
		if strings.TrimSpace(p) == checkType {
			return true
		}
	}
	return false
}
