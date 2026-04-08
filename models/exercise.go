package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Exercise represents a predefined exercise in the library.
type Exercise struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	ExerciseLibID    string         `gorm:"type:varchar(100);uniqueIndex:idx_exercise_lib_id,where:deleted_at IS NULL" json:"exercise_lib_id"`
	Name             string         `gorm:"type:varchar(255);not null;uniqueIndex:idx_exercises_name,where:deleted_at IS NULL" json:"name"`
	Force            string         `gorm:"type:varchar(50)" json:"force"`
	Level            string         `gorm:"type:varchar(50)" json:"level"`
	Mechanic         string         `gorm:"type:varchar(50)" json:"mechanic"`
	Equipment        string         `gorm:"type:varchar(100)" json:"equipment"`
	Category         string         `gorm:"type:varchar(100)" json:"category"`
	PrimaryMuscles   string         `gorm:"type:varchar(255)" json:"primary_muscles"`
	SecondaryMuscles string         `gorm:"type:varchar(255)" json:"secondary_muscles"`
	Instructions     string         `gorm:"type:text" json:"instructions"`
	ImageURL         string         `gorm:"type:varchar(512)" json:"image_url"`
	AltImageURL      string         `gorm:"type:varchar(512)" json:"alt_image_url"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Has-many
	WorkoutExercises []WorkoutExercise `gorm:"foreignKey:ExerciseID" json:"workout_exercises,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (e *Exercise) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
		tx.Statement.SetColumn("ID", e.ID)
	}
	if e.ExerciseLibID == "" {
		e.ExerciseLibID = "local-" + e.ID.String()
		tx.Statement.SetColumn("ExerciseLibID", e.ExerciseLibID)
	}
	return nil
}
