package seed

import (
	"testing"
)

// Тесты проверяют корректность данных справочника дефектов.
// Не требуют БД — работают с константными данными пакета.

func TestAllSections_Count(t *testing.T) {
	want := 6
	got := len(allSections)
	if got != want {
		t.Errorf("allSections: got %d sections, want %d", got, want)
	}
}

func TestAllSections_SectionNames(t *testing.T) {
	wantSections := []string{"window", "ceiling", "wall", "floor", "door", "plumbing"}
	for i, want := range wantSections {
		if i >= len(allSections) {
			t.Fatalf("section %d missing", i)
		}
		if got := allSections[i].section; got != want {
			t.Errorf("allSections[%d].section = %q, want %q", i, got, want)
		}
	}
}

func TestAllSections_TotalTemplates(t *testing.T) {
	total := 0
	for _, s := range allSections {
		total += len(s.templates)
	}
	want := 48
	if total != want {
		t.Errorf("total templates: got %d, want %d", total, want)
	}
}

func TestAllSections_PerSection(t *testing.T) {
	counts := map[string]int{
		"window":   16,
		"ceiling":  4,
		"wall":     12,
		"floor":    8,
		"door":     6,
		"plumbing": 2,
	}
	for _, s := range allSections {
		want, ok := counts[s.section]
		if !ok {
			t.Errorf("unexpected section: %q", s.section)
			continue
		}
		if got := len(s.templates); got != want {
			t.Errorf("section %q: got %d templates, want %d", s.section, got, want)
		}
	}
}

func TestAllTemplates_HaveNames(t *testing.T) {
	for _, s := range allSections {
		for _, tmpl := range s.templates {
			if tmpl.Name == "" {
				t.Errorf("section %q: template with OrderIndex %d has empty name", s.section, tmpl.OrderIndex)
			}
		}
	}
}

func TestAllTemplates_HaveOrderIndex(t *testing.T) {
	for _, s := range allSections {
		for _, tmpl := range s.templates {
			if tmpl.OrderIndex <= 0 {
				t.Errorf("section %q, template %q: OrderIndex must be > 0, got %d",
					s.section, tmpl.Name, tmpl.OrderIndex)
			}
		}
	}
}

func TestAllTemplates_SectionMatchesParent(t *testing.T) {
	for _, s := range allSections {
		for _, tmpl := range s.templates {
			if tmpl.Section != s.section {
				t.Errorf("template %q: Section field %q != parent section %q",
					tmpl.Name, tmpl.Section, s.section)
			}
		}
	}
}

func TestAllTemplates_OrderIndexUnique(t *testing.T) {
	for _, s := range allSections {
		seen := map[int]bool{}
		for _, tmpl := range s.templates {
			if seen[tmpl.OrderIndex] {
				t.Errorf("section %q: duplicate OrderIndex %d", s.section, tmpl.OrderIndex)
			}
			seen[tmpl.OrderIndex] = true
		}
	}
}

func TestFixUnits_TargetsExistInTemplates(t *testing.T) {
	// Проверяем что имена из fixUnits реально присутствуют в allSections
	fixTargets := []string{
		"Следы от инструмента <0,3 мм.",
		"Откл. ширины шва ±0,5 мм",
		"Царапины на стеклах >10",
	}
	allNames := map[string]bool{}
	for _, s := range allSections {
		for _, tmpl := range s.templates {
			allNames[tmpl.Name] = true
		}
	}
	for _, name := range fixTargets {
		if !allNames[name] {
			t.Errorf("fixUnits target %q not found in allSections", name)
		}
	}
}

func TestFixThresholds_TargetsExistInTemplates(t *testing.T) {
	fixTargets := []string{
		"Царапины на стеклах >10",
	}
	allNames := map[string]bool{}
	for _, s := range allSections {
		for _, tmpl := range s.templates {
			allNames[tmpl.Name] = true
		}
	}
	for _, name := range fixTargets {
		if !allNames[name] {
			t.Errorf("fixThresholds target %q not found in allSections", name)
		}
	}
}

func TestWindowSection_HasExpectedDefects(t *testing.T) {
	var windowSection *sectionSeed
	for i := range allSections {
		if allSections[i].section == "window" {
			windowSection = &allSections[i]
			break
		}
	}
	if windowSection == nil {
		t.Fatal("window section not found")
	}

	// Несколько ключевых дефектов окон
	expected := []string{
		"Отклонение от прямолинейности",
		"Разность длин диагоналей",
		"Царапины на стеклах >10",
		"Угол наклона слива",
	}
	nameSet := map[string]bool{}
	for _, tmpl := range windowSection.templates {
		nameSet[tmpl.Name] = true
	}
	for _, name := range expected {
		if !nameSet[name] {
			t.Errorf("window section missing expected defect: %q", name)
		}
	}
}
