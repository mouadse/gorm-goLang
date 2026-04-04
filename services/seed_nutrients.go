package services

import (
	"fmt"
	"log"

	"fitness-tracker/models"

	"gorm.io/gorm"
)

// SeedNutrients ensures all Level 1 nutrients exist in the nutrients table.
// This is idempotent — safe to call multiple times without creating duplicates.
// Returns the number of nutrients created.
func SeedNutrients(db *gorm.DB) (int, error) {
	level1 := models.Level1Nutrients()
	created := 0

	for _, info := range level1 {
		nutrient := models.Nutrient{
			Code:     info.Code,
			Name:     info.Name,
			Unit:     info.Unit,
			Category: info.Category,
		}

		result := db.Where("code = ?", info.Code).FirstOrCreate(&nutrient)
		if result.Error != nil {
			return created, fmt.Errorf("seed nutrient %s: %w", info.Code, result.Error)
		}

		if result.RowsAffected > 0 {
			created++
		}
	}

	log.Printf("Nutrient seeding complete: %d created, %d already existed", created, len(level1)-created)
	return created, nil
}

// NutrientCodeToID builds a lookup map from nutrient code to its database UUID.
// This is used during USDA import to link food_nutrients records efficiently.
func NutrientCodeToID(db *gorm.DB) (map[string]models.Nutrient, error) {
	var nutrients []models.Nutrient
	if err := db.Find(&nutrients).Error; err != nil {
		return nil, fmt.Errorf("load nutrients: %w", err)
	}

	lookup := make(map[string]models.Nutrient, len(nutrients))
	for _, n := range nutrients {
		lookup[n.Code] = n
	}

	return lookup, nil
}
