package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkoutExercise is a join table between Workouts and Exercises,
// capturing sets, reps, weight, and rest time for each exercise in a workout.
type WorkoutExercise struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkoutID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"workout_id"`
	ExerciseID uuid.UUID      `gorm:"type:uuid;not null;index" json:"exercise_id"`
	Order      int            `gorm:"type:int;not null;default:0" json:"order"`
	Sets       int            `gorm:"type:int" json:"sets"`
	Reps       int            `gorm:"type:int" json:"reps"`
	Weight     float64        `gorm:"type:decimal(6,2)" json:"weight"`
	RestTime   int            `gorm:"type:int" json:"rest_time"` // seconds
	Notes      string         `gorm:"type:text" json:"notes"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	Workout  Workout  `gorm:"foreignKey:WorkoutID" json:"workout,omitempty"`
	Exercise Exercise `gorm:"foreignKey:ExerciseID" json:"exercise,omitempty"`

	// Has-many
	WorkoutSets []WorkoutSet `gorm:"foreignKey:WorkoutExerciseID" json:"workout_sets,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (we *WorkoutExercise) BeforeCreate(tx *gorm.DB) error {
	if we.ID == uuid.Nil {
		we.ID = uuid.New()
	}
	return nil
}
