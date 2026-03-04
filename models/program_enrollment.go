package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProgramEnrollment tracks a user's enrollment in a workout program.
type ProgramEnrollment struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_program_enrollment,priority:1,where:deleted_at IS NULL" json:"user_id"`
	WorkoutProgramID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_user_program_enrollment,priority:2,where:deleted_at IS NULL" json:"workout_program_id"`
	Status           string         `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	StartedOn        time.Time      `gorm:"type:date;not null" json:"started_on"`
	CurrentWeek      int            `gorm:"type:int;not null;default:1" json:"current_week"`
	CompletedDays    int            `gorm:"type:int;not null;default:0" json:"completed_days"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User           User           `gorm:"foreignKey:UserID" json:"user,omitempty"`
	WorkoutProgram WorkoutProgram `gorm:"foreignKey:WorkoutProgramID" json:"workout_program,omitempty"`

	// Has-many
	ProgramProgress []ProgramProgress `gorm:"foreignKey:ProgramEnrollmentID" json:"program_progress,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (pe *ProgramEnrollment) BeforeCreate(tx *gorm.DB) error {
	if pe.ID == uuid.Nil {
		pe.ID = uuid.New()
	}
	if pe.UserID == uuid.Nil || pe.WorkoutProgramID == uuid.Nil {
		return errors.New("user_id and workout_program_id must be set")
	}
	if pe.StartedOn.IsZero() {
		pe.StartedOn = time.Now().UTC()
	}
	if pe.Status == "" {
		pe.Status = "active"
	}
	if pe.CurrentWeek <= 0 {
		pe.CurrentWeek = 1
	}
	if pe.CompletedDays < 0 {
		return errors.New("completed_days cannot be negative")
	}
	return nil
}
