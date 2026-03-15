package pdf

import (
	"bytes"
	"embed"
	"fmt"
	"inspection-app/internal/models"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
)

//go:embed fonts
var fontFS embed.FS

const (
	pageW    = 210.0
	pageH    = 297.0
	marginL  = 15.0
	marginR  = 15.0
	marginT  = 15.0
	marginB  = 15.0
	contentW = pageW - marginL - marginR
)

var fontCandidates = []string{
	// Windows
	"C:/Windows/Fonts/arial.ttf",
	// Linux — Liberation Sans (apt install fonts-liberation)
	"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
	// Linux — DejaVu (часто предустановлен)
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans.ttf",
}

var fontBoldCandidates = []string{
	"C:/Windows/Fonts/arialbd.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/liberation/LiberationSans-Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
}

func findFont(candidates []string) string {
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Generate создаёт PDF для акта осмотра и возвращает путь к файлу
func Generate(inspection *models.Inspection, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать директорию: %w", err)
	}

	f := fpdf.New("P", "mm", "A4", "")
	f.SetMargins(marginL, marginT, marginR)
	f.SetAutoPageBreak(true, marginB)
	f.AliasNbPages("{nb}")

	// Подключаем шрифт с кириллицей: сначала embedded (go:embed), потом системные пути
	regularBytes, errR := fontFS.ReadFile("fonts/font.ttf")
	boldBytes, errB := fontFS.ReadFile("fonts/font_bold.ttf")
	if errR == nil && errB == nil {
		f.AddUTF8FontFromBytes("Arial", "", regularBytes)
		f.AddUTF8FontFromBytes("Arial", "B", boldBytes)
	} else if fp := findFont(fontCandidates); fp != "" {
		if fb := findFont(fontBoldCandidates); fb != "" {
			f.AddUTF8Font("Arial", "", fp)
			f.AddUTF8Font("Arial", "B", fb)
		}
	}

	// Колонтитул с номером страницы
	f.SetFooterFuncLpi(func(lastPage bool) {
		f.SetY(-10)
		setFont(f, "", 8)
		f.CellFormat(contentW, 5,
			fmt.Sprintf("Страница %d из {nb}", f.PageNo()),
			"", 0, "R", false, 0, "")
	})

	// ===== Страница 1: Шапка акта =====
	f.AddPage()

	setFont(f, "B", 12)
	f.CellFormat(contentW, 8, "Акт осмотра объекта №"+inspection.ActNumber, "B", 1, "C", false, 0, "")
	f.Ln(3)

	row2col(f, "Дата обследования:", inspection.Date.Format("02.01.2006"),
		"Время обследования:", inspection.InspectionTime)
	labelValue(f, "Адрес:", inspection.Address)
	f.Ln(1)
	row4col(f,
		"Кол-во комнат:", strconv.Itoa(inspection.RoomsCount),
		"Этаж:", strconv.Itoa(inspection.Floor),
		"Общая площадь:", fmtFloat(inspection.TotalArea)+" м²",
	)
	row4col(f,
		"t наружн.=", fmtTempOutside(inspection.TempOutside),
		"t внутр.=", fmtTempInside(inspection.TempInside),
		"RH=", fmtHumidity(inspection.Humidity),
	)

	// Высота таблицы замеров: заголовок(6) + строк*6 + отступ(4)
	// При 4+ окнах — две таблицы
	measTableH := 0.0
	if hasAnyMeasurements(inspection.Rooms) {
		oneTableH := 10.0 + float64(len(inspection.Rooms))*6.0
		if maxWindowsUsed(inspection.Rooms) >= 4 {
			measTableH = oneTableH*2 + 8
		} else {
			measTableH = oneTableH
		}
	}
	// Нижняя граница первой страницы (с учётом колонтитула)
	pg1Bottom := pageH - marginB - 10.0

	// План помещений — на первой странице, пропорционально (без растяжки)
	hasContent := false
	if inspection.PlanImage != "" {
		imgPath := "web/static/uploads/" + filepath.Base(inspection.PlanImage)
		if _, err := os.Stat(imgPath); err == nil {
			f.Ln(3)
			setFont(f, "B", 11)
			f.CellFormat(contentW, 7, "ПЛАН ПОМЕЩЕНИЙ", "", 1, "C", false, 0, "")
			f.Ln(2)

			// Получаем размеры изображения для пропорционального масштабирования
			info := f.RegisterImageOptions(imgPath, fpdf.ImageOptions{})
			if info != nil {
				iW, iH := info.Extent()
				if iW > 0 && iH > 0 {
					// Масштабируем под ширину страницы, сохраняем пропорции
					drawW := contentW
					drawH := iH * (drawW / iW)
					// Ограничиваем высоту так, чтобы ниже поместилась таблица замеров
					maxH := pg1Bottom - measTableH - f.GetY() - 5
					if maxH < 30 {
						maxH = 30
					}
					if drawH > maxH {
						scale := maxH / drawH
						drawH = maxH
						drawW = drawW * scale
					}
					xOff := (contentW - drawW) / 2
					f.ImageOptions(imgPath, marginL+xOff, f.GetY(), drawW, drawH, false, fpdf.ImageOptions{}, 0, "")
					f.SetY(f.GetY() + drawH + 3)
				}
			}
			hasContent = true
		}
	}

	// Таблица замеров — прикреплена к низу первой страницы (как подписи)
	if hasAnyMeasurements(inspection.Rooms) {
		if !hasContent {
			f.Ln(5)
		}
		// Опускаем позицию к низу страницы
		f.SetY(pg1Bottom - measTableH)
		drawMeasurementsTable(f, inspection.Rooms)
		hasContent = true
	}

	// ===== Дефекты по помещениям — с новой страницы (если на стр.1 был план/замеры)
	if hasContent {
		f.AddPage()
	} else {
		f.Ln(6) // отступ между шапкой и первым помещением
	}
	firstRoom := true
	for i := range inspection.Rooms {
		room := &inspection.Rooms[i]
		if !roomHasAnyDefects(room) {
			continue
		}
		if !firstRoom {
			f.Ln(4)
		}
		// Если до конца страницы меньше 28mm — начать новую страницу
		if f.GetY() > pageH-marginB-28 {
			f.AddPage()
		}
		drawRoomDefects(f, room)
		firstRoom = false
	}

	// ===== QR-код с ссылкой на фотоматериалы =====
	if inspection.PhotoFolderURL != "" {
		const qrH = 33.0 // высота блока с QR (подпись + изображение + отступ)
		if f.GetY()+qrH > pageH-marginB-10 {
			f.AddPage()
		}
		addQRCode(f, inspection.PhotoFolderURL)
	}

	// ===== Подписи сторон — внизу последней страницы =====
	const sigH = 56.0
	// Нижняя граница области контента (с отступом для колонтитула)
	bottomLine := pageH - marginB - 10.0
	if f.GetY()+sigH > bottomLine {
		f.AddPage()
	}
	f.SetY(bottomLine - sigH)
	drawSignatures(f, inspection)

	// Сохраняем файл
	filename := fmt.Sprintf("act_%s.pdf", inspection.ActNumber)
	outPath := filepath.Join(outputDir, filename)
	if err := f.OutputFileAndClose(outPath); err != nil {
		return "", fmt.Errorf("ошибка сохранения PDF: %w", err)
	}

	return outPath, nil
}

func hasAnyMeasurements(rooms []models.InspectionRoom) bool {
	for _, r := range rooms {
		if r.Length > 0 || r.Width > 0 || r.Height > 0 ||
			r.Window1Height > 0 || r.Window1Width > 0 ||
			r.Window2Height > 0 || r.Window3Height > 0 ||
			r.Window4Height > 0 || r.Window5Height > 0 ||
			r.DoorHeight > 0 || r.DoorWidth > 0 {
			return true
		}
	}
	return false
}

// maxWindowsUsed returns the highest window number (1-5) that has data across all rooms.
func maxWindowsUsed(rooms []models.InspectionRoom) int {
	max := 1
	for _, r := range rooms {
		if r.Window5Height > 0 || r.Window5Width > 0 {
			return 5
		}
		if r.Window4Height > 0 || r.Window4Width > 0 {
			if 4 > max {
				max = 4
			}
		} else if r.Window3Height > 0 || r.Window3Width > 0 {
			if 3 > max {
				max = 3
			}
		} else if r.Window2Height > 0 || r.Window2Width > 0 {
			if 2 > max {
				max = 2
			}
		}
	}
	return max
}

func roomHasAnyDefects(room *models.InspectionRoom) bool {
	for _, d := range room.Defects {
		if d.Value != "" || d.Notes != "" {
			return true
		}
	}
	return false
}

func drawMeasurementsTable(f *fpdf.Fpdf, rooms []models.InspectionRoom) {
	if len(rooms) == 0 {
		return
	}

	numWin := maxWindowsUsed(rooms)
	const rowH = 6.0

	windowVals := func(r models.InspectionRoom) []float64 {
		return []float64{
			r.Window1Height, r.Window1Width,
			r.Window2Height, r.Window2Width,
			r.Window3Height, r.Window3Width,
			r.Window4Height, r.Window4Width,
			r.Window5Height, r.Window5Width,
		}
	}

	// drawTwoLineHeader рисует заголовок: если в названии есть пробел — две строки,
	// иначе — одна строка, центрированная по вертикали.
	drawTwoLineHeader := func(hdrs []string, wids []float64, hdrH float64, fs float64) {
		startY := f.GetY()
		// Проход 1: рамки
		xCur := marginL
		for i := range hdrs {
			f.SetXY(xCur, startY)
			f.CellFormat(wids[i], hdrH, "", "1", 0, "C", false, 0, "")
			xCur += wids[i]
		}
		// Проход 2: текст
		setFont(f, "B", fs)
		xCur = marginL
		for i, h := range hdrs {
			if idx := strings.Index(h, " "); idx >= 0 {
				// Два слова — разбиваем на две строки
				f.SetXY(xCur, startY)
				f.CellFormat(wids[i], hdrH/2, h[:idx], "", 0, "C", false, 0, "")
				f.SetXY(xCur, startY+hdrH/2)
				f.CellFormat(wids[i], hdrH/2, h[idx+1:], "", 0, "C", false, 0, "")
			} else {
				// Одно слово — по центру вертикально
				f.SetXY(xCur, startY+(hdrH-rowH)/2)
				f.CellFormat(wids[i], rowH, h, "", 0, "C", false, 0, "")
			}
			xCur += wids[i]
		}
		f.SetXY(marginL, startY+hdrH)
	}

	if numWin >= 4 {
		// === Таблица 1: основные размеры (без окон) ===
		// №(8) + Помещение(35 фикс.) + 5 колонок данных (остаток поровну)
		const mainNameW = 35.0
		mainDataColW := (contentW - 8.0 - mainNameW) / 5.0
		// Заголовки: слова с пробелом → двухстрочные
		mainHeaders := []string{"№", "Помещение", "Длина", "Ширина", "Высота", "Дверь высота", "Дверь ширина"}
		mainWidths := []float64{8, mainNameW, mainDataColW, mainDataColW, mainDataColW, mainDataColW, mainDataColW}

		drawTwoLineHeader(mainHeaders, mainWidths, 12.0, 9)

		setFont(f, "", 9)
		for _, room := range rooms {
			// Перенос строки в названии помещения если не влезает
			nameLines := wrapText(f, room.RoomName, mainNameW)
			nRows := len(nameLines)
			if nRows < 1 {
				nRows = 1
			}
			rH := float64(nRows) * rowH
			startY := f.GetY()
			xCur := marginL
			// Рамки строки
			allCols := append([]float64{}, mainWidths...)
			for _, w := range allCols {
				f.SetXY(xCur, startY)
				f.CellFormat(w, rH, "", "1", 0, "C", false, 0, "")
				xCur += w
			}
			// Текст: № и данные
			vals := []string{
				strconv.Itoa(room.RoomNumber), "",
				fmtFloat(room.Length), fmtFloat(room.Width), fmtFloat(room.Height),
				fmtFloat(room.DoorHeight), fmtFloat(room.DoorWidth),
			}
			xCur = marginL
			for i, v := range vals {
				if i == 1 {
					// Название помещения — с переносом строк
					for li, line := range nameLines {
						f.SetXY(xCur, startY+float64(li)*rowH)
						f.CellFormat(mainWidths[i], rowH, line, "", 0, "C", false, 0, "")
					}
				} else {
					f.SetXY(xCur, startY+(rH-rowH)/2)
					f.CellFormat(mainWidths[i], rowH, v, "", 0, "C", false, 0, "")
				}
				xCur += mainWidths[i]
			}
			f.SetXY(marginL, startY+rH)
		}
		f.Ln(5)

		// === Таблица 2: размеры окон ===
		// №(8) + Помещение(35 фикс.) + numWin*2 колонок (остаток поровну)
		const winNameW = 35.0
		winColW := (contentW - 8.0 - winNameW) / float64(numWin*2)
		winFs := 9.0
		if numWin >= 5 {
			winFs = 7.5
		} else {
			winFs = 8.0
		}

		winHeaders := []string{"№", "Помещение"}
		winWidths := []float64{8, winNameW}
		for i := 1; i <= numWin; i++ {
			winHeaders = append(winHeaders, fmt.Sprintf("Ок-%d выс", i))
			winHeaders = append(winHeaders, fmt.Sprintf("Ок-%d шир", i))
			winWidths = append(winWidths, winColW)
			winWidths = append(winWidths, winColW)
		}
		drawTwoLineHeader(winHeaders, winWidths, 12.0, winFs)

		setFont(f, "", winFs)
		for _, room := range rooms {
			wins := windowVals(room)
			// Пропускаем помещения без размеров окон
			hasWin := false
			for _, v := range wins {
				if v > 0 {
					hasWin = true
					break
				}
			}
			if !hasWin {
				continue
			}
			row := []string{strconv.Itoa(room.RoomNumber), room.RoomName}
			for i := 0; i < numWin*2; i++ {
				row = append(row, fmtFloat(wins[i]))
			}
			for i, cell := range row {
				f.CellFormat(winWidths[i], rowH, cell, "1", 0, "C", false, 0, "")
			}
			f.Ln(-1)
		}
		f.Ln(4)

	} else {
		// === Одна таблица (numWin < 4) ===
		// №(8) + Помещение(45 фикс.) + window cols + Д выс(13) + Д шир(13)
		const singleNameW = 45.0
		// Оставшееся место делим между окнами и дверями
		winColW := (contentW - 8.0 - singleNameW - 26.0) / float64(numWin*2)
		if winColW > 16.0 {
			winColW = 16.0
		}

		headers := []string{"№", "Помещение", "Дл.", "Шир.", "Выс."}
		widths := []float64{8, singleNameW, 13, 13, 13}
		for i := 1; i <= numWin; i++ {
			headers = append(headers, fmt.Sprintf("Ок-%d выс", i))
			headers = append(headers, fmt.Sprintf("Ок-%d шир", i))
			widths = append(widths, winColW)
			widths = append(widths, winColW)
		}
		headers = append(headers, "Д выс", "Д шир")
		widths = append(widths, 13, 13)

		setFont(f, "B", 9)
		for i, h := range headers {
			f.CellFormat(widths[i], rowH, h, "1", 0, "C", false, 0, "")
		}
		f.Ln(-1)

		setFont(f, "", 9)
		for _, room := range rooms {
			wins := windowVals(room)
			row := []string{
				strconv.Itoa(room.RoomNumber),
				room.RoomName,
				fmtFloat(room.Length),
				fmtFloat(room.Width),
				fmtFloat(room.Height),
			}
			for i := 0; i < numWin*2; i++ {
				row = append(row, fmtFloat(wins[i]))
			}
			row = append(row, fmtFloat(room.DoorHeight), fmtFloat(room.DoorWidth))
			for i, cell := range row {
				f.CellFormat(widths[i], rowH, cell, "1", 0, "C", false, 0, "")
			}
			f.Ln(-1)
		}
		f.Ln(4)
	}
}

func drawRoomDefects(f *fpdf.Fpdf, room *models.InspectionRoom) {
	// Заголовок помещения
	setFont(f, "B", 11)
	name := fmt.Sprintf("Помещение %d", room.RoomNumber)
	if room.RoomName != "" {
		name += " — " + room.RoomName
	}
	f.SetFillColor(67, 97, 238)
	f.SetTextColor(255, 255, 255)
	f.CellFormat(contentW, 7, name, "", 1, "L", true, 0, "")
	f.SetTextColor(0, 0, 0)
	f.SetFillColor(255, 255, 255)
	f.Ln(2)

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

		setFont(f, "B", 9)
		f.SetFillColor(240, 243, 255)
		f.CellFormat(contentW, 6, sec.label, "LR", 1, "L", true, 0, "")
		f.SetFillColor(255, 255, 255)

		if sec.key == "wall" {
			drawWallDefects(f, defects)
		} else {
			drawSimpleDefects(f, defects)
		}
	}
}

// splitByCommas разбивает строку по запятым — каждая часть на отдельной строке.
// Запятые между цифрами (десятичный разделитель, напр. "0,6") не разбиваются.
func splitByCommas(s string) []string {
	if s == "" {
		return []string{""}
	}
	runes := []rune(s)
	var parts []string
	current := ""
	for i, r := range runes {
		if r == ',' {
			// Не разбиваем, если запятая стоит между цифрами (десятичный разделитель)
			prevIsDigit := i > 0 && runes[i-1] >= '0' && runes[i-1] <= '9'
			nextIsDigit := i+1 < len(runes) && runes[i+1] >= '0' && runes[i+1] <= '9'
			if prevIsDigit && nextIsDigit {
				current += string(r)
				continue
			}
			current += ","
			p := strings.TrimSpace(current)
			if p != "" && p != "," {
				parts = append(parts, p)
			}
			current = ""
		} else {
			current += string(r)
		}
	}
	if p := strings.TrimSpace(current); p != "" {
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return []string{s}
	}
	return parts
}

// splitByWords разбивает строку по пробелам — каждое слово на отдельной строке.
func splitByWords(s string) []string {
	if s == "" {
		return []string{""}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	return words
}

// wrapText разбивает текст на строки, каждая из которых помещается в ширину w.
// Использует GetStringWidth для точного измерения с учётом кириллицы.
// Вычитает 2 мм на внутренние отступы ячейки (cellMargin).
func wrapText(f *fpdf.Fpdf, text string, w float64) []string {
	// 2 мм запас на cellMargin (по 1 мм с каждой стороны)
	availW := w - 2
	if availW < 8 {
		availW = w
	}
	if text == "" {
		return []string{""}
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			if f.GetStringWidth(candidate) <= availW {
				current = candidate
			} else {
				if current != "" {
					lines = append(lines, current)
					current = ""
				}
				// Если одно слово шире колонки — разбиваем по символам
				if f.GetStringWidth(word) > availW {
					runes := []rune(word)
					partial := ""
					for _, r := range runes {
						test := partial + string(r)
						if f.GetStringWidth(test) <= availW {
							partial = test
						} else {
							if partial != "" {
								lines = append(lines, partial)
							}
							partial = string(r)
						}
					}
					current = partial
				} else {
					current = word
				}
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func drawSimpleDefects(f *fpdf.Fpdf, defects []models.RoomDefect) {
	const lineH = 5.5
	const nameW = contentW * 0.70
	const valW = contentW * 0.30

	setFont(f, "", 9)
	for _, d := range defects {
		if d.Notes != "" {
			f.SetFillColor(255, 253, 240)
			f.MultiCell(contentW, lineH, "Прочее: "+d.Notes, "LRB", "L", true)
			f.SetFillColor(255, 255, 255)
			continue
		}
		if d.Value == "" {
			continue
		}
		name := d.DefectTemplate.Name
		val := d.Value
		if d.DefectTemplate.Unit != "" {
			val += d.DefectTemplate.Unit
		}
		if name == "" {
			continue
		}

		// Вычисляем строки для каждой колонки явно, без MultiCell
		nameLines := wrapText(f, name, nameW)
		valLines := wrapText(f, val, valW)
		if len(valLines) == 0 {
			valLines = []string{""}
		}
		maxLines := len(nameLines)
		if len(valLines) > maxLines {
			maxLines = len(valLines)
		}
		rowH := float64(maxLines) * lineH

		if f.GetY()+rowH > pageH-marginB {
			f.AddPage()
		}
		startY := f.GetY()

		// Рисуем каждую строку явно — гарантированное совпадение с rowH
		for i, line := range nameLines {
			f.SetXY(marginL, startY+float64(i)*lineH)
			f.CellFormat(nameW, lineH, line, "", 0, "L", false, 0, "")
		}
		for i, line := range valLines {
			f.SetXY(marginL+nameW, startY+float64(i)*lineH)
			f.CellFormat(valW, lineH, line, "", 0, "C", false, 0, "")
		}

		endY := startY + rowH
		f.Line(marginL, startY, marginL+contentW, startY)
		f.Line(marginL, endY, marginL+contentW, endY)
		f.Line(marginL, startY, marginL, endY)
		f.Line(marginL+nameW, startY, marginL+nameW, endY)
		f.Line(marginL+contentW, startY, marginL+contentW, endY)

		f.SetXY(marginL, endY)
	}
}

func drawWallDefects(f *fpdf.Fpdf, defects []models.RoomDefect) {
	type wallEntry struct {
		name   string
		values [5]string
	}
	entries := make(map[uint]*wallEntry)
	order := []uint{}

	for _, d := range defects {
		if d.Notes != "" {
			setFont(f, "", 9)
			f.MultiCell(contentW, 5.5, "Прочее: "+d.Notes, "LRB", "L", false)
			continue
		}
		if d.Value == "" || d.DefectTemplateID == nil || d.WallNumber < 1 || d.WallNumber > 4 {
			continue
		}
		tid := *d.DefectTemplateID
		if _, ok := entries[tid]; !ok {
			entries[tid] = &wallEntry{name: d.DefectTemplate.Name}
			order = append(order, tid)
		}
		val := d.Value
		if d.DefectTemplate.Unit != "" {
			val += d.DefectTemplate.Unit
		}
		entries[tid].values[d.WallNumber] = val
	}

	if len(order) == 0 {
		return
	}

	const lineH = 5.0
	colW := contentW / 5
	nameW := colW * 2
	wallW := colW * 0.75

	setFont(f, "B", 8)
	f.CellFormat(nameW, lineH, "Дефект", "1", 0, "C", false, 0, "")
	for _, w := range []string{"Ст 1", "Ст 2", "Ст 3", "Ст 4"} {
		f.CellFormat(wallW, lineH, w, "1", 0, "C", false, 0, "")
	}
	f.Ln(-1)

	setFont(f, "", 8)
	for _, tid := range order {
		e := entries[tid]

		// Вычисляем строки для каждой колонки явно
		nameLines := wrapText(f, e.name, nameW)
		maxLines := len(nameLines)
		wallAllLines := [5][]string{}
		for w := 1; w <= 4; w++ {
			lines := wrapText(f, e.values[w], wallW)
			if len(lines) == 0 {
				lines = []string{""}
			}
			wallAllLines[w] = lines
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
		}
		rowH := float64(maxLines) * lineH

		if f.GetY()+rowH > pageH-marginB {
			f.AddPage()
		}
		startY := f.GetY()

		// Рисуем каждую строку явно
		for i, line := range nameLines {
			f.SetXY(marginL, startY+float64(i)*lineH)
			f.CellFormat(nameW, lineH, line, "", 0, "L", false, 0, "")
		}
		for w := 1; w <= 4; w++ {
			colX := marginL + nameW + float64(w-1)*wallW
			for i, line := range wallAllLines[w] {
				f.SetXY(colX, startY+float64(i)*lineH)
				f.CellFormat(wallW, lineH, line, "", 0, "C", false, 0, "")
			}
		}

		endY := startY + rowH
		totalW := nameW + 4*wallW
		f.Line(marginL, startY, marginL+totalW, startY)
		f.Line(marginL, endY, marginL+totalW, endY)
		f.Line(marginL, startY, marginL, endY)
		f.Line(marginL+totalW, startY, marginL+totalW, endY)
		f.Line(marginL+nameW, startY, marginL+nameW, endY)
		for w := 1; w <= 3; w++ {
			sepX := marginL + nameW + float64(w)*wallW
			f.Line(sepX, startY, sepX, endY)
		}

		f.SetXY(marginL, endY)
	}
}

func drawSignatures(f *fpdf.Fpdf, inspection *models.Inspection) {
	setFont(f, "B", 10)
	f.CellFormat(contentW, 6, "Подписи сторон", "", 1, "C", false, 0, "")
	f.Ln(5)

	sigLine := func(role, name string) {
		setFont(f, "", 9)
		f.CellFormat(50, 5, role, "", 0, "L", false, 0, "")
		f.CellFormat(55, 5, "", "B", 0, "C", false, 0, "")
		f.CellFormat(5, 5, "", "", 0, "C", false, 0, "")
		f.CellFormat(70, 5, name, "B", 1, "C", false, 0, "")
		setFont(f, "", 7)
		f.CellFormat(50, 4, "", "", 0, "", false, 0, "")
		f.CellFormat(55, 4, "(подпись)", "", 0, "C", false, 0, "")
		f.CellFormat(5, 4, "", "", 0, "", false, 0, "")
		f.CellFormat(70, 4, "ФИО", "", 1, "C", false, 0, "")
		f.Ln(5)
	}

	sigLine("Осмотр проводил:", inspection.User.Initials)
	sigLine("Собственник:", inspection.OwnerName)
	sigLine("Представитель застройщика:", inspection.DeveloperRepName)
}

// ===== Вспомогательные функции =====

func setFont(f *fpdf.Fpdf, style string, size float64) {
	_, errR := fontFS.ReadFile("fonts/font.ttf")
	if errR == nil || findFont(fontCandidates) != "" {
		f.SetFont("Arial", style, size)
	} else {
		f.SetFont("Helvetica", style, size)
	}
}

func row2col(f *fpdf.Fpdf, l1, v1, l2, v2 string) {
	half := contentW / 2 // 90mm
	setFont(f, "B", 9)
	lw1 := f.GetStringWidth(l1) + 2
	f.CellFormat(lw1, 6, l1, "", 0, "L", false, 0, "")
	setFont(f, "", 9)
	f.CellFormat(half-lw1, 6, v1, "B", 0, "L", false, 0, "")
	setFont(f, "B", 9)
	lw2 := f.GetStringWidth(l2) + 2
	f.CellFormat(lw2, 6, l2, "", 0, "L", false, 0, "")
	setFont(f, "", 9)
	f.CellFormat(half-lw2, 6, v2, "B", 1, "L", false, 0, "")
	f.Ln(2)
}

func row4col(f *fpdf.Fpdf, l1, v1, l2, v2, l3, v3 string) {
	third := contentW / 3 // 60mm
	pairs := [][2]string{{l1, v1}, {l2, v2}, {l3, v3}}
	for _, p := range pairs {
		setFont(f, "B", 9)
		lw := f.GetStringWidth(p[0]) + 2 // естественная ширина + отступ
		if lw > third-10 {
			lw = third - 10
		}
		f.CellFormat(lw, 6, p[0], "", 0, "L", false, 0, "")
		setFont(f, "", 9)
		f.CellFormat(third-lw, 6, p[1], "B", 0, "L", false, 0, "")
	}
	f.Ln(-1) // перейти вниз на высоту ячейки (6мм от верха строки)
	f.Ln(2)  // + 2мм отступ
}

func labelValue(f *fpdf.Fpdf, label, value string) {
	setFont(f, "B", 9)
	f.CellFormat(25, 6, label, "", 0, "L", false, 0, "")
	setFont(f, "", 9)
	f.CellFormat(contentW-25, 6, value, "B", 1, "L", false, 0, "")
	f.Ln(2)
}

func fmtFloat(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func fmtTempOutside(v float64) string {
	if v == 0 {
		return ""
	}
	abs := v
	if abs < 0 {
		abs = -abs
	}
	return "-" + strconv.FormatFloat(abs, 'f', -1, 64) + "°C"
}

func fmtTempInside(v float64) string {
	if v == 0 {
		return ""
	}
	abs := v
	if abs < 0 {
		abs = -abs
	}
	return "+" + strconv.FormatFloat(abs, 'f', -1, 64) + "°C"
}

func fmtHumidity(v float64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 1, 64) + "%"
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
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, "/")
}

// addQRCode вставляет QR-код со ссылкой на фотоматериалы осмотра.
func addQRCode(f *fpdf.Fpdf, url string) {
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return
	}

	setFont(f, "", 8)
	f.CellFormat(contentW, 5, "Фотоматериалы осмотра:", "", 1, "C", false, 0, "")

	const qrSize = 25.0
	f.RegisterImageReader("qr_photos", "PNG", bytes.NewReader(png))
	x := marginL + (contentW-qrSize)/2
	f.Image("qr_photos", x, f.GetY(), qrSize, qrSize, false, "PNG", 0, "")
	f.SetY(f.GetY() + qrSize + 3)
}
