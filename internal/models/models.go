package models

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleInspector Role = "inspector"
)

// User — пользователь системы
type User struct {
	gorm.Model
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	FullName     string `gorm:"not null"`
	Initials     string `gorm:"not null"`
	Role         Role   `gorm:"not null;default:'inspector'"`
	AvatarURL    string
	ResetToken   string
	ResetExpiry  *time.Time
}

// Inspection — акт осмотра объекта
type Inspection struct {
	gorm.Model
	ActNumber        string `gorm:"not null"`
	UserID           uint   `gorm:"not null"`
	User             User
	Date             time.Time `gorm:"not null"`
	InspectionTime   string
	Address          string `gorm:"not null"`
	RoomsCount       int
	Floor            int
	TotalArea        float64
	TempOutside      float64
	TempInside       float64
	Humidity         float64
	PlanImage        string
	OwnerName        string
	DeveloperRepName string
	Status           string `gorm:"not null;default:'draft'"`
	PhotoFolderURL   string // публичная ссылка на папку с фото в облаке

	Rooms []InspectionRoom `gorm:"foreignKey:InspectionID"`
}

// Photo — фотография дефекта
type Photo struct {
	gorm.Model
	DefectID uint   `gorm:"not null;index"`
	FileURL  string // публичная ссылка на файл (после синхронизации с облаком)
	FilePath string // локальный путь до файла (до синхронизации)
	FileName string
}

// InspectionRoom — помещение (основная единица, содержит замеры и дефекты)
type InspectionRoom struct {
	gorm.Model
	InspectionID uint `gorm:"not null;index"`
	RoomNumber   int
	RoomName     string

	// Замеры помещения
	Length float64
	Width  float64
	Height float64

	// Откос Окно 1
	Window1Height float64
	Window1Width  float64

	// Откос Окно 2
	Window2Height float64
	Window2Width  float64

	// Откос Окно 3
	Window3Height float64
	Window3Width  float64

	// Откос Окно 4
	Window4Height float64
	Window4Width  float64

	// Откос Окно 5
	Window5Height float64
	Window5Width  float64

	// Дверь/проём
	DoorHeight float64
	DoorWidth  float64

	// Типы отделки
	WindowType string // pvc | al | wood
	WallType   string // paint

	// Дефекты этого помещения
	Defects []RoomDefect `gorm:"foreignKey:RoomID"`
}

// RoomDefect — дефект в конкретном помещении
type RoomDefect struct {
	gorm.Model
	RoomID           uint           `gorm:"not null;index"`
	DefectTemplateID *uint          // nil = запись "Прочее"
	DefectTemplate   DefectTemplate `gorm:"foreignKey:DefectTemplateID"`
	Section          string         // window | ceiling | wall | floor | door | plumbing
	Value            string
	WallNumber       int    // 0 = не стена, 1-4 = ст1-ст4
	Notes            string // текст поля "Прочее"
	Photos           []Photo `gorm:"foreignKey:DefectID"`
}

// DefectTemplate — справочник дефектов
type DefectTemplate struct {
	gorm.Model
	Section    string `gorm:"not null;index"`
	Name       string `gorm:"not null"`
	Threshold  string
	Unit       string
	OrderIndex int
}

// Document — сгенерированный документ
type Document struct {
	gorm.Model
	InspectionID uint `gorm:"not null"`
	Inspection   Inspection
	Format       string `gorm:"not null"` // pdf | docx
	FilePath     string `gorm:"not null"`
	GeneratedBy  uint   `gorm:"not null"`
}
