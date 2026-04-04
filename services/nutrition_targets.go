// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"math"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NutritionGoal represents the user's nutritional goal.
type NutritionGoal string

const (
	GoalBuildMuscle NutritionGoal = "build_muscle"
	GoalLoseFat     NutritionGoal = "lose_fat"
	GoalMaintain    NutritionGoal = "maintain"
)

// ActivityLevel represents the user's activity level.
type ActivityLevel string

const (
	ActivitySedentary        ActivityLevel = "sedentary"
	ActivityLightlyActive    ActivityLevel = "lightly_active"
	ActivityModeratelyActive ActivityLevel = "moderately_active"
	ActivityActive           ActivityLevel = "active"
	ActivityVeryActive       ActivityLevel = "very_active"
)

// NutritionTargets represents calculated nutrition goals.
type NutritionTargets struct {
	Calories      int    `json:"calories"`
	Protein       int    `json:"protein"` // grams
	Carbs         int    `json:"carbs"`   // grams
	Fat           int    `json:"fat"`     // grams
	Fiber         int    `json:"fiber"`   // grams, optional
	Water         int    `json:"water"`   // ml, optional
	Goal          string `json:"goal"`
	ActivityLevel string `json:"activity_level"`
	IsOverride    bool   `json:"is_override"` // true if manually overridden
}

// NutritionTargetInput contains inputs for target calculation.
type NutritionTargetInput struct {
	UserID           uuid.UUID
	Goal             NutritionGoal
	Weight           float64 // kg
	Height           float64 // cm
	Age              int
	ActivityLevel    ActivityLevel
	WorkoutFrequency int     // workouts per week
	BodyFatPct       float64 // optional, for more accurate calculations
}

// DailyNutritionSummary represents a user's daily nutrition intake.
type DailyNutritionSummary struct {
	Date           time.Time `json:"date"`
	TotalCalories  float64   `json:"total_calories"`
	TotalProtein   float64   `json:"total_protein"`
	TotalCarbs     float64   `json:"total_carbs"`
	TotalFat       float64   `json:"total_fat"`
	TotalFiber     float64   `json:"total_fiber"`
	MealCount      int       `json:"meal_count"`
	TargetCalories int       `json:"target_calories"`
	TargetProtein  int       `json:"target_protein"`
	CaloriesDelta  float64   `json:"calories_delta"`
	ProteinDelta   float64   `json:"protein_delta"`
}

// ActivityMultipliers maps activity levels to TDEE multipliers.
var ActivityMultipliers = map[ActivityLevel]float64{
	ActivitySedentary:        1.2,
	ActivityLightlyActive:    1.375,
	ActivityModeratelyActive: 1.55,
	ActivityActive:           1.725,
	ActivityVeryActive:       1.9,
}

// GoalModifiers maps goals to calorie adjustments.
var GoalModifiers = map[NutritionGoal]struct {
	CaloriePct  float64 // percentage adjustment
	ProteinMult float64 // per kg of body weight
}{
	GoalBuildMuscle: {CaloriePct: 1.10, ProteinMult: 2.2}, // +10% calories, 2.2g/kg protein
	GoalLoseFat:     {CaloriePct: 0.80, ProteinMult: 2.4}, // -20% calories, 2.4g/kg protein
	GoalMaintain:    {CaloriePct: 1.0, ProteinMult: 1.8},  // maintain calories, 1.8g/kg protein
}

// NutritionTargetService provides business logic for nutrition target calculations.
type NutritionTargetService struct {
	db *gorm.DB
}

// NewNutritionTargetService creates a new nutrition target service.
func NewNutritionTargetService(db *gorm.DB) *NutritionTargetService {
	return &NutritionTargetService{db: db}
}

// CalculateTDEE calculates Total Daily Energy Expenditure using Mifflin-St Jeor equation.
// BMR = 10 * weight (kg) + 6.25 * height (cm) - 5 * age (y) + 5 (for males, -161 for females)
// Since we don't have gender, we use a neutral average (+5).
func CalculateTDEE(weight, height float64, age int, activityLevel ActivityLevel) int {
	if weight <= 0 || height <= 0 || age <= 0 {
		return 0
	}

	bmr := 10*weight + 6.25*height - 5*float64(age) + 5

	multiplier := ActivityMultipliers[activityLevel]
	if multiplier == 0 {
		multiplier = 1.2 // default tosedentary
	}

	return int(bmr * multiplier)
}

// CalculateNutritionTargets computes macro targets based on user profile and goal.
func CalculateNutritionTargets(input NutritionTargetInput) *NutritionTargets {
	// Calculate base TDEE
	tdee := CalculateTDEE(input.Weight, input.Height, input.Age, input.ActivityLevel)

	// Get goal modifier
	modifier, ok := GoalModifiers[input.Goal]
	if !ok {
		modifier = GoalModifiers[GoalMaintain]
	}

	// Adjust calories for goal
	calories := int(float64(tdee) * modifier.CaloriePct)

	// Calculate protein based on body weight and goal
	protein := int(input.Weight * modifier.ProteinMult)

	// Allocate remaining calories to carbs and fat
	// Protein calories = protein * 4
	// Fat calories = (totalCalories - proteinCalories) * fatRatio
	// Carb calories = (totalCalories - proteinCalories) * (1 - fatRatio)
	// Using 30% of remaining calories for fat (standard split)
	proteinCalories := float64(protein) * 4
	fatCalories := (float64(calories) - proteinCalories) * 0.30
	carbCalories := float64(calories) - proteinCalories - fatCalories

	fat := int(fatCalories / 9)    // 9 calories per gram of fat
	carbs := int(carbCalories / 4) // 4 calories per gram of carbs

	// Calculate fiber (14g per 1000 calories, recommended minimum)
	fiber := int(float64(calories) / 1000 * 14)

	// Calculate water intake (30ml per kg body weight)
	water := int(input.Weight * 30)

	// Ensure non-negative values
	if calories < 0 {
		calories = 0
	}
	if protein < 0 {
		protein = 0
	}
	if carbs < 0 {
		carbs = 0
	}
	if fat < 0 {
		fat = 0
	}

	return &NutritionTargets{
		Calories:      calories,
		Protein:       protein,
		Carbs:         carbs,
		Fat:           fat,
		Fiber:         fiber,
		Water:         water,
		Goal:          string(input.Goal),
		ActivityLevel: string(input.ActivityLevel),
		IsOverride:    false,
	}
}

// GetUserNutritionTargets retrieves or calculates nutrition targets for a user.
func (s *NutritionTargetService) GetUserNutritionTargets(userID uuid.UUID) (*NutritionTargets, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	// Calculate age from date of birth
	age := user.Age
	if user.DateOfBirth != nil {
		now := time.Now().UTC()
		age = now.Year() - user.DateOfBirth.Year()
		if now.Month() < user.DateOfBirth.Month() ||
			(now.Month() == user.DateOfBirth.Month() && now.Day() < user.DateOfBirth.Day()) {
			age--
		}
	}

	// Parse goal
	var goal NutritionGoal
	switch strings.ToLower(user.Goal) {
	case "build_muscle", "bulking", "muscle_gain":
		goal = GoalBuildMuscle
	case "lose_fat", "cutting", "fat_loss", "weight_loss":
		goal = GoalLoseFat
	default:
		goal = GoalMaintain
	}

	// Parse activity level
	var activityLevel ActivityLevel
	switch strings.ToLower(user.ActivityLevel) {
	case "sedentary":
		activityLevel = ActivitySedentary
	case "lightly_active", "lightly active":
		activityLevel = ActivityLightlyActive
	case "moderately_active", "moderately active":
		activityLevel = ActivityModeratelyActive
	case "active":
		activityLevel = ActivityActive
	case "very_active", "very active":
		activityLevel = ActivityVeryActive
	default:
		activityLevel = ActivitySedentary
	}

	input := NutritionTargetInput{
		UserID:        userID,
		Goal:          goal,
		Weight:        user.Weight,
		Height:        user.Height,
		Age:           age,
		ActivityLevel: activityLevel,
	}

	targets := CalculateNutritionTargets(input)

	// Ifuser has a manual TDEE override, use it
	if user.TDEE > 0 && targets.Calories > 0 {
		// Scale all macros proportionally
		scaleFactor := float64(user.TDEE) / float64(targets.Calories)
		targets.Calories = user.TDEE
		targets.Protein = int(float64(targets.Protein) * scaleFactor)
		targets.Carbs = int(float64(targets.Carbs) * scaleFactor)
		targets.Fat = int(float64(targets.Fat) * scaleFactor)
		targets.IsOverride = true
	}

	return targets, nil
}

// GetDailyNutritionSummary calculates daily nutrition totals from meals.
func (s *NutritionTargetService) GetDailyNutritionSummary(userID uuid.UUID, date time.Time) (*DailyNutritionSummary, error) {
	dayStart := startOfDayUTC(date)
	nextDay := dayStart.AddDate(0, 0, 1)

	// Get user for targets
	targets, err := s.GetUserNutritionTargets(userID)
	if err != nil {
		return nil, err
	}

	// Get meals for the date
	var meals []models.Meal
	err = s.db.
		Preload("Items.Food").
		Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, nextDay).
		Find(&meals).Error
	if err != nil {
		return nil, err
	}

	summary := &DailyNutritionSummary{
		Date:           date,
		TargetCalories: targets.Calories,
		TargetProtein:  targets.Protein,
		MealCount:      len(meals),
	}

	for _, meal := range meals {
		meal.CalculateTotals()
		summary.TotalCalories += meal.TotalCalories
		summary.TotalProtein += meal.TotalProtein
		summary.TotalCarbs += meal.TotalCarbs
		summary.TotalFat += meal.TotalFat
		summary.TotalFiber += meal.TotalFiber
	}

	summary.CaloriesDelta = summary.TotalCalories - float64(summary.TargetCalories)
	summary.ProteinDelta = summary.TotalProtein - float64(summary.TargetProtein)

	return summary, nil
}

// AdjustForWorkout modifies nutrition targets based on workout activity.
// This is a helper for the integration rules service.
func AdjustForWorkout(baseTargets *NutritionTargets, workoutType string, durationMinutes int, volume float64) *NutritionTargets {
	adjusted := *baseTargets

	// Additional calories based on workout type and duration
	// These are baseline adjustments that integration rules can use
	var calorieAdjustment int

	switch strings.ToLower(workoutType) {
	case "cardio":
		// ~10 calories per minute for moderate cardio
		calorieAdjustment = durationMinutes * 10
	case "push", "pull", "legs":
		// Resistance training: base adjustment per workout type
		if strings.ToLower(workoutType) == "legs" {
			calorieAdjustment = 200 // Leg days burn more
		} else {
			calorieAdjustment = 150
		}
		// Add volume-based adjustment
		if volume > 0 {
			calorieAdjustment += int(volume / 100) // +1 calorie per 100kg volume
		}
	default:
		calorieAdjustment = 100
	}

	adjusted.Calories += calorieAdjustment

	// Increase carbs for replenishment (50% of extra calories)
	carbIncrease := int(float64(calorieAdjustment) * 0.5 / 4)
	adjusted.Carbs += carbIncrease

	return &adjusted
}

// CalculateMacroPercentages returns the macro ratio as percentages.
func CalculateMacroPercentages(protein, carbs, fat int) map[string]float64 {
	totalCalories := float64(protein)*4 + float64(carbs)*4 + float64(fat)*9
	if totalCalories == 0 {
		return map[string]float64{"protein": 0, "carbs": 0, "fat": 0}
	}

	return map[string]float64{
		"protein": math.Round(float64(protein)*4/totalCalories*100*10) / 10,
		"carbs":   math.Round(float64(carbs)*4/totalCalories*100*10) / 10,
		"fat":     math.Round(float64(fat)*9/totalCalories*100*10) / 10,
	}
}
