package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Workout represents a single workout session logged by a user.
type Workout struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Date      time.Time      `gorm:"type:date;not null" json:"date"`
	Duration  int            `gorm:"type:int" json:"duration"`
	Notes     string         `gorm:"type:text" json:"notes"`
	Type      string         `gorm:"type:varchar(50)" json:"type"` // push/pull/legs/cardio
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// Has-many
	WorkoutExercises []WorkoutExercise `gorm:"foreignKey:WorkoutID" json:"workout_exercises,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (w *Workout) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}
