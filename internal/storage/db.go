package storage

import (
	"inspection-app/internal/models"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect открывает соединение с PostgreSQL по DSN.
func Connect(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	log.Println("БД подключена:", dsn)
}

// ConnectFromEnv читает DATABASE_URL из переменных окружения и подключается к БД.
func ConnectFromEnv() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL не задан")
	}
	Connect(dsn)
}

func Migrate() {
	// Конвертируем legacy DefectTemplateID=0 в NULL до применения FK-ограничения.
	// Если таблица ещё не существует — ошибка игнорируется.
	DB.Exec("UPDATE room_defects SET defect_template_id = NULL WHERE defect_template_id = 0")

	err := DB.AutoMigrate(
		&models.User{},
		&models.Inspection{},
		&models.InspectionRoom{},
		&models.RoomDefect{},
		&models.DefectTemplate{},
		&models.Document{},
		&models.Photo{},
	)
	if err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}
	log.Println("Миграции применены")
}
