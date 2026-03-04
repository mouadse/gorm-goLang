package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProgramProgress stores per-day completion data for a program enrollment.
type ProgramProgress struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProgramEnrollmentID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_enrollment_week_day,priority:1,where:deleted_at IS NULL" json:"program_enrollment_id"`
	WeekNumber          int            `gorm:"type:int;not null;uniqueIndex:idx_enrollment_week_day,priority:2,where:deleted_at IS NULL" json:"week_number"`
	DayNumber           int            `gorm:"type:int;not null;uniqueIndex:idx_enrollment_week_day,priority:3,where:deleted_at IS NULL" json:"day_number"`
	WorkoutID           *uuid.UUID     `gorm:"type:uuid;index" json:"workout_id,omitempty"`
	Completed           bool           `gorm:"type:boolean;not null;default:false" json:"completed"`
	CompletedAt         *time.Time     `gorm:"type:timestamptz" json:"completed_at,omitempty"`
	Notes               string         `gorm:"type:text" json:"notes"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	ProgramEnrollment ProgramEnrollment `gorm:"foreignKey:ProgramEnrollmentID" json:"program_enrollment,omitempty"`
	Workout           Workout           `gorm:"foreignKey:WorkoutID" json:"workout,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (pp *ProgramProgress) BeforeCreate(tx *gorm.DB) error {
	if pp.ID == uuid.Nil {
		pp.ID = uuid.New()
	}
	if pp.ProgramEnrollmentID == uuid.Nil {
		return errors.New("program_enrollment_id must be set")
	}
	if pp.WeekNumber <= 0 {
		return errors.New("week_number must be greater than zero")
	}
	if pp.DayNumber <= 0 {
		return errors.New("day_number must be greater than zero")
	}
	if pp.DayNumber > 7 {
		return errors.New("day_number must be between 1 and 7")
	}
	if pp.Completed && pp.CompletedAt == nil {
		now := time.Now().UTC()
		pp.CompletedAt = &now
	}
	return nil
}
