package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Авторизация ---

func TestGetInspections_Unauthorized(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("GET /inspections без токена: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("redirect: got %q, want /login", loc)
	}
}

func TestGetInspections_InvalidToken(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "invalid.token.value"})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("GET /inspections с невалидным токеном: got %d, want 302", w.Code)
	}
}

// --- Базовый доступ ---

func TestGetInspections_AuthorizedInspector_ReturnsOK(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "inspector@test.com", "pass123", "Иванов Иван Иванович", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /inspections авторизован: got %d, want 200", w.Code)
	}
}

// --- Изоляция данных (инспектор видит только свои) ---

func TestGetInspections_InspectorSeesOnlyOwnInspections(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user1 := newUser(t, "one@test.com", "pass", "Иванов Иван Иванович", models.RoleInspector)
	user2 := newUser(t, "two@test.com", "pass", "Петров Пётр Петрович", models.RoleInspector)

	newInspection(t, user1.ID, "ул. Ленина, 1", "Захаров", "draft", time.Now())
	newInspection(t, user2.ID, "ул. Пушкина, 5", "Кузнецов", "draft", time.Now())

	tok := tokenFor(t, user1.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?tab=draft", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "ул. Ленина, 1") {
		t.Error("инспектор должен видеть свой осмотр (ул. Ленина, 1)")
	}
	if strings.Contains(body, "ул. Пушкина, 5") {
		t.Error("инспектор НЕ должен видеть чужой осмотр (ул. Пушкина, 5)")
	}
}

func TestGetInspections_AdminSeesAll(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass", "Админов Админ Админович", models.RoleAdmin)
	inspector := newUser(t, "insp@test.com", "pass", "Инспектор Иван Иванович", models.RoleInspector)

	newInspection(t, admin.ID, "пр. Мира, 10", "Белов", "draft", time.Now())
	newInspection(t, inspector.ID, "пр. Победы, 3", "Смирнов", "draft", time.Now())

	tok := tokenFor(t, admin.ID, "admin")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?tab=draft", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "пр. Мира, 10") {
		t.Error("admin должен видеть осмотр на пр. Мира, 10")
	}
	if !strings.Contains(body, "пр. Победы, 3") {
		t.Error("admin должен видеть осмотр на пр. Победы, 3")
	}
}

// --- Вкладки draft / completed ---

func TestGetInspections_TabDraft_ShowsOnlyDraft(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "Черновик ул., 1", "Черновиков", "draft", time.Now())
	newInspection(t, user.ID, "Завершённая ул., 2", "Завершёнов", "completed", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?tab=draft", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Черновик ул.") {
		t.Error("вкладка draft: должен показываться черновик")
	}
	if strings.Contains(body, "Завершённая ул.") {
		t.Error("вкладка draft: завершённый осмотр не должен отображаться")
	}
}

func TestGetInspections_TabCompleted_ShowsOnlyCompleted(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "Черновик ул., 1", "Черновиков", "draft", time.Now())
	newInspection(t, user.ID, "Готовая ул., 99", "Готовов", "completed", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?tab=completed", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "Черновик ул.") {
		t.Error("вкладка completed: черновик не должен отображаться")
	}
	if !strings.Contains(body, "Готовая ул.") {
		t.Error("вкладка completed: должен показываться завершённый осмотр")
	}
}

// --- Фильтр по собственнику ---

func TestGetInspections_FilterOwner_Match(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "ул. Цветочная, 5", "Захаров Иван", "draft", time.Now())
	newInspection(t, user.ID, "ул. Солнечная, 8", "Кузнецов Пётр", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?owner=Захаров", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "ул. Цветочная, 5") {
		t.Error("фильтр owner=Захаров: осмотр с Захаровым должен отображаться")
	}
	if strings.Contains(body, "ул. Солнечная, 8") {
		t.Error("фильтр owner=Захаров: осмотр с Кузнецовым не должен отображаться")
	}
}

func TestGetInspections_FilterOwner_CaseInsensitiveLike(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "пр. Советский, 1", "Морозов Сергей", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?owner=Мороз", nil) // Частичное совпадение
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "пр. Советский, 1") {
		t.Error("фильтр owner: должно работать частичное совпадение (LIKE)")
	}
}

func TestGetInspections_FilterOwner_NoMatch(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "ул. Речная, 3", "Волков Андрей", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?owner=Несуществующий", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "ул. Речная, 3") {
		t.Error("фильтр owner без совпадений: осмотр не должен отображаться")
	}
	if !strings.Contains(body, "ничего не найдено") && !strings.Contains(body, "нет") {
		// Хотя бы пустая таблица или пустое состояние
	}
}

// --- Фильтр по адресу ---

func TestGetInspections_FilterAddress_Match(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "ул. Ленина, 42", "Фёдоров", "draft", time.Now())
	newInspection(t, user.ID, "ул. Маркса, 7", "Николаев", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?address=Ленина", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "ул. Ленина, 42") {
		t.Error("фильтр address=Ленина: осмотр на ул. Ленина должен отображаться")
	}
	if strings.Contains(body, "ул. Маркса, 7") {
		t.Error("фильтр address=Ленина: осмотр на ул. Маркса не должен отображаться")
	}
}

func TestGetInspections_FilterAddress_PartialMatch(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "пр. Октябрьский, 15", "Алексеев", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?address=Октябр", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "пр. Октябрьский") {
		t.Error("фильтр address: должно работать частичное совпадение")
	}
}

// --- Фильтр по инспектору (только для admin) ---

func TestGetInspections_FilterInspector_AdminOnly(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	admin := newUser(t, "admin@test.com", "pass", "Админов Админ Админович", models.RoleAdmin)
	insp1 := newUser(t, "ivanov@test.com", "pass", "Иванов Иван Иванович", models.RoleInspector)
	insp2 := newUser(t, "petrov@test.com", "pass", "Петров Пётр Петрович", models.RoleInspector)

	newInspection(t, insp1.ID, "ул. Мира, 1", "Зайцев", "draft", time.Now())
	newInspection(t, insp2.ID, "ул. Труда, 2", "Лисов", "draft", time.Now())

	tok := tokenFor(t, admin.ID, "admin")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?inspector=Иванов", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "ул. Мира, 1") {
		t.Error("фильтр inspector=Иванов: осмотр Иванова должен отображаться")
	}
	if strings.Contains(body, "ул. Труда, 2") {
		t.Error("фильтр inspector=Иванов: осмотр Петрова не должен отображаться")
	}
}

// --- Фильтр по дате ---

func TestGetInspections_FilterDateFrom(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	dateOld := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	dateNew := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	newInspection(t, user.ID, "Старый адрес, 1", "Дедов", "draft", dateOld)
	newInspection(t, user.ID, "Новый адрес, 2", "Юнов", "draft", dateNew)

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?date_from=2026-01-01", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Новый адрес, 2") {
		t.Error("date_from=2026-01-01: осмотр от 2026-03-01 должен отображаться")
	}
	if strings.Contains(body, "Старый адрес, 1") {
		t.Error("date_from=2026-01-01: осмотр от 2025-01-10 не должен отображаться")
	}
}

func TestGetInspections_FilterDateTo(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	dateOld := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	dateNew := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	newInspection(t, user.ID, "Ранний адрес, 5", "Ранов", "draft", dateOld)
	newInspection(t, user.ID, "Поздний адрес, 9", "Поздов", "draft", dateNew)

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?date_to=2025-12-31", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Ранний адрес, 5") {
		t.Error("date_to=2025-12-31: осмотр от 2025-06-15 должен отображаться")
	}
	if strings.Contains(body, "Поздний адрес, 9") {
		t.Error("date_to=2025-12-31: осмотр от 2026-03-10 не должен отображаться")
	}
}

func TestGetInspections_FilterDateRange(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "Февраль адрес", "Февралёв", "draft", time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC))
	newInspection(t, user.ID, "Март адрес", "Мартов", "draft", time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
	newInspection(t, user.ID, "Апрель адрес", "Апрелев", "draft", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC))

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?date_from=2026-02-01&date_to=2026-03-31", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Февраль адрес") {
		t.Error("диапазон дат: осмотр в феврале должен отображаться")
	}
	if !strings.Contains(body, "Март адрес") {
		t.Error("диапазон дат: осмотр в марте должен отображаться")
	}
	if strings.Contains(body, "Апрель адрес") {
		t.Error("диапазон дат: осмотр в апреле не должен отображаться")
	}
}

func TestGetInspections_FilterDateInvalid_Ignored(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "Любой адрес, 1", "Любов", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	// Невалидный формат даты — фильтр должен игнорироваться
	req, _ := http.NewRequest("GET", "/inspections?date_from=not-a-date", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("невалидная дата: got %d, want 200", w.Code)
	}
}

// --- Комбинированные фильтры ---

func TestGetInspections_MultipleFilters(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "ул. Весенняя, 1", "Громов", "draft", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	newInspection(t, user.ID, "ул. Весенняя, 2", "Тихов", "draft", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newInspection(t, user.ID, "ул. Летняя, 3", "Громов", "draft", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?address=Весенняя&owner=Громов&date_from=2026-02-01", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// Только первый осмотр удовлетворяет всем трём фильтрам
	if !strings.Contains(body, "ул. Весенняя, 1") {
		t.Error("комбо-фильтр: осмотр 1 (Весенняя+Громов+март) должен отображаться")
	}
	if strings.Contains(body, "ул. Весенняя, 2") {
		t.Error("комбо-фильтр: осмотр 2 (Тихов, январь) не должен отображаться")
	}
	if strings.Contains(body, "ул. Летняя, 3") {
		t.Error("комбо-фильтр: осмотр 3 (Летняя) не должен отображаться")
	}
}

// --- UI: кнопка «Сбросить» ---

func TestGetInspections_HasFiltersBadge_WithFilter(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?owner=Кто-то", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "Сбросить") {
		t.Error("при активном фильтре должна быть кнопка 'Сбросить'")
	}
}

func TestGetInspections_HasFiltersBadge_WithoutFilter(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	// Без фильтров кнопки "Сбросить" быть не должно
	body := w.Body.String()
	// Убираем из подсчёта ссылку "Сбросить фильтры" в пустом состоянии
	// (она тоже не должна быть, т.к. hasFilters=false)
	if strings.Contains(body, "Сбросить") {
		t.Error("без фильтров кнопка 'Сбросить' не должна отображаться")
	}
}

// --- Счётчики вкладок учитывают фильтры ---

func TestGetInspections_CountsReflectFilters(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	newInspection(t, user.ID, "пр. Садовый, 1", "Цветков", "draft", time.Now())
	newInspection(t, user.ID, "пр. Садовый, 2", "Цветков", "completed", time.Now())
	newInspection(t, user.ID, "пр. Лесной, 3", "Дубов", "draft", time.Now())

	tok := tokenFor(t, user.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections?owner=Цветков", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	body := w.Body.String()
	// При фильтре owner=Цветков: 1 draft и 1 completed
	// Счётчики должны показывать 1, не 2
	if !strings.Contains(body, ">1<") {
		t.Log("body:", body[:500])
		t.Error("счётчик вкладки при фильтре должен показывать 1")
	}
}

// --- GET /inspections/:id — доступ к чужому осмотру ---

func TestGetInspection_ForbiddenForOtherInspector(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	owner := newUser(t, "owner@test.com", "pass", "Хозяев Хозяин Хозяинович", models.RoleInspector)
	other := newUser(t, "other@test.com", "pass", "Чужой Человек Чужович", models.RoleInspector)
	insp := newInspection(t, owner.ID, "Тайный адрес, 1", "Секретов", "draft", time.Now())

	tok := tokenFor(t, other.ID, "inspector")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/inspections/%d", insp.ID), nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("GET /inspections/:id чужой: got %d, want 403", w.Code)
	}
}

func TestGetInspection_NotFound(t *testing.T) {
	setupTestDB(t)
	r := setupRouter(t)

	user := newUser(t, "u@test.com", "pass", "Тестов Тест Тестович", models.RoleInspector)
	tok := tokenFor(t, user.ID, "inspector")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/inspections/99999", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tok})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("GET /inspections/99999: got %d, want 404", w.Code)
	}
}
