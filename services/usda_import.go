package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"fitness-tracker/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// USDA nutrient IDs
	usdaNutrientCalories              = 1008
	usdaNutrientEnergyAtwaterGeneral  = 2047
	usdaNutrientEnergyAtwaterSpecific = 2048
	usdaNutrientProtein               = 1003
	usdaNutrientFat                   = 1004
	usdaNutrientCarbs                 = 1005
	usdaNutrientFiber                 = 1079
	usdaNutrientSugar                 = 1063
	usdaNutrientSodium                = 1093

	importBatchSize = 100
)

// ImportStats reports the outcome of a USDA import run.
type ImportStats struct {
	FoodCount   int           `json:"food_count"`
	NewFoods    int           `json:"new_foods"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Duration    time.Duration `json:"duration"`
	NutrientRow int           `json:"nutrient_rows"`
}

// USDAImportService handles importing USDA Foundation Foods data into PostgreSQL.
type USDAImportService struct {
	db *gorm.DB
}

// NewUSDAImportService creates a new import service backed by the given database.
func NewUSDAImportService(db *gorm.DB) *USDAImportService {
	return &USDAImportService{db: db}
}

// --- Raw USDA JSON types (ported from standalone app) ---

type usdaRawFood struct {
	FdcID         int                   `json:"fdcId"`
	Description   string                `json:"description"`
	FoodCategory  usdaRawFoodCategory   `json:"foodCategory"`
	FoodNutrients []usdaRawFoodNutrient `json:"foodNutrients"`
	FoodPortions  []usdaRawFoodPortion  `json:"foodPortions"`
}

type usdaRawFoodCategory struct {
	Description string `json:"description"`
}

type usdaRawFoodNutrient struct {
	Amount   float64         `json:"amount"`
	Nutrient usdaRawNutrient `json:"nutrient"`
}

type usdaRawNutrient struct {
	ID int `json:"id"`
}

type usdaRawFoodPortion struct {
	ID          int                `json:"id"`
	Value       float64            `json:"value"`
	MeasureUnit usdaRawMeasureUnit `json:"measureUnit"`
	Modifier    string             `json:"modifier"`
	GramWeight  float64            `json:"gramWeight"`
	Amount      float64            `json:"amount"`
}

type usdaRawMeasureUnit struct {
	Name string `json:"name"`
}

// extractedFood holds the parsed food data ready for database insertion.
type extractedFood struct {
	fdcID       int
	name        string
	category    string
	calories    float64
	protein     float64
	carbs       float64
	fat         float64
	fiber       float64
	sugar       float64
	sodium      float64
	servingSize float64
	servingUnit string
	// Per-100g nutrient amounts keyed by USDA nutrient ID
	nutrientsPer100g map[int]float64
}

// ImportFromFile streams the USDA Foundation Foods JSON file and inserts/updates
// foods and their nutrients into PostgreSQL. The import is idempotent — running
// it multiple times with the same data produces no duplicates.
func (s *USDAImportService) ImportFromFile(ctx context.Context, path string) (ImportStats, error) {
	startedAt := time.Now()

	// Ensure nutrients table is seeded
	if _, err := SeedNutrients(s.db); err != nil {
		return ImportStats{}, fmt.Errorf("seed nutrients: %w", err)
	}

	// Build nutrient code → DB record lookup
	nutrientLookup, err := NutrientCodeToID(s.db)
	if err != nil {
		return ImportStats{}, fmt.Errorf("build nutrient lookup: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return ImportStats{}, fmt.Errorf("open dataset: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	var batch []extractedFood
	stats := ImportStats{}

	err = decodeFoundationFoods(decoder, func(raw usdaRawFood) error {
		if err := ctx.Err(); err != nil {
			return err
		}

		if raw.FdcID == 0 || strings.TrimSpace(raw.Description) == "" {
			return nil
		}

		food := extractFood(raw)
		batch = append(batch, food)

		if len(batch) >= importBatchSize {
			inserted, nutrientRows, err := s.persistBatch(batch, nutrientLookup)
			if err != nil {
				return err
			}
			stats.FoodCount += len(batch)
			stats.NewFoods += inserted
			stats.NutrientRow += nutrientRows
			batch = batch[:0]
		}

		return nil
	})
	if err != nil {
		return ImportStats{}, fmt.Errorf("decode USDA dataset: %w", err)
	}

	// Flush remaining batch
	if len(batch) > 0 {
		inserted, nutrientRows, err := s.persistBatch(batch, nutrientLookup)
		if err != nil {
			return ImportStats{}, fmt.Errorf("flush final batch: %w", err)
		}
		stats.FoodCount += len(batch)
		stats.NewFoods += inserted
		stats.NutrientRow += nutrientRows
	}

	stats.Duration = time.Since(startedAt)
	stats.UpdatedAt = time.Now().UTC()

	log.Printf(
		"USDA import complete: %d foods processed (%d new), %d nutrient rows, took %s",
		stats.FoodCount, stats.NewFoods, stats.NutrientRow, stats.Duration.Round(time.Millisecond),
	)

	return stats, nil
}

// extractFood converts a raw USDA food into our internal representation.
func extractFood(raw usdaRawFood) extractedFood {
	var (
		calories        float64
		atwaterGeneral  float64
		atwaterSpecific float64
		protein         float64
		fat             float64
		carbs           float64
		fiber           float64
		sugar           float64
		sodium          float64
	)

	nutrientsPer100g := make(map[int]float64, len(raw.FoodNutrients))
	for _, n := range raw.FoodNutrients {
		nutrientsPer100g[n.Nutrient.ID] = n.Amount

		switch n.Nutrient.ID {
		case usdaNutrientCalories:
			calories = n.Amount
		case usdaNutrientEnergyAtwaterGeneral:
			atwaterGeneral = n.Amount
		case usdaNutrientEnergyAtwaterSpecific:
			atwaterSpecific = n.Amount
		case usdaNutrientProtein:
			protein = n.Amount
		case usdaNutrientFat:
			fat = n.Amount
		case usdaNutrientCarbs:
			carbs = n.Amount
		case usdaNutrientFiber:
			fiber = n.Amount
		case usdaNutrientSugar:
			sugar = n.Amount
		case usdaNutrientSodium:
			sodium = n.Amount
		}
	}

	// Fallback calorie sources
	if calories == 0 {
		calories = atwaterGeneral
	}
	if calories == 0 {
		calories = atwaterSpecific
	}

	// Best serving size from portions
	servingSize := 100.0
	servingUnit := "g"
	if len(raw.FoodPortions) > 0 {
		bestPortion := raw.FoodPortions[0]
		if bestPortion.GramWeight > 0 {
			servingSize = bestPortion.GramWeight
			if bestPortion.MeasureUnit.Name != "" {
				servingUnit = bestPortion.MeasureUnit.Name
			}
		}
	}

	multiplier := servingSize / 100.0

	return extractedFood{
		fdcID:            raw.FdcID,
		name:             strings.TrimSpace(raw.Description),
		category:         strings.TrimSpace(raw.FoodCategory.Description),
		calories:         calories * multiplier,
		protein:          protein * multiplier,
		carbs:            carbs * multiplier,
		fat:              fat * multiplier,
		fiber:            fiber * multiplier,
		sugar:            sugar * multiplier,
		sodium:           sodium * multiplier,
		servingSize:      servingSize,
		servingUnit:      servingUnit,
		nutrientsPer100g: nutrientsPer100g,
	}
}

// persistBatch writes a batch of extracted foods to the database in a single transaction.
// Returns the number of newly inserted foods and total nutrient rows upserted.
func (s *USDAImportService) persistBatch(
	batch []extractedFood,
	nutrientLookup map[string]models.Nutrient,
) (int, int, error) {
	newFoods := 0
	nutrientRows := 0

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, ef := range batch {
			fdcID := ef.fdcID
			food := models.Food{
				FdcID:         &fdcID,
				Name:          ef.name,
				Brand:         "USDA",
				Category:      ef.category,
				Source:        "usda",
				ServingSize:   ef.servingSize,
				ServingUnit:   ef.servingUnit,
				Calories:      round2(ef.calories),
				Protein:       round2(ef.protein),
				Carbohydrates: round2(ef.carbs),
				Fat:           round2(ef.fat),
				Fiber:         round2(ef.fiber),
				Sugar:         round2(ef.sugar),
				Sodium:        round2(ef.sodium),
			}

			// Upsert food by FDC ID
			var existing models.Food
			err := tx.Unscoped().Where("fdc_id = ?", fdcID).First(&existing).Error
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("find food fdc_id=%d: %w", fdcID, err)
				}
				// Insert
				if result := tx.Create(&food); result.Error != nil {
					return fmt.Errorf("create food fdc_id=%d: %w", fdcID, result.Error)
				}
				newFoods++
			} else {
				// Update
				food.ID = existing.ID
				food.CreatedAt = existing.CreatedAt
				// food.DeletedAt implicitly zero from init, so Save() will restore if deleted
				if result := tx.Unscoped().Save(&food); result.Error != nil {
					return fmt.Errorf("update food fdc_id=%d: %w", fdcID, result.Error)
				}
			}

			// Upsert food_nutrients for Level 1 nutrients
			for usdaID, amount := range ef.nutrientsPer100g {
				code := strconv.Itoa(usdaID)
				nutrient, ok := nutrientLookup[code]
				if !ok {
					continue // Nutrient not seeded
				}

				foodNutrient := models.FoodNutrient{
					FoodID:        food.ID,
					NutrientID:    nutrient.ID,
					AmountPer100g: round4(amount),
				}

				err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "food_id"}, {Name: "nutrient_id"}},
					// GORM maps AmountPer100g -> amount_per100g (not amount_per_100g) unless an explicit column tag is set.
					DoUpdates: clause.AssignmentColumns([]string{"amount_per100g", "updated_at"}),
				}).Create(&foodNutrient).Error
				if err != nil {
					// Fallback: try Where+Assign+FirstOrCreate if the constraint name doesn't match
					err = tx.Where("food_id = ? AND nutrient_id = ?", food.ID, nutrient.ID).
						Assign(models.FoodNutrient{AmountPer100g: round4(amount)}).
						FirstOrCreate(&foodNutrient).Error
					if err != nil {
						return fmt.Errorf("upsert food_nutrient food=%s nutrient=%s: %w", food.ID, nutrient.Code, err)
					}
				}
				nutrientRows++
			}
		}

		return nil
	})

	return newFoods, nutrientRows, err
}

// --- JSON Streaming Decoder (ported from standalone app) ---

// decodeFoundationFoods streams the USDA JSON file and calls handle for each food item.
// The JSON structure is: {"FoundationFoods": [{...}, {...}, ...]}
func decodeFoundationFoods(decoder *json.Decoder, handle func(usdaRawFood) error) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("read JSON root: %w", err)
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return fmt.Errorf("unexpected JSON root token %v", token)
	}

	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("read JSON key: %w", err)
		}

		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("unexpected JSON key token %v", keyToken)
		}

		if key != "FoundationFoods" {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return fmt.Errorf("skip JSON field %q: %w", key, err)
			}
			continue
		}

		arrayToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("read FoundationFoods token: %w", err)
		}

		arrayDelim, ok := arrayToken.(json.Delim)
		if !ok || arrayDelim != '[' {
			return fmt.Errorf("unexpected FoundationFoods token %v", arrayToken)
		}

		for decoder.More() {
			var food usdaRawFood
			if err := decoder.Decode(&food); err != nil {
				return fmt.Errorf("decode food item: %w", err)
			}
			if err := handle(food); err != nil {
				return err
			}
		}

		closingToken, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("close FoundationFoods array: %w", err)
		}

		closingDelim, ok := closingToken.(json.Delim)
		if !ok || closingDelim != ']' {
			return fmt.Errorf("unexpected FoundationFoods closing token %v", closingToken)
		}
	}

	_, err = decoder.Token()
	if err != nil {
		return fmt.Errorf("close JSON root: %w", err)
	}

	return nil
}

// --- Helpers ---

// round2 rounds to 2 decimal places.
func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// round4 rounds to 4 decimal places.
func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
