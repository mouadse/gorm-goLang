package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkoutTemplate struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OwnerID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name      string         `gorm:"type:varchar(255);not null" json:"name"`
	Type      string         `gorm:"type:varchar(50)" json:"type"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Owner                    User                      `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	WorkoutTemplateExercises []WorkoutTemplateExercise `gorm:"foreignKey:TemplateID" json:"workout_template_exercises,omitempty"`
}

func (t *WorkoutTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	if t.OwnerID == uuid.Nil {
		return errors.New("owner_id must be set")
	}

	if t.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

type WorkoutTemplateExercise struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TemplateID uuid.UUID      `gorm:"type:uuid;not null;index" json:"template_id"`
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

	Template            WorkoutTemplate      `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	Exercise            Exercise             `gorm:"foreignKey:ExerciseID" json:"exercise,omitempty"`
	WorkoutTemplateSets []WorkoutTemplateSet `gorm:"foreignKey:TemplateExerciseID" json:"workout_template_sets,omitempty"`
}

func (e *WorkoutTemplateExercise) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}

	if e.TemplateID == uuid.Nil {
		return errors.New("template_id must be set")
	}

	if e.ExerciseID == uuid.Nil {
		return errors.New("exercise_id must be set")
	}

	if e.Order <= 0 {
		return errors.New("order must be greater than zero")
	}

	return nil
}

type WorkoutTemplateSet struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TemplateExerciseID uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_template_exercise_set_number,priority:1,where:deleted_at IS NULL" json:"template_exercise_id"`
	SetNumber          int            `gorm:"type:int;not null;uniqueIndex:idx_template_exercise_set_number,priority:2,where:deleted_at IS NULL" json:"set_number"`
	Reps               int            `gorm:"type:int;not null" json:"reps"`
	Weight             float64        `gorm:"type:decimal(6,2)" json:"weight"`
	RestSeconds        int            `gorm:"type:int" json:"rest_seconds"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

	TemplateExercise WorkoutTemplateExercise `gorm:"foreignKey:TemplateExerciseID" json:"template_exercise,omitempty"`
}

func (s *WorkoutTemplateSet) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}

	if s.TemplateExerciseID == uuid.Nil {
		return errors.New("template_exercise_id must be set")
	}

	if s.SetNumber <= 0 {
		return errors.New("set_number must be greater than zero")
	}

	if s.Reps < 0 {
		return errors.New("reps cannot be negative")
	}

	if s.Weight < 0 {
		return errors.New("weight cannot be negative")
	}

	return nil
}
