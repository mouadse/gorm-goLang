package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkoutSet stores set-by-set workout logging details.
type WorkoutSet struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkoutExerciseID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_workout_exercise_set_number,priority:1,where:deleted_at IS NULL" json:"workout_exercise_id"`
	SetNumber         int            `gorm:"type:int;not null;uniqueIndex:idx_workout_exercise_set_number,priority:2,where:deleted_at IS NULL" json:"set_number"`
	Reps              int            `gorm:"type:int;not null" json:"reps"`
	Weight            float64        `gorm:"type:decimal(6,2)" json:"weight"`
	RPE               float64        `gorm:"type:decimal(3,1)" json:"rpe"`
	RestSeconds       int            `gorm:"type:int" json:"rest_seconds"`
	Completed         bool           `gorm:"type:boolean;not null;default:true" json:"completed"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	WorkoutExercise WorkoutExercise `gorm:"foreignKey:WorkoutExerciseID" json:"workout_exercise,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (ws *WorkoutSet) BeforeCreate(tx *gorm.DB) error {
	if ws.ID == uuid.Nil {
		ws.ID = uuid.New()
	}
	if ws.WorkoutExerciseID == uuid.Nil {
		return errors.New("workout_exercise_id must be set")
	}
	if ws.SetNumber <= 0 {
		return errors.New("set_number must be greater than zero")
	}
	if ws.Reps < 0 {
		return errors.New("reps cannot be negative")
	}
	if ws.Weight < 0 {
		return errors.New("weight cannot be negative")
	}
	return nil
}
