package storage

import (
	"inspection-app/internal/models"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(dsn string) {
	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	log.Println("БД подключена:", dsn)
}

func Migrate() {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Inspection{},
		&models.InspectionRoom{},
		&models.DefectTemplate{},
		&models.InspectionDefect{},
		&models.Document{},
	)
	if err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}
	log.Println("Миграции применены")
}
