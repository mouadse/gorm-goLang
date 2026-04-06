package services

import (
	"testing"

	"github.com/google/uuid"
)

func TestCalculateEstimated1RM(t *testing.T) {
	tests := []struct {
		name     string
		weight   float64
		reps     int
		expected float64
	}{
		{"1 rep max", 100, 1, 100},
		{"5 reps", 100, 5, 116.67},
		{"10 reps", 100, 10, 133.33},
		{"zero weight", 0, 10, 0},
		{"zero reps", 100, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateEstimated1RM(tt.weight, tt.reps)
			// Allow for floating point precision
			delta := 0.01
			if tt.expected > 0 && (result < tt.expected-delta || result > tt.expected+delta*100) {
				t.Errorf("CalculateEstimated1RM(%f, %d) = %f, want approximately %f", tt.weight, tt.reps, result, tt.expected)
			}
		})
	}
}

func TestCalculateSetVolume(t *testing.T) {
	tests := []struct {
		name     string
		weight   float64
		reps     int
		expected float64
	}{
		{"standard set", 100, 10, 1000},
		{"heavy single", 200, 1, 200},
		{"zero weight", 0, 10, 0},
		{"zero reps", 100, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSetVolume(tt.weight, tt.reps)
			if result != tt.expected {
				t.Errorf("CalculateSetVolume(%f, %d) = %f, want %f", tt.weight, tt.reps, result, tt.expected)
			}
		})
	}
}

func TestFindHeaviestSet(t *testing.T) {
	tests := []struct {
		name         string
		sets         []WorkoutSetData
		expectFound  bool
		expectWeight float64
	}{
		{
			name:         "empty sets",
			sets:         []WorkoutSetData{},
			expectFound:  false,
			expectWeight: 0,
		},
		{
			name: "single completed set",
			sets: []WorkoutSetData{
				{Weight: 100, Reps: 10, Completed: true},
			},
			expectFound:  true,
			expectWeight: 100,
		},
		{
			name: "multiple sets",
			sets: []WorkoutSetData{
				{Weight: 80, Reps: 10, Completed: true},
				{Weight: 100, Reps: 8, Completed: true},
				{Weight: 90, Reps: 9, Completed: true},
			},
			expectFound:  true,
			expectWeight: 100,
		},
		{
			name: "skip incomplete sets",
			sets: []WorkoutSetData{
				{Weight: 120, Reps: 5, Completed: false},
				{Weight: 100, Reps: 8, Completed: true},
			},
			expectFound:  true,
			expectWeight: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			heaviest, found := FindHeaviestSet(tt.sets)
			if found != tt.expectFound {
				t.Errorf("FindHeaviestSet() found = %v, want %v", found, tt.expectFound)
			}
			if found && heaviest.Weight != tt.expectWeight {
				t.Errorf("FindHeaviestSet() weight = %f, want %f", heaviest.Weight, tt.expectWeight)
			}
		})
	}
}

func TestFindBest1RM(t *testing.T) {
	sets := []WorkoutSetData{
		{Weight: 100, Reps: 10, Completed: true}, // 1RM ≈ 133
		{Weight: 120, Reps: 5, Completed: true},  // 1RM ≈ 140
		{Weight: 140, Reps: 3, Completed: true},  // 1RM ≈ 154
	}

	best1RM, found := FindBest1RM(sets)
	if !found {
		t.Error("FindBest1RM() should find a result")
	}
	// The 140x3 set should have highest estimated 1RM
	if best1RM < 150 {
		t.Errorf("FindBest1RM() = %f, expected >150", best1RM)
	}
}

func TestCalculateExerciseSessionVolume(t *testing.T) {
	sets := []WorkoutSetData{
		{Weight: 100, Reps: 10, Completed: true}, // 1000
		{Weight: 120, Reps: 8, Completed: true},  // 960
		{Weight: 140, Reps: 5, Completed: false}, // skipped
	}

	volume := CalculateExerciseSessionVolume(sets)
	expected := float64(1000 + 960) // 1960

	if volume != expected {
		t.Errorf("CalculateExerciseSessionVolume() = %f, want %f", volume, expected)
	}
}

func TestCalculateWorkoutVolume(t *testing.T) {
	exercises := []WorkoutExerciseSession{
		{
			ExerciseID: uuid.New(),
			Sets: []WorkoutSetData{
				{Weight: 100, Reps: 10, Completed: true},
				{Weight: 120, Reps: 8, Completed: true},
			},
		},
		{
			ExerciseID: uuid.New(),
			Sets: []WorkoutSetData{
				{Weight: 50, Reps: 15, Completed: true},
			},
		},
	}

	volume := CalculateWorkoutVolume(exercises)
	expected := float64(100*10 + 120*8 + 50*15) // 1000 + 960 + 750 = 2710

	if volume != expected {
		t.Errorf("CalculateWorkoutVolume() = %f, want %f", volume, expected)
	}
}

func TestCalculateTDEE(t *testing.T) {
	tests := []struct {
		name          string
		weight        float64
		height        float64
		age           int
		activityLevel ActivityLevel
		expectRange   [2]int // min, max
	}{
		{
			name:          "sedentary male",
			weight:        80,
			height:        180,
			age:           30,
			activityLevel: ActivitySedentary,
			expectRange:   [2]int{1800, 2200},
		},
		{
			name:          "active male",
			weight:        80,
			height:        180,
			age:           30,
			activityLevel: ActivityActive,
			expectRange:   [2]int{2600, 3200},
		},
		{
			name:          "very active",
			weight:        70,
			height:        175,
			age:           25,
			activityLevel: ActivityVeryActive,
			expectRange:   [2]int{2800, 3400},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdee := CalculateTDEE(tt.weight, tt.height, tt.age, tt.activityLevel)
			if tdee < tt.expectRange[0] || tdee > tt.expectRange[1] {
				t.Errorf("CalculateTDEE() = %d, want between %d and %d", tdee, tt.expectRange[0], tt.expectRange[1])
			}
		})
	}
}

func TestCalculateNutritionTargets(t *testing.T) {
	tests := []struct {
		name               string
		input              NutritionTargetInput
		expectCalorieRange [2]int
		expectProteinRange [2]int
	}{
		{
			name: "muscle gain",
			input: NutritionTargetInput{
				Goal:          GoalBuildMuscle,
				Weight:        80,
				Height:        180,
				Age:           30,
				ActivityLevel: ActivityModeratelyActive,
			},
			expectCalorieRange: [2]int{2500, 3200},
			expectProteinRange: [2]int{160, 200},
		},
		{
			name: "fat loss",
			input: NutritionTargetInput{
				Goal:          GoalLoseFat,
				Weight:        90,
				Height:        175,
				Age:           35,
				ActivityLevel: ActivityLightlyActive,
			},
			expectCalorieRange: [2]int{1600, 2200},
			expectProteinRange: [2]int{200, 250},
		},
		{
			name: "maintenance",
			input: NutritionTargetInput{
				Goal:          GoalMaintain,
				Weight:        75,
				Height:        170,
				Age:           28,
				ActivityLevel: ActivitySedentary,
			},
			expectCalorieRange: [2]int{1800, 2300},
			expectProteinRange: [2]int{120, 160},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := CalculateNutritionTargets(tt.input)
			if targets.Calories < tt.expectCalorieRange[0] || targets.Calories > tt.expectCalorieRange[1] {
				t.Errorf("CalculateNutritionTargets() calories = %d, want between %d and %d",
					targets.Calories, tt.expectCalorieRange[0], tt.expectCalorieRange[1])
			}
			if targets.Protein < tt.expectProteinRange[0] || targets.Protein > tt.expectProteinRange[1] {
				t.Errorf("CalculateNutritionTargets() protein = %d, want between %d and %d",
					targets.Protein, tt.expectProteinRange[0], tt.expectProteinRange[1])
			}
		})
	}
}

func TestCalculateMacroPercentages(t *testing.T) {
	tests := []struct {
		name             string
		protein          int
		carbs            int
		fat              int
		expectProteinPct float64
	}{
		{"balanced macros", 150, 250, 67, 26.0}, // 600 + 1000 + 600 = 2200 cal
		{"high protein", 200, 150, 67, 35.0},    // 800 + 600 + 600 = 2000 cal
		{"zero macros", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			percentages := CalculateMacroPercentages(tt.protein, tt.carbs, tt.fat)

			if tt.name == "zero macros" {
				if percentages["protein"] != 0 || percentages["carbs"] != 0 || percentages["fat"] != 0 {
					t.Errorf("CalculateMacroPercentages() should return 0 for zero macros")
				}
				return
			}

			// Allow 5% tolerance
			delta := 5.0
			if percentages["protein"] < tt.expectProteinPct-delta || percentages["protein"] > tt.expectProteinPct+delta {
				t.Errorf("CalculateMacroPercentages() protein pct = %f, want approximately %f",
					percentages["protein"], tt.expectProteinPct)
			}

			// Check total sums to 100
			total := percentages["protein"] + percentages["carbs"] + percentages["fat"]
			if total < 99.0 || total > 101.0 {
				t.Errorf("CalculateMacroPercentages() total = %f, should sum to 100", total)
			}
		})
	}
}

func TestAdjustForWorkout(t *testing.T) {
	baseTargets := &NutritionTargets{
		Calories: 2000,
		Protein:  150,
		Carbs:    200,
		Fat:      67,
	}

	tests := []struct {
		name             string
		workoutType      string
		duration         int
		volume           float64
		expectCalorieAdj int
	}{
		{"cardio 30min", "cardio", 30, 0, 300},
		{"cardio 15min", "cardio", 15, 0, 150},
		{"leg day", "legs", 60, 1000, 200},
		{"push day", "push", 45, 800, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjusted := AdjustForWorkout(baseTargets, tt.workoutType, tt.duration, tt.volume)
			calorieAdj := adjusted.Calories - baseTargets.Calories

			// The calorie adjustment should be at least the expected value
			if calorieAdj < tt.expectCalorieAdj {
				t.Errorf("AdjustForWorkout() calorie adjustment = %d, want at least %d", calorieAdj, tt.expectCalorieAdj)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{"normal email", "Test@Example.COM", "test@example.com", false},
		{"trim whitespace", "  user@example.com  ", "user@example.com", false},
		{"empty string", "", "", true},
		{"whitespace only", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeEmail(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("NormalizeEmail() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("NormalizeEmail() unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("NormalizeEmail() = %q, want %q", result, tt.expected)
				}
			}
		})
	}
}

func TestRequirePassword(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasError bool
	}{
		{"valid password", "password123", false},
		{"too short", "pass", true},
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"exactly 8 chars", "12345678", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RequirePassword(tt.input)
			if tt.hasError && err == nil {
				t.Error("RequirePassword() should return error")
			}
			if !tt.hasError && err != nil {
				t.Errorf("RequirePassword() unexpected error: %v", err)
			}
		})
	}
}

func TestHashAndComparePassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == password {
		t.Error("HashPassword() hash should not equal password")
	}

	if len(hash) < 20 {
		t.Error("HashPassword() hash seems too short")
	}

	// Test correct password
	if err := ComparePassword(hash, password); err != nil {
		t.Errorf("ComparePassword() with correct password: %v", err)
	}

	// Test wrong password
	if err := ComparePassword(hash, "wrongpassword"); err == nil {
		t.Error("ComparePassword() should fail with wrong password")
	}
}

func TestGenerateSecureToken(t *testing.T) {
	token1, err := GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken() error: %v", err)
	}

	if len(token1) < 20 {
		t.Errorf("GenerateSecureToken() token too short: %d", len(token1))
	}

	// Generate another token and verify they're different
	token2, err := GenerateSecureToken()
	if err != nil {
		t.Fatalf("GenerateSecureToken() error: %v", err)
	}

	if token1 == token2 {
		t.Error("GenerateSecureToken() should generate unique tokens")
	}
}

func TestActivityLevelMapping(t *testing.T) {
	// Verify activity multipliers are correct
	expectedMultipliers := map[ActivityLevel]float64{
		ActivitySedentary:        1.2,
		ActivityLightlyActive:    1.375,
		ActivityModeratelyActive: 1.55,
		ActivityActive:           1.725,
		ActivityVeryActive:       1.9,
	}

	for level, expected := range expectedMultipliers {
		actual := ActivityMultipliers[level]
		if actual != expected {
			t.Errorf("ActivityMultipliers[%s] = %f, want %f", level, actual, expected)
		}
	}
}

func TestGoalModifiers(t *testing.T) {
	// Verify goal modifiers are correct
	tests := []struct {
		goal              NutritionGoal
		expectCaloriePct  float64
		expectProteinMult float64
	}{
		{GoalBuildMuscle, 1.10, 2.2},
		{GoalLoseFat, 0.80, 2.4},
		{GoalMaintain, 1.0, 1.8},
	}

	for _, tt := range tests {
		modifier, ok := GoalModifiers[tt.goal]
		if !ok {
			t.Errorf("GoalModifiers[%s] not found", tt.goal)
			continue
		}
		if modifier.CaloriePct != tt.expectCaloriePct {
			t.Errorf("GoalModifiers[%s].CaloriePct = %f, want %f", tt.goal, modifier.CaloriePct, tt.expectCaloriePct)
		}
		if modifier.ProteinMult != tt.expectProteinMult {
			t.Errorf("GoalModifiers[%s].ProteinMult = %f, want %f", tt.goal, modifier.ProteinMult, tt.expectProteinMult)
		}
	}
}
