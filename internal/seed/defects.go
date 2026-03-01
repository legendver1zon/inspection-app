package seed

import (
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"log"
)

// DefectTemplates — все дефекты по секциям
var DefectTemplates = []models.DefectTemplate{
	// Окна (window)
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
	{Section: "window", Name: "Угол наклона слива >100°", Threshold: "", Unit: "", OrderIndex: 14},
	{Section: "window", Name: "Смещение дист. рамок", Threshold: "3", Unit: "мм", OrderIndex: 15},
	{Section: "window", Name: "Царапины на стеклах >10", Threshold: "", Unit: "", OrderIndex: 16},

	// Потолок (ceiling)
	{Section: "ceiling", Name: "Наличие инородных веществ", Threshold: "", Unit: "", OrderIndex: 1},
	{Section: "ceiling", Name: "Следы от инструмента, раковины", Threshold: "", Unit: "", OrderIndex: 2},
	{Section: "ceiling", Name: "Отслоение покрытия, трещины", Threshold: "", Unit: "", OrderIndex: 3},
	{Section: "ceiling", Name: "Полосы, пятна, подтеки, брызги, меление поверхности и исправления", Threshold: "", Unit: "", OrderIndex: 4},

	// Стены — Окраска (wall_paint)
	{Section: "wall_paint", Name: "Наличие инородных веществ", Threshold: "", Unit: "", OrderIndex: 1},
	{Section: "wall_paint", Name: "Отклонение от вертикали", Threshold: "10", Unit: "мм", OrderIndex: 2},
	{Section: "wall_paint", Name: "Отклонение от вертикали конструк. ГКЛ", Threshold: "1", Unit: "мм", OrderIndex: 3},
	{Section: "wall_paint", Name: "Отклонение плитки от вертикали", Threshold: "4", Unit: "мм", OrderIndex: 4},
	{Section: "wall_paint", Name: "Откл. швов от верт. и горизонтали", Threshold: "1,5", Unit: "мм", OrderIndex: 5},
	{Section: "wall_paint", Name: "Полосы, пятна, подтеки, брызги, меление поверхности и исправления", Threshold: "", Unit: "", OrderIndex: 6},
	{Section: "wall_paint", Name: "Отслоение покрытия", Threshold: "", Unit: "", OrderIndex: 7},
	{Section: "wall_paint", Name: "Следы от инструмента <0,3 мм.", Threshold: "", Unit: "", OrderIndex: 8},
	{Section: "wall_paint", Name: "Зыбкость конструкции ГКЛ", Threshold: "", Unit: "", OrderIndex: 9},
	{Section: "wall_paint", Name: "Неровности плоскости плитки", Threshold: "2", Unit: "мм", OrderIndex: 10},
	{Section: "wall_paint", Name: "Откл. ширины шва ±0,5 мм", Threshold: "", Unit: "", OrderIndex: 11},
	{Section: "wall_paint", Name: "Воздушные пузыри, заматины, пятна, загрязнения, доклейки и отсл.", Threshold: "", Unit: "", OrderIndex: 12},
}

// SeedDefects — заполняет справочник дефектов если он пустой
func SeedDefects() {
	var count int64
	storage.DB.Model(&models.DefectTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	if err := storage.DB.Create(&DefectTemplates).Error; err != nil {
		log.Fatalf("Ошибка заполнения справочника дефектов: %v", err)
	}
	log.Printf("Справочник дефектов заполнен: %d записей", len(DefectTemplates))
}
