package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Meal represents a meal logged by a user (e.g. breakfast, lunch, dinner, snack).
type Meal struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index:idx_meals_user_date_type,priority:1,where:deleted_at IS NULL" json:"user_id"`
	MealType  string         `gorm:"type:varchar(50);not null;index:idx_meals_user_date_type,priority:3,where:deleted_at IS NULL" json:"meal_type"` // breakfast/lunch/dinner/snack
	Date      time.Time      `gorm:"type:date;not null;index:idx_meals_user_date_type,priority:2,where:deleted_at IS NULL" json:"date"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// Has-many
	MealFoods []MealFood `gorm:"foreignKey:MealID" json:"meal_foods,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (m *Meal) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
