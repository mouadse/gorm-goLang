package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MealFood represents a specific food item included in a meal.
type MealFood struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	MealID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"meal_id"`
	FoodID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"food_id"`
	Quantity  float64        `gorm:"type:decimal(10,2);not null" json:"quantity"` // amount in food's serving unit
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Meal Meal `gorm:"foreignKey:MealID" json:"-"`
	Food Food `gorm:"foreignKey:FoodID" json:"food,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (mf *MealFood) BeforeCreate(tx *gorm.DB) error {
	if mf.ID == uuid.Nil {
		mf.ID = uuid.New()
	}
	return nil
}
