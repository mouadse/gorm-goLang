package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MealFood is a join table between Meals and Foods,
// capturing the quantity and unit of each food in a meal.
type MealFood struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MealID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"meal_id"`
	FoodID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"food_id"`
	Quantity  float64        `gorm:"type:decimal(8,2)" json:"quantity"`
	Unit      string         `gorm:"type:varchar(50)" json:"unit"` // grams, ml, serving, etc.
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Belongs-to
	Meal Meal `gorm:"foreignKey:MealID" json:"meal,omitempty"`
	Food Food `gorm:"foreignKey:FoodID" json:"food,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (mf *MealFood) BeforeCreate(tx *gorm.DB) error {
	if mf.ID == uuid.Nil {
		mf.ID = uuid.New()
	}
	return nil
}
