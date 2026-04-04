package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Nutrient represents a type of nutrient (vitamin, mineral, etc.)
type Nutrient struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Code      string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"code"` // USDA nutrient number (e.g., "301" for Calcium)
	Name      string         `gorm:"type:varchar(255);not null" json:"name"`            // Display name (e.g., "Calcium, Ca")
	Unit      string         `gorm:"type:varchar(20);not null" json:"unit"`             // Unit of measurement (e.g., "mg", "µg")
	Category  string         `gorm:"type:varchar(50);not null" json:"category"`         // vitamin, mineral, macronutrient, etc.
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate sets a new UUID before inserting.
func (n *Nutrient) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// FoodNutrient links foods to their nutrient values (normalized many-to-many)
type FoodNutrient struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	FoodID        uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_food_nutrients_unique" json:"food_id"`
	NutrientID    uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_food_nutrients_unique" json:"nutrient_id"`
	AmountPer100g float64        `gorm:"type:decimal(10,4);not null" json:"amount_per_100g"` // Amount per 100g of food
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Food     Food     `gorm:"foreignKey:FoodID" json:"food,omitempty"`
	Nutrient Nutrient `gorm:"foreignKey:NutrientID" json:"nutrient,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (fn *FoodNutrient) BeforeCreate(tx *gorm.DB) error {
	if fn.ID == uuid.Nil {
		fn.ID = uuid.New()
	}
	return nil
}

// Level1Nutrients returns the USDA nutrient IDs for Phase 4 Level 1 nutrients
// These are the 15-20 key nutrients to track initially
func Level1Nutrients() map[int]NutrientInfo {
	return map[int]NutrientInfo{
		// Vitamins
		1104: {Code: "1104", Name: "Vitamin A, RAE", Unit: "µg", Category: "vitamin"},
		1162: {Code: "1162", Name: "Vitamin C, total ascorbic acid", Unit: "mg", Category: "vitamin"},
		1114: {Code: "1114", Name: "Vitamin D (D2 + D3)", Unit: "µg", Category: "vitamin"},
		1109: {Code: "1109", Name: "Vitamin E (alpha-tocopherol)", Unit: "mg", Category: "vitamin"},
		1185: {Code: "1185", Name: "Vitamin K (phylloquinone)", Unit: "µg", Category: "vitamin"},
		1165: {Code: "1165", Name: "Thiamin (B1)", Unit: "mg", Category: "vitamin"},
		1166: {Code: "1166", Name: "Riboflavin (B2)", Unit: "mg", Category: "vitamin"},
		1167: {Code: "1167", Name: "Niacin (B3)", Unit: "mg", Category: "vitamin"},
		1170: {Code: "1170", Name: "Pantothenic acid (B5)", Unit: "mg", Category: "vitamin"},
		1175: {Code: "1175", Name: "Vitamin B-6", Unit: "mg", Category: "vitamin"},
		1178: {Code: "1178", Name: "Vitamin B-12", Unit: "µg", Category: "vitamin"},
		1177: {Code: "1177", Name: "Folate, total", Unit: "µg", Category: "vitamin"},
		// Minerals
		1087: {Code: "1087", Name: "Calcium, Ca", Unit: "mg", Category: "mineral"},
		1089: {Code: "1089", Name: "Iron, Fe", Unit: "mg", Category: "mineral"},
		1095: {Code: "1095", Name: "Zinc, Zn", Unit: "mg", Category: "mineral"},
		1090: {Code: "1090", Name: "Magnesium, Mg", Unit: "mg", Category: "mineral"},
		1092: {Code: "1092", Name: "Potassium, K", Unit: "mg", Category: "mineral"},
		1093: {Code: "1093", Name: "Sodium, Na", Unit: "mg", Category: "mineral"},
		1091: {Code: "1091", Name: "Phosphorus, P", Unit: "mg", Category: "mineral"},
	}
}

// NutrientInfo holds display information for a nutrient
type NutrientInfo struct {
	Code     string
	Name     string
	Unit     string
	Category string
}

// NutrientMappings maps USDA nutrient IDs to our internal codes
func NutrientMappings() map[int]string {
	return map[int]string{
		// Energy
		1008: "ENERC_KCAL",
		2047: "ENERC_KJ",
		// Macronutrients
		1003: "PROCNT",
		1004: "FAT",
		1005: "CHOCDF",
		1079: "FIBTG",
		1063: "SUGAR",
		// Vitamins
		1104: "VITA_RAE",
		1162: "VITC",
		1114: "VITD",
		1109: "VITE",
		1185: "VITK",
		1165: "THIA",
		1166: "RIBF",
		1167: "NIA",
		1170: "PANTAC",
		1175: "VITB6A",
		1178: "VITB12",
		1177: "FOL",
		// Minerals
		1087: "CA",
		1089: "FE",
		1095: "ZN",
		1090: "MG",
		1092: "K",
		1093: "NA",
		1091: "P",
		// Lipids
		1253: "CHOLE",
	}
}
