package pdf

import (
	"fmt"
	"inspection-app/internal/models"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-pdf/fpdf"
)

const (
	fontPath    = "C:/Windows/Fonts/arial.ttf"
	fontBoldPath = "C:/Windows/Fonts/arialbd.ttf"
	pageW       = 210.0
	pageH       = 297.0
	marginL     = 15.0
	marginR     = 15.0
	marginT     = 15.0
	contentW    = pageW - marginL - marginR
)

// Generate создаёт PDF для акта осмотра и возвращает путь к файлу
func Generate(inspection *models.Inspection, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать директорию: %w", err)
	}

	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(marginL, marginT, marginR)
	f.SetAutoPageBreak(true, 15)

	// Подключаем шрифт с кириллицей
	if _, err := os.Stat(fontPath); err == nil {
		f.AddUTF8Font("Arial", "", fontPath)
		f.AddUTF8Font("Arial", "B", fontBoldPath)
	} else {
		// Fallback: встроенный шрифт (без кириллицы)
		f.SetFont("Helvetica", "", 10)
	}

	// ===== Страница 1: Шапка акта =====
	f.AddPage()
	setFont(f, "B", 13)
	f.CellFormat(contentW, 8, "Акт осмотра объекта №"+inspection.ActNumber, "B", 1, "C", false, 0, "")
	f.Ln(2)

	setFont(f, "", 10)
	row2col(f, "Дата обследования:", inspection.Date.Format("02.01.2006"), "Время обследования:", inspection.InspectionTime)
	f.Ln(1)
	labelValue(f, "Адрес:", inspection.Address)
	f.Ln(2)
	row4col(f,
		"Кол-во комнат:", strconv.Itoa(inspection.RoomsCount),
		"Этаж:", strconv.Itoa(inspection.Floor),
		"Общая площадь:", fmtFloat(inspection.TotalArea)+" м²",
	)
	row4col(f,
		"t наружн.=", fmtFloat(inspection.TempOutside)+"°C",
		"t внутр.=", fmtFloat(inspection.TempInside)+"°C",
		"RH=", fmtFloat(inspection.Humidity)+"%",
	)

	// План помещений
	f.Ln(4)
	setFont(f, "B", 11)
	f.CellFormat(contentW, 7, "ПЛАН ПОМЕЩЕНИЙ", "", 1, "C", false, 0, "")

	if inspection.PlanImage != "" {
		imgPath := "web/static/uploads/" + filepath.Base(inspection.PlanImage)
		if _, err := os.Stat(imgPath); err == nil {
			f.ImageOptions(imgPath, marginL, f.GetY()+2, contentW, 80, false,
				fpdf.ImageOptions{ImageType: "", ReadDpi: true}, 0, "")
			f.Ln(84)
		}
	} else {
		f.Rect(marginL, f.GetY()+2, contentW, 80, "D")
		f.Ln(84)
	}

	// Таблица замеров помещений
	drawMeasurementsTable(f, inspection.Rooms)

	// ===== Страницы 2+: Дефекты по помещениям =====
	for _, room := range inspection.Rooms {
		f.AddPage()
		drawRoomDefects(f, &room)
	}

	// ===== Последняя страница: Подписи =====
	f.AddPage()
	drawSignatures(f, inspection)

	// Сохраняем файл
	filename := fmt.Sprintf("act_%s.pdf", inspection.ActNumber)
	outPath := filepath.Join(outputDir, filename)
	if err := f.OutputFileAndClose(outPath); err != nil {
		return "", fmt.Errorf("ошибка сохранения PDF: %w", err)
	}

	return outPath, nil
}

func drawMeasurementsTable(f *fpdf.Fpdf, rooms []models.InspectionRoom) {
	if len(rooms) == 0 {
		return
	}
	setFont(f, "B", 9)
	headers := []string{"№", "Помещение", "Дл.", "Шир.", "Выс.", "О1 выс", "О1 шир", "О2 выс", "О2 шир", "Д выс", "Д шир"}
	widths := []float64{8, 40, 14, 14, 14, 14, 14, 14, 14, 14, 14}

	for i, h := range headers {
		f.CellFormat(widths[i], 6, h, "1", 0, "C", false, 0, "")
	}
	f.Ln(-1)

	setFont(f, "", 9)
	for _, room := range rooms {
		row := []string{
			strconv.Itoa(room.RoomNumber),
			room.RoomName,
			fmtFloat(room.Length),
			fmtFloat(room.Width),
			fmtFloat(room.Height),
			fmtFloat(room.Window1Height),
			fmtFloat(room.Window1Width),
			fmtFloat(room.Window2Height),
			fmtFloat(room.Window2Width),
			fmtFloat(room.DoorHeight),
			fmtFloat(room.DoorWidth),
		}
		for i, cell := range row {
			f.CellFormat(widths[i], 6, cell, "1", 0, "C", false, 0, "")
		}
		f.Ln(-1)
	}
	f.Ln(4)
}

func drawRoomDefects(f *fpdf.Fpdf, room *models.InspectionRoom) {
	// Заголовок помещения
	setFont(f, "B", 12)
	name := fmt.Sprintf("Помещение %d", room.RoomNumber)
	if room.RoomName != "" {
		name += " — " + room.RoomName
	}
	f.SetFillColor(67, 97, 238)
	f.SetTextColor(255, 255, 255)
	f.CellFormat(contentW, 8, name, "", 1, "L", true, 0, "")
	f.SetTextColor(0, 0, 0)
	f.Ln(3)

	// Группируем дефекты по секции
	bySection := make(map[string][]models.RoomDefect)
	for _, d := range room.Defects {
		bySection[d.Section] = append(bySection[d.Section], d)
	}

	sections := []struct {
		key   string
		label string
	}{
		{"window", "Окна (тип: " + windowTypeName(room.WindowType) + ")"},
		{"ceiling", "Потолок"},
		{"wall", "Стены (тип: " + wallTypeName(room.WallType) + ")"},
		{"floor", "Пол"},
		{"door", "Двери"},
		{"plumbing", "Сантехника"},
	}

	for _, sec := range sections {
		defects := bySection[sec.key]
		// Пропускаем пустые секции
		hasData := false
		for _, d := range defects {
			if d.Value != "" || d.Notes != "" {
				hasData = true
				break
			}
		}
		if !hasData {
			continue
		}

		setFont(f, "B", 10)
		f.SetFillColor(240, 243, 255)
		f.CellFormat(contentW, 6, sec.label, "LR", 1, "L", true, 0, "")

		if sec.key == "wall" {
			drawWallDefects(f, defects)
		} else {
			drawSimpleDefects(f, defects)
		}
	}
}

func drawSimpleDefects(f *fpdf.Fpdf, defects []models.RoomDefect) {
	setFont(f, "", 9)
	for _, d := range defects {
		if d.Notes != "" {
			f.SetFillColor(255, 253, 240)
			f.CellFormat(contentW*0.7, 5.5, "Прочее: "+d.Notes, "LRB", 1, "L", true, 0, "")
			continue
		}
		if d.Value == "" {
			continue
		}
		name := ""
		if d.DefectTemplate.Name != "" {
			name = d.DefectTemplate.Name
		}
		f.CellFormat(contentW*0.7, 5.5, name, "LB", 0, "L", false, 0, "")
		f.CellFormat(contentW*0.3, 5.5, d.Value, "RB", 1, "C", false, 0, "")
	}
}

func drawWallDefects(f *fpdf.Fpdf, defects []models.RoomDefect) {
	// Группируем по templateID
	type wallEntry struct {
		name   string
		values [5]string // индекс 1-4
	}
	entries := make(map[uint]*wallEntry)
	order := []uint{}

	for _, d := range defects {
		if d.Notes != "" {
			setFont(f, "", 9)
			f.CellFormat(contentW*0.7, 5.5, "Прочее: "+d.Notes, "LRB", 1, "L", false, 0, "")
			continue
		}
		if d.Value == "" || d.WallNumber < 1 || d.WallNumber > 4 {
			continue
		}
		if _, ok := entries[d.DefectTemplateID]; !ok {
			entries[d.DefectTemplateID] = &wallEntry{name: d.DefectTemplate.Name}
			order = append(order, d.DefectTemplateID)
		}
		entries[d.DefectTemplateID].values[d.WallNumber] = d.Value
	}

	if len(order) == 0 {
		return
	}

	setFont(f, "B", 8)
	colW := contentW / 5
	f.CellFormat(colW*2, 5, "Дефект", "1", 0, "C", false, 0, "")
	for _, w := range []string{"Ст 1", "Ст 2", "Ст 3", "Ст 4"} {
		f.CellFormat(colW*0.75, 5, w, "1", 0, "C", false, 0, "")
	}
	f.Ln(-1)

	setFont(f, "", 8)
	for _, tid := range order {
		e := entries[tid]
		f.CellFormat(colW*2, 5, e.name, "1", 0, "L", false, 0, "")
		for w := 1; w <= 4; w++ {
			f.CellFormat(colW*0.75, 5, e.values[w], "1", 0, "C", false, 0, "")
		}
		f.Ln(-1)
	}
}

func drawSignatures(f *fpdf.Fpdf, inspection *models.Inspection) {
	f.Ln(20)
	setFont(f, "B", 11)
	f.CellFormat(contentW, 7, "Подписи сторон", "", 1, "C", false, 0, "")
	f.Ln(10)

	sigLine := func(role, name string) {
		setFont(f, "", 10)
		f.CellFormat(50, 6, role, "", 0, "L", false, 0, "")
		f.CellFormat(60, 6, "", "B", 0, "C", false, 0, "") // линия подписи
		f.CellFormat(10, 6, "", "", 0, "C", false, 0, "")
		f.CellFormat(60, 6, name, "B", 1, "C", false, 0, "")
		setFont(f, "", 8)
		f.CellFormat(50, 5, "", "", 0, "", false, 0, "")
		f.CellFormat(60, 5, "(подпись)", "", 0, "C", false, 0, "")
		f.CellFormat(10, 5, "", "", 0, "", false, 0, "")
		f.CellFormat(60, 5, "ФИО", "", 1, "C", false, 0, "")
		f.Ln(8)
	}

	sigLine("Осмотр проводил:", inspection.User.Initials)
	sigLine("Собственник:", inspection.OwnerName)
	sigLine("Представитель застройщика:", inspection.DeveloperRepName)
}

// ===== Вспомогательные функции =====

func setFont(f *fpdf.Fpdf, style string, size float64) {
	if _, err := os.Stat(fontPath); err == nil {
		f.SetFont("Arial", style, size)
	} else {
		f.SetFont("Helvetica", style, size)
	}
}

func row2col(f *fpdf.Fpdf, l1, v1, l2, v2 string) {
	half := contentW / 2
	setFont(f, "B", 10)
	f.CellFormat(30, 6, l1, "", 0, "L", false, 0, "")
	setFont(f, "", 10)
	f.CellFormat(half-30, 6, v1, "B", 0, "L", false, 0, "")
	setFont(f, "B", 10)
	f.CellFormat(30, 6, l2, "", 0, "L", false, 0, "")
	setFont(f, "", 10)
	f.CellFormat(half-30, 6, v2, "B", 1, "L", false, 0, "")
	f.Ln(2)
}

func row4col(f *fpdf.Fpdf, l1, v1, l2, v2, l3, v3 string) {
	third := contentW / 3
	pairs := [][2]string{{l1, v1}, {l2, v2}, {l3, v3}}
	for _, p := range pairs {
		setFont(f, "B", 10)
		f.CellFormat(20, 6, p[0], "", 0, "L", false, 0, "")
		setFont(f, "", 10)
		f.CellFormat(third-20, 6, p[1], "B", 0, "L", false, 0, "")
	}
	f.Ln(4)
}

func labelValue(f *fpdf.Fpdf, label, value string) {
	setFont(f, "B", 10)
	f.CellFormat(25, 6, label, "", 0, "L", false, 0, "")
	setFont(f, "", 10)
	f.CellFormat(contentW-25, 6, value, "B", 1, "L", false, 0, "")
	f.Ln(2)
}

func fmtFloat(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func windowTypeName(t string) string {
	switch t {
	case "pvc":
		return "ПВХ"
	case "al":
		return "Al"
	case "wood":
		return "Дерево"
	}
	return "—"
}

func wallTypeName(t string) string {
	switch t {
	case "paint":
		return "Окраска"
	case "tile":
		return "Плитка"
	case "gkl":
		return "ГКЛ"
	}
	return "—"
}
