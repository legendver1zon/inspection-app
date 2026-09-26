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
	ActNumber        string `gorm:"uniqueIndex:idx_inspections_act_number,where:deleted_at IS NULL;not null"`
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
	Status           string `gorm:"not null;default:'draft';index"`
	PhotoFolderURL   string // публичная ссылка на папку с фото в облаке

	// Общие замечания по квартире (не привязаны к помещению)
	Electricity  string
	Ventilation  string
	GeneralNotes string

	Rooms []InspectionRoom `gorm:"foreignKey:InspectionID"`
}

// Виды фото: дефекта, общего вида помещения и общих замечаний по квартире.
const (
	PhotoKindDefect      = "defect"
	PhotoKindRoom        = "room"
	PhotoKindElectricity = "electricity"
	PhotoKindVentilation = "ventilation"
	PhotoKindGeneral     = "general"
)

// Photo — фотография. У фото дефекта заполнен DefectID, у общего вида
// помещения — RoomNumber, у общих замечаний — только Kind.
type Photo struct {
	gorm.Model
	InspectionID  uint   `gorm:"index"`
	Kind          string `gorm:"size:16;not null;default:'defect';index"`
	RoomNumber    int
	DefectID      *uint  `gorm:"index"`
	FileURL       string // публичная ссылка на файл (после синхронизации с облаком)
	FilePath      string // локальный путь до файла (до синхронизации)
	FileName      string
	UploadStatus  string     `gorm:"not null;default:'done';index"` // pending | uploading | done | failed
	RetryCount    int        `gorm:"not null;default:0"`
	LastError     string     // последняя ошибка загрузки (для диагностики)
	LastAttemptAt *time.Time // время последней попытки загрузки
	ClientID      *string    `gorm:"size:64;uniqueIndex"` // идентификатор из офлайн-очереди клиента (идемпотентность)
}

// BeforeCreate доопределяет осмотр по дефекту, если фото создано без
// InspectionID (старый код, служебные утилиты, фикстуры тестов).
func (p *Photo) BeforeCreate(tx *gorm.DB) error {
	if p.Kind == "" {
		p.Kind = PhotoKindDefect
	}
	if p.InspectionID != 0 || p.DefectID == nil {
		return nil
	}
	var row struct{ InspectionID uint }
	tx.Session(&gorm.Session{NewDB: true}).Unscoped().
		Table("room_defects").
		Select("inspection_rooms.inspection_id").
		Joins("JOIN inspection_rooms ON inspection_rooms.id = room_defects.room_id").
		Where("room_defects.id = ?", *p.DefectID).
		Scan(&row)
	p.InspectionID = row.InspectionID
	return nil
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
	WallNumber       int     // 0 = не стена, 1-4 = ст1-ст4
	Notes            string  // текст поля "Прочее"
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
