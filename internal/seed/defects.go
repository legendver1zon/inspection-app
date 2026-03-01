package seed

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"log"
)

type sectionSeed struct {
	section   string
	templates []models.DefectTemplate
}

var allSections = []sectionSeed{
	{
		section: "window",
		templates: []models.DefectTemplate{
			{Section: "window", Name: "Отклонение от прямолинейности", Threshold: "1", Unit: "мм", OrderIndex: 1},
			{Section: "window", Name: "Зазоры в угловых и Т-образных соед.", Threshold: "0,5", Unit: "мм", OrderIndex: 2},
			{Section: "window", Name: "Трещины в сварных швах, поджоги", Threshold: "", Unit: "", OrderIndex: 3},
			{Section: "window", Name: "Отклонение от вертикали и горизонтали на 1 м.", Threshold: "", Unit: "", OrderIndex: 4},
			{Section: "window", Name: "Цвет, глянец, дефекты поверхности", Threshold: "", Unit: "", OrderIndex: 5},
			{Section: "window", Name: "Уклон подоконника <1%, горизонт. 0,5%", Threshold: "", Unit: "", OrderIndex: 6},
			{Section: "window", Name: "Выступание герметика внутрь камеры", Threshold: "2", Unit: "мм", OrderIndex: 7},
			{Section: "window", Name: "Загрязнения внутренней поверхности стекол", Threshold: "", Unit: "", OrderIndex: 8},
			{Section: "window", Name: "Перепад лицевых поверхн.", Threshold: "1", Unit: "мм", OrderIndex: 9},
			{Section: "window", Name: "Разность длин диагоналей", Threshold: "3", Unit: "мм", OrderIndex: 10},
			{Section: "window", Name: "Защитная пленка", Threshold: "", Unit: "", OrderIndex: 11},
			{Section: "window", Name: "Угловые перегибы уплотнит.", Threshold: "", Unit: "", OrderIndex: 12},
			{Section: "window", Name: "Зазоры между откосами, подок.", Threshold: "", Unit: "", OrderIndex: 13},
			{Section: "window", Name: "Угол наклона слива", Threshold: ">100", Unit: "°", OrderIndex: 14},
			{Section: "window", Name: "Смещение дист. рамок", Threshold: "3", Unit: "мм", OrderIndex: 15},
			{Section: "window", Name: "Царапины на стеклах >10", Threshold: "", Unit: "", OrderIndex: 16},
		},
	},
	{
		section: "ceiling",
		templates: []models.DefectTemplate{
			{Section: "ceiling", Name: "Наличие инородных веществ", Threshold: "", Unit: "", OrderIndex: 1},
			{Section: "ceiling", Name: "Следы от инструмента, раковины", Threshold: "", Unit: "", OrderIndex: 2},
			{Section: "ceiling", Name: "Отслоение покрытия, трещины", Threshold: "", Unit: "", OrderIndex: 3},
			{Section: "ceiling", Name: "Полосы, пятна, подтеки, брызги, меление поверхности и исправления", Threshold: "", Unit: "", OrderIndex: 4},
		},
	},
	{
		section: "wall",
		templates: []models.DefectTemplate{
			{Section: "wall", Name: "Наличие инородных веществ", Threshold: "", Unit: "", OrderIndex: 1},
			{Section: "wall", Name: "Отклонение от вертикали", Threshold: "10", Unit: "мм", OrderIndex: 2},
			{Section: "wall", Name: "Отклонение от вертикали конструк. ГКЛ", Threshold: "1", Unit: "мм", OrderIndex: 3},
			{Section: "wall", Name: "Отклонение плитки от вертикали", Threshold: "4", Unit: "мм", OrderIndex: 4},
			{Section: "wall", Name: "Откл. швов от верт. и горизонтали", Threshold: "1,5", Unit: "мм", OrderIndex: 5},
			{Section: "wall", Name: "Полосы, пятна, подтеки, брызги, меление поверхности и исправления", Threshold: "", Unit: "", OrderIndex: 6},
			{Section: "wall", Name: "Отслоение покрытия", Threshold: "", Unit: "", OrderIndex: 7},
			{Section: "wall", Name: "Следы от инструмента <0,3 мм.", Threshold: "", Unit: "", OrderIndex: 8},
			{Section: "wall", Name: "Зыбкость конструкции ГКЛ", Threshold: "", Unit: "", OrderIndex: 9},
			{Section: "wall", Name: "Неровности плоскости плитки", Threshold: "2", Unit: "мм", OrderIndex: 10},
			{Section: "wall", Name: "Откл. ширины шва ±0,5 мм", Threshold: "", Unit: "", OrderIndex: 11},
			{Section: "wall", Name: "Воздушные пузыри, заматины, пятна, загрязнения, доклейки и отсл.", Threshold: "", Unit: "", OrderIndex: 12},
		},
	},
	{
		section: "floor",
		templates: []models.DefectTemplate{
			{Section: "floor", Name: "Наличие трещин, отслоений и пыления", Threshold: "", Unit: "", OrderIndex: 1},
			{Section: "floor", Name: "Уступы между смежными изделиями", Threshold: "1", Unit: "мм", OrderIndex: 2},
			{Section: "floor", Name: "Зазоры между плинтус. полом или стенами", Threshold: "", Unit: "", OrderIndex: 3},
			{Section: "floor", Name: "Выбоины, трещины, волны, вздутия, приподнятые кромки", Threshold: "0,15", Unit: "мм", OrderIndex: 4},
			{Section: "floor", Name: "Отклонения от плоскости на 2 м.", Threshold: "", Unit: "", OrderIndex: 5},
			{Section: "floor", Name: "Отклонения от горизон.", Threshold: "0,2", Unit: "%", OrderIndex: 6},
			{Section: "floor", Name: "Зазоры между ламинатом", Threshold: "0,2", Unit: "мм", OrderIndex: 7},
			{Section: "floor", Name: "Цвет покрытия, загрязнения строительные материалы", Threshold: "", Unit: "", OrderIndex: 8},
		},
	},
	{
		section: "door",
		templates: []models.DefectTemplate{
			{Section: "door", Name: "Отклонение от прямолинейности", Threshold: "1", Unit: "мм", OrderIndex: 1},
			{Section: "door", Name: "Отклонение от верт. и гориз. на 1 м.", Threshold: "", Unit: "", OrderIndex: 2},
			{Section: "door", Name: "Трещин, мех. повреждений, ржавчины", Threshold: "", Unit: "", OrderIndex: 3},
			{Section: "door", Name: "Перепад лицевых поверхн.", Threshold: "0,7", Unit: "мм", OrderIndex: 4},
			{Section: "door", Name: "Зазоры в местах неподвижных соедин.", Threshold: "0,3", Unit: "мм", OrderIndex: 5},
			{Section: "door", Name: "Отслоение покрытия", Threshold: "", Unit: "", OrderIndex: 6},
		},
	},
	{
		section: "plumbing",
		templates: []models.DefectTemplate{
			{Section: "plumbing", Name: "Отклонение трубопровода от вертикали", Threshold: "2", Unit: "мм", OrderIndex: 1},
			{Section: "plumbing", Name: "Отклон. радиат. от верт. и гориз.", Threshold: "", Unit: "", OrderIndex: 2},
		},
	},
}

// SeedDefects — заполняет справочник дефектов по секциям (пропускает уже заполненные)
func SeedDefects() {
	total := 0
	for _, s := range allSections {
		var count int64
		storage.DB.Model(&models.DefectTemplate{}).Where("section = ?", s.section).Count(&count)
		if count > 0 {
			continue
		}
		if err := storage.DB.Create(&s.templates).Error; err != nil {
			log.Fatalf("Ошибка заполнения секции %s: %v", s.section, err)
		}
		total += len(s.templates)
	}
	if total > 0 {
		log.Printf("Справочник дефектов: добавлено %d записей", total)
	}
}
