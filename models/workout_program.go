package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkoutProgram represents a structured multi-week workout plan created by an admin.
type WorkoutProgram struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	Description   string         `gorm:"type:text" json:"description"`
	DurationWeeks int            `gorm:"type:int" json:"duration_weeks"`
	CreatedBy     uuid.UUID      `gorm:"type:uuid;not null;index" json:"created_by"` // FK → Users (admin)
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	Creator User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (wp *WorkoutProgram) BeforeCreate(tx *gorm.DB) error {
	if wp.ID == uuid.Nil {
		wp.ID = uuid.New()
	}
	return nil
}
