package models

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleInspector Role = "inspector"
)

// User — пользователь системы
type User struct {
	gorm.Model
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	FullName     string `gorm:"not null"` // ФИО полностью
	Initials     string `gorm:"not null"` // Фамилия И.О.
	Role         Role   `gorm:"not null;default:'inspector'"`
}

// Inspection — акт осмотра объекта
type Inspection struct {
	gorm.Model
	ActNumber       string `gorm:"not null"`
	UserID          uint   `gorm:"not null"`
	User            User
	Date            time.Time `gorm:"not null"`
	InspectionTime  string    // Время осмотра, вводится вручную (например "10:30")
	Address         string    `gorm:"not null"`
	RoomsCount      int
	Floor           int
	TotalArea       float64
	TempOutside     float64
	TempInside      float64
	Humidity        float64
	PlanImage       string // путь к фото плана (nullable)
	OwnerName       string // ФИО собственника
	DeveloperRepName string // ФИО представителя застройщика
	Status          string `gorm:"not null;default:'draft'"` // draft | completed

	Rooms   []InspectionRoom   `gorm:"foreignKey:InspectionID"`
	Defects []InspectionDefect `gorm:"foreignKey:InspectionID"`
}

// InspectionRoom — замеры помещений (необязательная секция)
type InspectionRoom struct {
	gorm.Model
	InspectionID uint `gorm:"not null"`
	RoomNumber   int  // 1-10
	RoomName     string

	// Размеры помещения
	Length  float64
	Width   float64
	Height  float64

	// Откос Окно 1
	Window1Height float64
	Window1Width  float64

	// Откос Окно 2
	Window2Height float64
	Window2Width  float64

	// Дверь/проём
	DoorHeight float64
	DoorWidth  float64
}

// DefectTemplate — справочник дефектов (шаблоны)
type DefectTemplate struct {
	gorm.Model
	Section    string  // window | ceiling | wall | floor | plumbing | other
	Name       string  `gorm:"not null"`
	Threshold  string  // значение в скобках, например "1" для "(1)"
	Unit       string  // единица измерения, например "мм"
	OrderIndex int     // порядок отображения
}

// InspectionDefect — заполненный дефект в рамках осмотра
type InspectionDefect struct {
	gorm.Model
	InspectionID     uint `gorm:"not null"`
	DefectTemplateID uint `gorm:"not null"`
	DefectTemplate   DefectTemplate

	Value      string // введённое значение, например "2мм"
	WallNumber int    // 0 = не стена, 1-4 = ст1-ст4
	Notes      string // поле "Прочее" — произвольный текст
}

// Document — сгенерированный документ
type Document struct {
	gorm.Model
	InspectionID uint   `gorm:"not null"`
	Inspection   Inspection
	Format       string `gorm:"not null"` // pdf | docx
	FilePath     string `gorm:"not null"`
	GeneratedBy  uint   `gorm:"not null"` // user_id
}
