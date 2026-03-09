package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Exercise represents a predefined exercise in the library.
type Exercise struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(255);not null;uniqueIndex:idx_exercises_name,where:deleted_at IS NULL" json:"name"`
	MuscleGroup  string         `gorm:"type:varchar(100)" json:"muscle_group"`
	Equipment    string         `gorm:"type:varchar(100)" json:"equipment"`
	Difficulty   string         `gorm:"type:varchar(50)" json:"difficulty"`
	Instructions string         `gorm:"type:text" json:"instructions"`
	VideoURL     string         `gorm:"type:varchar(512)" json:"video_url"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Has-many
	WorkoutExercises []WorkoutExercise `gorm:"foreignKey:ExerciseID" json:"workout_exercises,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (e *Exercise) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
