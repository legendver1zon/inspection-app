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

func migrateUsers(src, dst *gorm.DB) {
	var records []models.User
	if err := src.Unscoped().Find(&records).Error; err != nil {
		log.Fatalf("Чтение users: %v", err)
	}
	if len(records) == 0 {
		log.Println("users: пусто, пропускаем")
		return
	}
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
	// Обнуляем вложенные ассоциации — они переносятся отдельно
	for i := range records {
		records[i].User = models.User{}
		records[i].Rooms = nil
	}
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
