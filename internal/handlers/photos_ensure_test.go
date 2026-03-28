package handlers

import (
	"fmt"
	"inspection-app/internal/models"
	"inspection-app/internal/storage"
	"strings"
	"testing"
	"time"
)

// --- EnsureInspectionFolder ---

// TestEnsureInspectionFolder_NoCloud проверяет что без cloudStore функция возвращает ("", nil).
func TestEnsureInspectionFolder_NoCloud(t *testing.T) {
	cloudStore = nil
	url, err := EnsureInspectionFolder(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty url, got %q", url)
	}
}

// TestEnsureInspectionFolder_CreateNew проверяет создание папки по ActNumber для нового осмотра.
func TestEnsureInspectionFolder_CreateNew(t *testing.T) {
	setupTestDB(t)

	mock := &mockCloudStore{
		publishFolURL: "https://disk.yandex.ru/d/newfolder",
		// FolderExists возвращает false для всех путей (папок нет)
		folderExistsMap: map[string]bool{},
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	user := newUser(t, "ensure1@test.com", "pass1234", "Тест", models.RoleInspector)
	insp := models.Inspection{
		ActNumber: "42-010124",
		UserID:    user.ID,
		Date:      time.Now(),
		Status:    "draft",
	}
	storage.DB.Create(&insp)

	url, err := EnsureInspectionFolder(insp.ID)
	if err != nil {
		t.Fatalf("EnsureInspectionFolder: %v", err)
	}
	if url != "https://disk.yandex.ru/d/newfolder" {
		t.Errorf("got url %q", url)
	}

	// Проверяем что EnsurePath вызвалась с ActNumber-путём
	found := false
	for _, p := range mock.ensuredPaths {
		if strings.Contains(p, "42-010124") {
			found = true
		}
	}
	if !found {
		t.Errorf("EnsurePath не вызвана с ActNumber путём; ensuredPaths: %v", mock.ensuredPaths)
	}

	// PhotoFolderURL должен быть сохранён в БД
	var updated models.Inspection
	storage.DB.First(&updated, insp.ID)
	if updated.PhotoFolderURL != "https://disk.yandex.ru/d/newfolder" {
		t.Errorf("PhotoFolderURL не сохранён в БД, got %q", updated.PhotoFolderURL)
	}
}

// TestEnsureInspectionFolder_AutoMigrate проверяет переименование папки inspections/{ID} → inspections/{ActNumber}.
func TestEnsureInspectionFolder_AutoMigrate(t *testing.T) {
	setupTestDB(t)

	user := newUser(t, "ensure2@test.com", "pass1234", "Тест", models.RoleInspector)
	insp := models.Inspection{
		ActNumber: "18-280326",
		UserID:    user.ID,
		Date:      time.Now(),
		Status:    "draft",
	}
	storage.DB.Create(&insp)

	idFolder := fmt.Sprintf("inspections/%d", insp.ID)
	actFolder := "inspections/18-280326"

	mock := &mockCloudStore{
		publishFolURL: "https://disk.yandex.ru/d/migrated",
		// ID-папка существует, ActNumber-папки нет
		folderExistsMap: map[string]bool{
			idFolder:  true,
			actFolder: false,
		},
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	url, err := EnsureInspectionFolder(insp.ID)
	if err != nil {
		t.Fatalf("EnsureInspectionFolder: %v", err)
	}
	if url != "https://disk.yandex.ru/d/migrated" {
		t.Errorf("got url %q", url)
	}

	// MoveFolder должен был вызваться один раз
	if len(mock.moveFolderCalls) != 1 {
		t.Fatalf("ожидался 1 вызов MoveFolder, got %d: %v", len(mock.moveFolderCalls), mock.moveFolderCalls)
	}
	call := mock.moveFolderCalls[0]
	if !strings.HasSuffix(call.Old, idFolder) {
		t.Errorf("MoveFolder.Old = %q, не заканчивается на %q", call.Old, idFolder)
	}
	if !strings.HasSuffix(call.New, actFolder) {
		t.Errorf("MoveFolder.New = %q, не заканчивается на %q", call.New, actFolder)
	}
}

// TestEnsureInspectionFolder_EarlyReturn проверяет что при уже заполненном PhotoFolderURL
// и отсутствии старой ID-папки функция возвращается без лишних API-вызовов.
func TestEnsureInspectionFolder_EarlyReturn(t *testing.T) {
	setupTestDB(t)

	user := newUser(t, "ensure3@test.com", "pass1234", "Тест", models.RoleInspector)
	insp := models.Inspection{
		ActNumber:      "99-010124",
		UserID:         user.ID,
		Date:           time.Now(),
		Status:         "draft",
		PhotoFolderURL: "https://disk.yandex.ru/d/existing",
	}
	storage.DB.Create(&insp)

	idFolder := fmt.Sprintf("inspections/%d", insp.ID)

	mock := &mockCloudStore{
		// ID-папки нет → ранний выход
		folderExistsMap: map[string]bool{
			idFolder: false,
		},
	}
	cloudStore = mock
	defer func() { cloudStore = nil }()

	url, err := EnsureInspectionFolder(insp.ID)
	if err != nil {
		t.Fatalf("EnsureInspectionFolder: %v", err)
	}
	if url != "https://disk.yandex.ru/d/existing" {
		t.Errorf("got url %q, want existing url", url)
	}

	// EnsurePath не должен вызываться (ранний выход)
	if len(mock.ensuredPaths) != 0 {
		t.Errorf("EnsurePath не должен вызываться при раннем выходе, got: %v", mock.ensuredPaths)
	}
	if len(mock.moveFolderCalls) != 0 {
		t.Errorf("MoveFolder не должен вызываться при раннем выходе")
	}
}

// --- buildUploadTask ---

// TestBuildUploadTask_UsesActNumber проверяет что путь к файлу содержит ActNumber, а не ID.
func TestBuildUploadTask_UsesActNumber(t *testing.T) {
	photo := &models.Photo{FileName: "photo_1.jpg"}
	info := defectInfo{
		RoomName:   "Кухня",
		RoomNumber: 1,
		Section:    "ceiling",
		WallNumber: 0,
		DefectName: "Трещина",
		ActNumber:  "18-280326",
	}

	task := buildUploadTask(photo, info, 1)

	if !strings.Contains(task.relFolder, "18-280326") {
		t.Errorf("relFolder %q не содержит ActNumber '18-280326'", task.relFolder)
	}
	if !strings.Contains(task.relFile, "18-280326") {
		t.Errorf("relFile %q не содержит ActNumber '18-280326'", task.relFile)
	}
	// Убеждаемся что ID осмотра не используется вместо ActNumber
	if strings.Contains(task.relFolder, "/0/") {
		t.Errorf("relFolder содержит числовой ID вместо ActNumber: %q", task.relFolder)
	}
}

// TestBuildUploadTask_FallbackActNumber проверяет fallback когда ActNumber — числовой ID.
func TestBuildUploadTask_FallbackActNumber(t *testing.T) {
	photo := &models.Photo{FileName: "photo_1.jpg"}
	info := defectInfo{
		RoomName:   "Ванная",
		RoomNumber: 2,
		Section:    "floor",
		ActNumber:  "27", // fallback: равен ID
	}

	task := buildUploadTask(photo, info, 1)

	if !strings.Contains(task.relFolder, "27") {
		t.Errorf("relFolder %q не содержит fallback ActNumber '27'", task.relFolder)
	}
}

// TestBuildUploadTask_WallSection проверяет что стены включают номер стены в путь.
func TestBuildUploadTask_WallSection(t *testing.T) {
	photo := &models.Photo{FileName: "photo_1.jpg"}
	info := defectInfo{
		RoomName:   "Спальня",
		RoomNumber: 3,
		Section:    "wall",
		WallNumber: 2,
		DefectName: "Пятно",
		ActNumber:  "5-010124",
	}

	task := buildUploadTask(photo, info, 1)

	if !strings.Contains(task.relFolder, "Стена_2") {
		t.Errorf("relFolder %q должен содержать 'Стена_2'", task.relFolder)
	}
}

// --- buildDefectInfoMap ---

// TestBuildDefectInfoMap_ActNumber проверяет что ActNumber из осмотра попадает в infoMap.
func TestBuildDefectInfoMap_ActNumber(t *testing.T) {
	setupTestDB(t)

	user := newUser(t, "map1@test.com", "pass1234", "Тест", models.RoleInspector)
	insp := models.Inspection{
		ActNumber: "77-310326",
		UserID:    user.ID,
		Date:      time.Now(),
		Status:    "draft",
	}
	storage.DB.Create(&insp)

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Зал"}
	storage.DB.Create(&room)

	tmpl := newDefectTemplate(t, "window", "Царапина")
	defect := models.RoomDefect{RoomID: room.ID, DefectTemplateID: &tmpl.ID, Section: "window", Value: "1 мм"}
	storage.DB.Create(&defect)

	infoMap := buildDefectInfoMap(insp.ID)

	if len(infoMap) != 1 {
		t.Fatalf("ожидался 1 дефект в infoMap, got %d", len(infoMap))
	}
	info := infoMap[defect.ID]
	if info.ActNumber != "77-310326" {
		t.Errorf("ActNumber = %q, want '77-310326'", info.ActNumber)
	}
	if info.RoomName != "Зал" {
		t.Errorf("RoomName = %q, want 'Зал'", info.RoomName)
	}
}

// TestBuildDefectInfoMap_FallbackToID проверяет fallback на ID когда ActNumber пустой.
func TestBuildDefectInfoMap_FallbackToID(t *testing.T) {
	setupTestDB(t)

	user := newUser(t, "map2@test.com", "pass1234", "Тест", models.RoleInspector)
	insp := models.Inspection{
		ActNumber: "", // намеренно пустой
		UserID:    user.ID,
		Date:      time.Now(),
		Status:    "draft",
	}
	storage.DB.Create(&insp)

	room := models.InspectionRoom{InspectionID: insp.ID, RoomNumber: 1, RoomName: "Кухня"}
	storage.DB.Create(&room)

	tmpl := newDefectTemplate(t, "floor", "Скол")
	defect := models.RoomDefect{RoomID: room.ID, DefectTemplateID: &tmpl.ID, Section: "floor", Value: "есть"}
	storage.DB.Create(&defect)

	infoMap := buildDefectInfoMap(insp.ID)

	info := infoMap[defect.ID]
	expectedFallback := fmt.Sprintf("%d", insp.ID)
	if info.ActNumber != expectedFallback {
		t.Errorf("ActNumber = %q, want fallback %q", info.ActNumber, expectedFallback)
	}
}
