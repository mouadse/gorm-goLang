package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ValidCardioModalities = map[string]bool{
	"running":     true,
	"cycling":     true,
	"walking":     true,
	"swimming":    true,
	"rowing":      true,
	"cardio":      true,
	"elliptical":  true,
	"stairmaster": true,
	"hiking":      true,
	"other":       true,
}

type WorkoutCardioEntry struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	WorkoutID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"workout_id"`
	Modality        string         `gorm:"type:varchar(50);not null" json:"modality"`
	DurationMinutes int            `gorm:"type:int;not null" json:"duration_minutes"`
	Distance        *float64       `gorm:"type:decimal(7,2)" json:"distance,omitempty"`
	DistanceUnit    *string        `gorm:"type:varchar(10)" json:"distance_unit,omitempty"`
	Pace            *float64       `gorm:"type:decimal(5,2)" json:"pace,omitempty"`
	CaloriesBurned  *int           `gorm:"type:int" json:"calories_burned,omitempty"`
	AvgHeartRate    *int           `gorm:"type:int" json:"avg_heart_rate,omitempty"`
	Notes           string         `gorm:"type:text" json:"notes"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	Workout Workout `gorm:"foreignKey:WorkoutID" json:"workout,omitempty"`
}

func (e *WorkoutCardioEntry) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}

	if e.WorkoutID == uuid.Nil {
		return errors.New("workout_id must be set")
	}

	if e.DurationMinutes <= 0 {
		return errors.New("duration_minutes must be greater than zero")
	}

	if strings.TrimSpace(e.Modality) == "" {
		return errors.New("modality is required")
	}

	if e.Distance != nil && *e.Distance < 0 {
		return errors.New("distance cannot be negative")
	}

	if e.CaloriesBurned != nil && *e.CaloriesBurned < 0 {
		return errors.New("calories_burned cannot be negative")
	}

	if e.AvgHeartRate != nil && *e.AvgHeartRate < 0 {
		return errors.New("avg_heart_rate cannot be negative")
	}

	return nil
}
