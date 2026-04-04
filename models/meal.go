package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Meal represents a meal logged by a user (e.g. breakfast, lunch, dinner, snack).
type Meal struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
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
	Items []MealFood `gorm:"foreignKey:MealID;constraint:OnDelete:CASCADE;" json:"items,omitempty"`

	// Computed fields
	TotalCalories float64 `gorm:"-" json:"total_calories"`
	TotalProtein  float64 `gorm:"-" json:"total_protein"`
	TotalCarbs    float64 `gorm:"-" json:"total_carbs"`
	TotalFat      float64 `gorm:"-" json:"total_fat"`
	TotalFiber    float64 `gorm:"-" json:"total_fiber"`
}

// CalculateTotals computes the sum of all macros from the meal's items.
func (m *Meal) CalculateTotals() {
	var calories, protein, carbs, fat, fiber float64
	for _, item := range m.Items {
		calories += item.Food.Calories * item.Quantity
		protein += item.Food.Protein * item.Quantity
		carbs += item.Food.Carbohydrates * item.Quantity
		fat += item.Food.Fat * item.Quantity
		fiber += item.Food.Fiber * item.Quantity
	}
	m.TotalCalories = calories
	m.TotalProtein = protein
	m.TotalCarbs = carbs
	m.TotalFat = fat
	m.TotalFiber = fiber
}

// BeforeCreate sets a new UUID before inserting.
func (m *Meal) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
