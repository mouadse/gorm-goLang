package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Food represents a food item in the nutrition database.
type Food struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name           string         `gorm:"type:varchar(255);not null;uniqueIndex:idx_name_brand,where:deleted_at IS NULL" json:"name"`
	Brand          string         `gorm:"type:varchar(255);not null;default:'';uniqueIndex:idx_name_brand,where:deleted_at IS NULL" json:"brand"`
	Calories       int            `gorm:"type:int" json:"calories"`
	Protein        float64        `gorm:"type:decimal(6,2)" json:"protein"`
	Carbs          float64        `gorm:"type:decimal(6,2)" json:"carbs"`
	Fats           float64        `gorm:"type:decimal(6,2)" json:"fats"`
	Fiber          float64        `gorm:"type:decimal(6,2)" json:"fiber"`
	Micronutrients datatypes.JSON `gorm:"type:jsonb" json:"micronutrients"`
	Verified       bool           `gorm:"type:boolean;default:false" json:"verified"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Has-many
	MealFoods []MealFood `gorm:"foreignKey:FoodID" json:"meal_foods,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (f *Food) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
