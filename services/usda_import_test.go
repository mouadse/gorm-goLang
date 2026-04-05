package services

import (
	"encoding/json"
	"strings"
	"testing"
)

const testUSDADataset = `{
  "FoundationFoods": [
    {
      "fdcId": 101,
      "description": "Red Delicious Apple, raw",
      "foodCategory": { "description": "Fruits" },
      "foodPortions": [
        {
          "id": 1001,
          "value": 1,
          "measureUnit": { "name": "medium" },
          "modifier": "",
          "gramWeight": 182,
          "amount": 1
        }
      ],
      "foodNutrients": [
        { "amount": 52, "nutrient": { "id": 1008 } },
        { "amount": 0.3, "nutrient": { "id": 1003 } },
        { "amount": 13.8, "nutrient": { "id": 1005 } },
        { "amount": 0.2, "nutrient": { "id": 1004 } },
        { "amount": 2.4, "nutrient": { "id": 1079 } },
        { "amount": 10.4, "nutrient": { "id": 1063 } },
        { "amount": 1.0, "nutrient": { "id": 1093 } },
        { "amount": 6.0, "nutrient": { "id": 1087 } },
        { "amount": 0.12, "nutrient": { "id": 1089 } },
        { "amount": 4.6, "nutrient": { "id": 1162 } }
      ]
    },
    {
      "fdcId": 202,
      "description": "Chicken breast, roasted",
      "foodCategory": { "description": "Poultry Products" },
      "foodPortions": [],
      "foodNutrients": [
        { "amount": 165, "nutrient": { "id": 1008 } },
        { "amount": 31.0, "nutrient": { "id": 1003 } },
        { "amount": 0.0, "nutrient": { "id": 1005 } },
        { "amount": 3.6, "nutrient": { "id": 1004 } }
      ]
    },
    {
      "fdcId": 303,
      "description": "Pear, raw",
      "foodCategory": { "description": "Fruits" },
      "foodPortions": [],
      "foodNutrients": [
        { "amount": 57, "nutrient": { "id": 1008 } },
        { "amount": 0.4, "nutrient": { "id": 1003 } },
        { "amount": 15.2, "nutrient": { "id": 1005 } },
        { "amount": 0.1, "nutrient": { "id": 1004 } }
      ]
    }
  ]
}`

func TestDecodeFoundationFoodsStreaming(t *testing.T) {
	t.Parallel()

	decoder := json.NewDecoder(strings.NewReader(testUSDADataset))
	foods := make([]usdaRawFood, 0, 3)

	err := decodeFoundationFoods(decoder, func(food usdaRawFood) error {
		foods = append(foods, food)
		return nil
	})
	if err != nil {
		t.Fatalf("decodeFoundationFoods() error = %v", err)
	}

	if len(foods) != 3 {
		t.Fatalf("expected 3 foods, got %d", len(foods))
	}

	if foods[0].Description != "Red Delicious Apple, raw" {
		t.Errorf("unexpected first food description: %q", foods[0].Description)
	}

	if foods[1].FdcID != 202 {
		t.Errorf("unexpected second food FDC ID: %d", foods[1].FdcID)
	}

	if foods[2].FoodCategory.Description != "Fruits" {
		t.Errorf("unexpected third food category: %q", foods[2].FoodCategory.Description)
	}
}

func TestExtractFoodNutrients(t *testing.T) {
	t.Parallel()

	decoder := json.NewDecoder(strings.NewReader(testUSDADataset))
	var raw usdaRawFood

	err := decodeFoundationFoods(decoder, func(food usdaRawFood) error {
		raw = food
		return errTestStop
	})
	if err != nil && err != errTestStop {
		t.Fatalf("decodeFoundationFoods() error = %v", err)
	}

	extracted := extractFood(raw)

	if extracted.name != "Red Delicious Apple, raw" {
		t.Errorf("name = %q", extracted.name)
	}

	if extracted.category != "Fruits" {
		t.Errorf("category = %q", extracted.category)
	}

	if extracted.calories != 94.64 {
		t.Errorf("calories = %v, want 94.64", extracted.calories)
	}

	if extracted.protein != 0.546 {
		t.Errorf("protein = %v, want 0.546", extracted.protein)
	}

	if extracted.carbs != 25.116000000000003 {
		t.Errorf("carbs = %v, want 25.116000000000003", extracted.carbs)
	}

	if extracted.fat != 0.36400000000000005 {
		t.Errorf("fat = %v, want 0.36400000000000005", extracted.fat)
	}

	if extracted.fiber != 4.368 {
		t.Errorf("fiber = %v, want 4.368", extracted.fiber)
	}

	if extracted.sugar != 18.928 {
		t.Errorf("sugar = %v, want 18.928", extracted.sugar)
	}

	if extracted.sodium != 1.82 {
		t.Errorf("sodium = %v, want 1.82", extracted.sodium)
	}

	// Should have the per-100g nutrient map populated
	if len(extracted.nutrientsPer100g) == 0 {
		t.Error("nutrientsPer100g is empty")
	}

	// Calcium (USDA ID 1087 = 6.0 mg)
	if v, ok := extracted.nutrientsPer100g[1087]; !ok || v != 6.0 {
		t.Errorf("calcium per 100g = %v, want 6.0", v)
	}

	// Iron (USDA ID 1089 = 0.12 mg)
	if v, ok := extracted.nutrientsPer100g[1089]; !ok || v != 0.12 {
		t.Errorf("iron per 100g = %v, want 0.12", v)
	}

	// Vitamin C (USDA ID 1162 = 4.6 mg)
	if v, ok := extracted.nutrientsPer100g[1162]; !ok || v != 4.6 {
		t.Errorf("vitamin C per 100g = %v, want 4.6", v)
	}
}

func TestExtractFoodServingFromPortion(t *testing.T) {
	t.Parallel()

	decoder := json.NewDecoder(strings.NewReader(testUSDADataset))
	var apple usdaRawFood

	err := decodeFoundationFoods(decoder, func(food usdaRawFood) error {
		apple = food
		return errTestStop
	})
	if err != nil && err != errTestStop {
		t.Fatalf("unexpected error: %v", err)
	}

	extracted := extractFood(apple)

	// Apple has a portion: 1 medium = 182g
	if extracted.servingSize != 182 {
		t.Errorf("serving size = %v, want 182", extracted.servingSize)
	}

	if extracted.servingUnit != "medium" {
		t.Errorf("serving unit = %q, want \"medium\"", extracted.servingUnit)
	}
}

func TestExtractFoodDefaultServing(t *testing.T) {
	t.Parallel()

	// Chicken breast has no portions — should default to 100g
	decoder := json.NewDecoder(strings.NewReader(testUSDADataset))
	var chicken usdaRawFood

	count := 0
	err := decodeFoundationFoods(decoder, func(food usdaRawFood) error {
		count++
		if count == 2 {
			chicken = food
			return errTestStop
		}
		return nil
	})
	if err != nil && err != errTestStop {
		t.Fatalf("unexpected error: %v", err)
	}

	extracted := extractFood(chicken)

	if extracted.servingSize != 100 {
		t.Errorf("serving size = %v, want 100 (default)", extracted.servingSize)
	}

	if extracted.servingUnit != "g" {
		t.Errorf("serving unit = %q, want \"g\" (default)", extracted.servingUnit)
	}
}

func TestExtractFoodAtwaterCalorieFallback(t *testing.T) {
	t.Parallel()

	// Test that Atwater energy values are used when standard calories are 0
	raw := usdaRawFood{
		FdcID:       900,
		Description: "Test food",
		FoodCategory: usdaRawFoodCategory{Description: "Test"},
		FoodNutrients: []usdaRawFoodNutrient{
			{Amount: 0, Nutrient: usdaRawNutrient{ID: usdaNutrientCalories}},
			{Amount: 250, Nutrient: usdaRawNutrient{ID: usdaNutrientEnergyAtwaterGeneral}},
		},
	}

	extracted := extractFood(raw)

	if extracted.calories != 250 {
		t.Errorf("calories = %v, want 250 (Atwater general fallback)", extracted.calories)
	}
}

func TestDecodeFoundationFoodsSkipsEmptyFoods(t *testing.T) {
	t.Parallel()

	dataset := `{
		"FoundationFoods": [
			{"fdcId": 0, "description": "Should be skipped", "foodCategory": {}, "foodNutrients": [], "foodPortions": []},
			{"fdcId": 1, "description": "", "foodCategory": {}, "foodNutrients": [], "foodPortions": []},
			{"fdcId": 2, "description": "   ", "foodCategory": {}, "foodNutrients": [], "foodPortions": []},
			{"fdcId": 3, "description": "Valid food", "foodCategory": {"description": "Test"}, "foodNutrients": [], "foodPortions": []}
		]
	}`

	decoder := json.NewDecoder(strings.NewReader(dataset))
	var foods []usdaRawFood

	err := decodeFoundationFoods(decoder, func(food usdaRawFood) error {
		foods = append(foods, food)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The decoder itself returns all foods; filtering happens in ImportFromFile.
	// But we still test the decoder handles them without errors.
	if len(foods) != 4 {
		t.Errorf("expected 4 decoded foods (filtering happens at import level), got %d", len(foods))
	}
}

func TestRound2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    float64
		expected float64
	}{
		{0.0, 0.0},
		{1.005, 1.0},   // IEEE-754: 1.005 is stored as 1.00499... → rounds to 1.00
		{1.006, 1.01},  // 1.006 rounds up to 1.01
		{3.14159, 3.14},
		{52.0, 52.0},
		{0.125, 0.13},
		{165.0, 165.0},
		{0.3, 0.3},
	}

	for _, tt := range tests {
		if got := round2(tt.input); got != tt.expected {
			t.Errorf("round2(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestRound4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    float64
		expected float64
	}{
		{0.0, 0.0},
		{0.12345, 0.1235},
		{6.12340, 6.1234},
	}

	for _, tt := range tests {
		if got := round4(tt.input); got != tt.expected {
			t.Errorf("round4(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

var errTestStop = &testStopError{}

type testStopError struct{}

func (*testStopError) Error() string { return "stop" }
