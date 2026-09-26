package storage

import (
	applog "inspection-app/internal/logger"
	"inspection-app/internal/models"
	"log"
	"net/url"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// maskDSN маскирует пароль в DSN для безопасного логирования.
func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}

var DB *gorm.DB

// Connect открывает соединение с PostgreSQL по DSN.
func Connect(dsn string) {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlog.Default.LogMode(gormlog.Warn),
	})
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Ошибка получения sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	applog.Info("database connected", "dsn", maskDSN(dsn), "max_open", 25, "max_idle", 5)
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
		&models.Signature{},
	)
	if err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}
	// Фото без дефекта (общий вид помещения, общие замечания): defect_id стал
	// необязательным, а inspection_id старым записям заполняем по цепочке
	// дефект → помещение (включая архивные).
	DB.Exec("ALTER TABLE photos ALTER COLUMN defect_id DROP NOT NULL")
	DB.Exec(`UPDATE photos p SET inspection_id = r.inspection_id
		FROM room_defects d JOIN inspection_rooms r ON r.id = d.room_id
		WHERE d.id = p.defect_id AND (p.inspection_id IS NULL OR p.inspection_id = 0)`)
	applog.Info("migrations applied")
}
