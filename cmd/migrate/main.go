// Утилита одноразовой миграции данных из SQLite в PostgreSQL.
//
// Запуск:
//
//	SQLITE_PATH=inspection.db DATABASE_URL=postgres://... go run ./cmd/migrate
package main

import (
	"inspection-app/internal/models"
	"log"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "inspection.db"
	}
	pgDSN := os.Getenv("DATABASE_URL")
	if pgDSN == "" {
		log.Fatal("DATABASE_URL не задан")
	}

	src, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Не удалось открыть SQLite (%s): %v", sqlitePath, err)
	}
	log.Printf("SQLite открыт: %s", sqlitePath)

	dst, err := gorm.Open(postgres.Open(pgDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к PostgreSQL: %v", err)
	}
	log.Println("PostgreSQL подключён")

	log.Println("Создаём схему в PostgreSQL...")
	if err := dst.AutoMigrate(
		&models.User{},
		&models.Inspection{},
		&models.InspectionRoom{},
		&models.RoomDefect{},
		&models.DefectTemplate{},
		&models.Document{},
		&models.Photo{},
	); err != nil {
		log.Fatalf("AutoMigrate: %v", err)
	}

	// Очищаем PostgreSQL перед вставкой (на случай повторного запуска)
	log.Println("Очищаем таблицы PostgreSQL...")
	dst.Exec("TRUNCATE photos, room_defects, inspection_rooms, documents, inspections, defect_templates, users RESTART IDENTITY CASCADE")

	// Отключаем FK-триггеры на время вставки данных.
	// Нужно, т.к. DefectTemplateID может быть 0 (запись "Прочее"),
	// а GORM создаёт FK-ограничение на эту колонку.
	for _, tbl := range []string{
		"users", "inspections", "inspection_rooms",
		"room_defects", "defect_templates", "documents", "photos",
	} {
		dst.Exec("ALTER TABLE " + tbl + " DISABLE TRIGGER ALL")
	}

	// Порядок важен: сначала родительские таблицы, потом дочерние
	migrateUsers(src, dst)
	migrateDefectTemplates(src, dst)
	migrateInspections(src, dst)
	migrateDocuments(src, dst)
	migrateInspectionRooms(src, dst)
	migrateRoomDefects(src, dst)
	migratePhotos(src, dst)

	// Включаем FK-триггеры обратно
	for _, tbl := range []string{
		"users", "inspections", "inspection_rooms",
		"room_defects", "defect_templates", "documents", "photos",
	} {
		dst.Exec("ALTER TABLE " + tbl + " ENABLE TRIGGER ALL")
	}

	resetSequences(dst)
	log.Println("Миграция завершена успешно!")
}

// sanitizeUTF8 проверяет строку на валидность UTF-8.
// Если невалидна — пробует декодировать из CP1251 (Windows Cyrillic).
// Если и это не помогает — заменяет кривые байты на "?".
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	decoded, err := charmap.Windows1251.NewDecoder().String(s)
	if err == nil && utf8.ValidString(decoded) {
		log.Printf("Конвертирована строка из CP1251: %q -> %q", s, decoded)
		return decoded
	}
	return strings.ToValidUTF8(s, "?")
}

// sanitizeRecord обходит все строковые поля структуры через reflect
// и применяет sanitizeUTF8 к каждому.
func sanitizeRecord(v interface{}) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return
	}
	rv = rv.Elem()
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			sanitizeRecord(rv.Index(i).Addr().Interface())
		}
		return
	}
	if rv.Kind() != reflect.Struct {
		return
	}
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		ft := rt.Field(i)
		if ft.Anonymous {
			sanitizeRecord(f.Addr().Interface())
			continue
		}
		if f.Kind() == reflect.String && f.CanSet() {
			f.SetString(sanitizeUTF8(f.String()))
		}
	}
}

func migrateUsers(src, dst *gorm.DB) {
	var records []models.User
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение users: %v", err)
	}
	if len(records) == 0 {
		log.Println("users: пусто, пропускаем")
		return
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись users: %v", err)
	}
	log.Printf("users: перенесено %d записей", len(records))
}

func migrateDefectTemplates(src, dst *gorm.DB) {
	var records []models.DefectTemplate
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение defect_templates: %v", err)
	}
	if len(records) == 0 {
		log.Println("defect_templates: пусто, пропускаем")
		return
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись defect_templates: %v", err)
	}
	log.Printf("defect_templates: перенесено %d записей", len(records))
}

func migrateInspections(src, dst *gorm.DB) {
	var records []models.Inspection
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение inspections: %v", err)
	}
	if len(records) == 0 {
		log.Println("inspections: пусто, пропускаем")
		return
	}
	for i := range records {
		records[i].User = models.User{}
		records[i].Rooms = nil
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().Omit("User", "Rooms").CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись inspections: %v", err)
	}
	log.Printf("inspections: перенесено %d записей", len(records))
}

func migrateDocuments(src, dst *gorm.DB) {
	var records []models.Document
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение documents: %v", err)
	}
	if len(records) == 0 {
		log.Println("documents: пусто, пропускаем")
		return
	}
	for i := range records {
		records[i].Inspection = models.Inspection{}
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().Omit("Inspection").CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись documents: %v", err)
	}
	log.Printf("documents: перенесено %d записей", len(records))
}

func migrateInspectionRooms(src, dst *gorm.DB) {
	var records []models.InspectionRoom
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение inspection_rooms: %v", err)
	}
	if len(records) == 0 {
		log.Println("inspection_rooms: пусто, пропускаем")
		return
	}
	for i := range records {
		records[i].Defects = nil
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().Omit("Defects").CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись inspection_rooms: %v", err)
	}
	log.Printf("inspection_rooms: перенесено %d записей", len(records))
}

func migrateRoomDefects(src, dst *gorm.DB) {
	var records []models.RoomDefect
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение room_defects: %v", err)
	}
	if len(records) == 0 {
		log.Println("room_defects: пусто, пропускаем")
		return
	}
	for i := range records {
		records[i].DefectTemplate = models.DefectTemplate{}
		records[i].Photos = nil
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().Omit("DefectTemplate", "Photos").CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись room_defects: %v", err)
	}
	log.Printf("room_defects: перенесено %d записей", len(records))
}

func migratePhotos(src, dst *gorm.DB) {
	var records []models.Photo
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение photos: %v", err)
	}
	if len(records) == 0 {
		log.Println("photos: пусто, пропускаем")
		return
	}
	sanitizeRecord(&records)
	if err := dst.Unscoped().CreateInBatches(&records, 100).Error; err != nil {
		log.Fatalf("Запись photos: %v", err)
	}
	log.Printf("photos: перенесено %d записей", len(records))
}

// resetSequences обновляет PostgreSQL-последовательности до MAX(id) каждой таблицы,
// чтобы следующий INSERT получил корректный автоинкрементный ID.
func resetSequences(db *gorm.DB) {
	tables := []string{
		"users", "inspections", "inspection_rooms",
		"room_defects", "defect_templates", "documents", "photos",
	}
	for _, table := range tables {
		sql := `SELECT setval(pg_get_serial_sequence(?, 'id'), COALESCE(MAX(id), 1)) FROM ` + table
		if err := db.Exec(sql, table).Error; err != nil {
			log.Printf("Предупреждение: сброс последовательности %s: %v", table, err)
		} else {
			log.Printf("Последовательность %s сброшена", table)
		}
	}
}
