package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Recipe represents a reusable collection of foods.
type Recipe struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Name      string         `gorm:"type:varchar(255);not null" json:"name"`
	Servings  int            `gorm:"type:int;not null;default:1" json:"servings"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Items []RecipeItem `gorm:"foreignKey:RecipeID;constraint:OnDelete:CASCADE;" json:"items,omitempty"`
}

// BeforeCreate sets a new UUID before inserting for Recipe.
func (r *Recipe) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// RecipeItem represents an ingredient inside a recipe.
type RecipeItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RecipeID  uuid.UUID `gorm:"type:uuid;not null;index" json:"recipe_id"`
	FoodID    uuid.UUID `gorm:"type:uuid;not null" json:"food_id"`
	Quantity  float64   `gorm:"type:decimal(10,2);not null" json:"quantity"` // Usually scaled serving multiplier
	CreatedAt time.Time `json:"created_at"`

	Food Food `gorm:"foreignKey:FoodID" json:"food,omitempty"`
}

// BeforeCreate sets a new UUID before inserting for RecipeItem.
func (ri *RecipeItem) BeforeCreate(tx *gorm.DB) error {
	if ri.ID == uuid.Nil {
		ri.ID = uuid.New()
	}
	return nil
}
