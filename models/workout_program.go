package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkoutProgram struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedBy   uuid.UUID      `gorm:"type:uuid;not null;index" json:"created_by"`
	IsActive    bool           `gorm:"type:boolean;default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Creator User          `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Weeks   []ProgramWeek `gorm:"foreignKey:ProgramID" json:"weeks,omitempty"`
}

func (p *WorkoutProgram) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	if p.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

type ProgramWeek struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	ProgramID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"program_id"`
	WeekNumber int            `gorm:"type:int;not null" json:"week_number"`
	Name       string         `gorm:"type:varchar(255)" json:"name"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Program  WorkoutProgram   `gorm:"foreignKey:ProgramID" json:"program,omitempty"`
	Sessions []ProgramSession `gorm:"foreignKey:WeekID" json:"sessions,omitempty"`
}

func (w *ProgramWeek) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}

	if w.ProgramID == uuid.Nil {
		return errors.New("program_id must be set")
	}

	return nil
}

type ProgramSession struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	WeekID            uuid.UUID      `gorm:"type:uuid;not null;index" json:"week_id"`
	DayNumber         int            `gorm:"type:int;not null" json:"day_number"`
	WorkoutTemplateID *uuid.UUID     `gorm:"type:uuid" json:"workout_template_id,omitempty"`
	Notes             string         `gorm:"type:text" json:"notes"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	Week     ProgramWeek      `gorm:"foreignKey:WeekID" json:"week,omitempty"`
	Template *WorkoutTemplate `gorm:"foreignKey:WorkoutTemplateID" json:"template,omitempty"`
}

func (s *ProgramSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	if s.WeekID == uuid.Nil {
		return errors.New("week_id must be set")
	}

	return nil
}

type ProgramAssignment struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	ProgramID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"program_id"`
	AssignedAt  time.Time      `gorm:"not null" json:"assigned_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Status      string         `gorm:"type:varchar(50);not null;default:'assigned'" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User    User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Program WorkoutProgram `gorm:"foreignKey:ProgramID" json:"program,omitempty"`
}

func (a *ProgramAssignment) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}

	if a.UserID == uuid.Nil {
		return errors.New("user_id must be set")
	}

	if a.ProgramID == uuid.Nil {
		return errors.New("program_id must be set")
	}

	if a.Status == "" {
		a.Status = "assigned"
	}

	return nil
}
