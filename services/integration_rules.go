// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"fitness-tracker/models"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IntegrationRule represents a single adjustment rule.
type IntegrationRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Applies     bool   `json:"applies"`
	Adjustment  string `json:"adjustment"` // Human-readable description
}

// NutritionAdjustment represents the final adjusted nutrition targets.
type NutritionAdjustment struct {
	BaseCalories     int               `json:"base_calories"`
	AdjustedCalories int               `json:"adjusted_calories"`
	BaseProtein      int               `json:"base_protein"`
	AdjustedProtein  int               `json:"adjusted_protein"`
	BaseCarbs        int               `json:"base_carbs"`
	AdjustedCarbs    int               `json:"adjusted_carbs"`
	BaseFat          int               `json:"base_fat"`
	AdjustedFat      int               `json:"adjusted_fat"`
	CalorieDelta     int               `json:"calorie_delta"`
	ProteinDelta     int               `json:"protein_delta"`
	CarbsDelta       int               `json:"carbs_delta"`
	FatDelta         int               `json:"fat_delta"`
	Rules            []IntegrationRule `json:"rules"`
}

// WorkoutNutritionContext contains all data needed for integration rules.
type WorkoutNutritionContext struct {
	UserID           uuid.UUID
	Date             time.Time
	Goal             string
	WorkoutType      string  // push/pull/legs/cardio/none
	WorkoutDuration  int     // minutes
	WorkoutVolume    float64 // total volume weight
	WorkoutSetsCount int     // number of completed sets
	HasWorkout       bool
	ProteinIntake    float64 // current protein intake for the day
	CalorieIntake    float64 // current calorie intake for the day
	WeeklyVolume     float64 // total volume for the week
	WeeklyWorkouts   int     // number of workouts this week
}

// IntegrationRulesService provides business logic for workout-nutrition integration.
type IntegrationRulesService struct {
	db *gorm.DB
}

// NewIntegrationRulesService creates a new integration rules service.
func NewIntegrationRulesService(db *gorm.DB) *IntegrationRulesService {
	return &IntegrationRulesService{db: db}
}

// ApplyIntegrationRules applies all relevant rules and returns adjusted targets.
// Rule precedence: later rules can modify earlier adjustments.
func (s *IntegrationRulesService) ApplyIntegrationRules(
	ctx WorkoutNutritionContext,
	baseTargets *NutritionTargets,
) *NutritionAdjustment {
	adjustment := &NutritionAdjustment{
		BaseCalories:     baseTargets.Calories,
		BaseProtein:      baseTargets.Protein,
		BaseCarbs:        baseTargets.Carbs,
		BaseFat:          baseTargets.Fat,
		AdjustedCalories: baseTargets.Calories,
		AdjustedProtein:  baseTargets.Protein,
		AdjustedCarbs:    baseTargets.Carbs,
		AdjustedFat:      baseTargets.Fat,
		CalorieDelta:     0,
		ProteinDelta:     0,
		CarbsDelta:       0,
		FatDelta:         0,
		Rules:            []IntegrationRule{},
	}

	// Rule 1: Leg Day Bonus
	// Leg days burn more calories and need more carbs for recovery
	if strings.EqualFold(ctx.WorkoutType, "legs") && ctx.HasWorkout {
		rule := IntegrationRule{
			ID:          "leg_day_bonus",
			Name:        "Leg Day Bonus",
			Description: "Leg days require additional calories for recovery",
			Applies:     true,
			Adjustment:  "+200 calories (mostly carbs)",
		}
		adjustment.AdjustedCalories += 200
		adjustment.AdjustedCarbs += 50 // ~50g carbs = 200 calories
		adjustment.CalorieDelta += 200
		adjustment.CarbsDelta += 50
		adjustment.Rules = append(adjustment.Rules, rule)
	}

	// Rule 2: No Workout Day
	// Reduce calories on rest days while maintaining protein
	if !ctx.HasWorkout {
		rule := IntegrationRule{
			ID:          "rest_day_adjustment",
			Name:        "Rest Day Adjustment",
			Description: "Lower calorie target on non-workout days",
			Applies:     true,
			Adjustment:  "-200 calories, maintain protein",
		}
		adjustment.AdjustedCalories -= 200
		adjustment.AdjustedCarbs -= 50 // Reduce carbs primarily
		adjustment.CalorieDelta -= 200
		adjustment.CarbsDelta -= 50
		adjustment.Rules = append(adjustment.Rules, rule)
	}

	// Rule 3: Cardio Bonus
	// Add calories for cardio sessions
	if strings.EqualFold(ctx.WorkoutType, "cardio") && ctx.HasWorkout {
		var bonus int
		if ctx.WorkoutDuration >= 30 {
			bonus = 300 // 30+ min cardio
		} else if ctx.WorkoutDuration >= 15 {
			bonus = 150 // 15-30 min cardio
		} else {
			bonus = 50 // < 15 min
		}

		rule := IntegrationRule{
			ID:          "cardio_bonus",
			Name:        "Cardio Bonus",
			Description: "Cardio sessions burn additional calories",
			Applies:     true,
			Adjustment:  fmt.Sprintf("+%d calories for %d min cardio", bonus, ctx.WorkoutDuration),
		}

		adjustment.AdjustedCalories += bonus
		adjustment.CalorieDelta += bonus
		adjustment.Rules = append(adjustment.Rules, rule)
	}

	// Rule 4: High Volume Week
	// If weekly volume is high (20+ sets per muscle group), increase calories
	if ctx.WeeklyWorkouts >= 4 && ctx.WeeklyVolume > 10000 { // arbitrary threshold
		bonusPct := int(float64(adjustment.BaseCalories) * 0.15)
		rule := IntegrationRule{
			ID:          "high_volume_week",
			Name:        "High Volume Week",
			Description: "High training volume requires additional recovery calories",
			Applies:     true,
			Adjustment:  "+15% calories for recovery",
		}
		adjustment.AdjustedCalories += bonusPct
		adjustment.CalorieDelta += bonusPct
		adjustment.Rules = append(adjustment.Rules, rule)
	}

	// Rule 5: Recovery Warning
	// Warn if training load is high AND protein is insufficient
	// This is an informational rule, doesn't adjust macros
	if ctx.HasWorkout && ctx.WorkoutVolume > 5000 {
		targetProtein := float64(adjustment.AdjustedProtein)
		proteinGap := targetProtein - ctx.ProteinIntake
		if proteinGap > targetProtein*0.3 {
			rule := IntegrationRule{
				ID:          "recovery_warning",
				Name:        "Recovery Warning",
				Description: "Training load is high but protein intake is low",
				Applies:     true,
				Adjustment:  "Increase protein intake for optimal recovery",
			}
			adjustment.Rules = append(adjustment.Rules, rule)
		}
	}

	// Rule 6: Goal Alignment Check
	// Warn if intake conflicts with stated goal
	// This is an informational rule
	adjustment.Rules = append(adjustment.Rules, checkGoalAlignment(ctx, baseTargets)...)

	return adjustment
}

// checkGoalAlignment validates nutrition against fitness goals.
func checkGoalAlignment(ctx WorkoutNutritionContext, targets *NutritionTargets) []IntegrationRule {
	var rules []IntegrationRule

	switch strings.ToLower(ctx.Goal) {
	case "build_muscle", "bulking", "muscle_gain":
		// For muscle gain, check if eating enough
		if ctx.CalorieIntake < float64(targets.Calories)*0.9 {
			rules = append(rules, IntegrationRule{
				ID:          "muscle_gain_calorie_warning",
				Name:        "Muscle Gain Calorie Warning",
				Description: "Calorie intake is below target for muscle gain",
				Applies:     true,
				Adjustment:  "Eat at or above target for optimal muscle gain",
			})
		}
		if ctx.ProteinIntake < float64(targets.Protein)*0.8 {
			rules = append(rules, IntegrationRule{
				ID:          "muscle_gain_protein_warning",
				Name:        "Muscle Gain Protein Warning",
				Description: "Protein intake is below target for muscle gain",
				Applies:     true,
				Adjustment:  fmt.Sprintf("Aim for %dg+ protein for muscle growth", targets.Protein),
			})
		}

	case "lose_fat", "cutting", "fat_loss", "weight_loss":
		// For fat loss, check if deficit is appropriate
		deficit := float64(targets.Calories) - ctx.CalorieIntake
		if deficit > 1000 {
			rules = append(rules, IntegrationRule{
				ID:          "fat_loss_deficit_warning",
				Name:        "Fat Loss Deficit Warning",
				Description: "Calorie deficit is too large, risking muscle loss",
				Applies:     true,
				Adjustment:  "Keep deficit under 1000 calories to preserve muscle",
			})
		}
		if ctx.ProteinIntake < float64(targets.Protein)*0.9 {
			rules = append(rules, IntegrationRule{
				ID:          "fat_loss_protein_warning",
				Name:        "Fat Loss Protein Warning",
				Description: "Protein intake is critical during fat loss",
				Applies:     true,
				Adjustment:  "Maintain high protein to preserve muscle mass",
			})
		}
	}

	return rules
}

// GetWorkoutNutritionContext builds the context for integration rules.
func (s *IntegrationRulesService) GetWorkoutNutritionContext(userID uuid.UUID, date time.Time) (*WorkoutNutritionContext, error) {
	dayStart := startOfDayUTC(date)
	nextDay := dayStart.AddDate(0, 0, 1)

	// Get user
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	ctx := &WorkoutNutritionContext{
		UserID: userID,
		Date:   date,
		Goal:   user.Goal,
	}

	// Get workout for the date
	var workout models.Workout
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, nextDay).First(&workout).Error
	if err == nil {
		ctx.HasWorkout = true
		ctx.WorkoutType = workout.Type
		ctx.WorkoutDuration = workout.Duration

		// Calculate workout volume
		var workoutExercises []models.WorkoutExercise
		err := s.db.
			Preload("WorkoutSets").
			Where("workout_id = ?", workout.ID).
			Find(&workoutExercises).Error
		if err == nil {
			for _, we := range workoutExercises {
				for _, set := range we.WorkoutSets {
					if set.Completed {
						ctx.WorkoutVolume += set.Weight * float64(set.Reps)
						ctx.WorkoutSetsCount++
					}
				}
			}
		}
	}

	// Get nutrition intake for the date
	var meals []models.Meal
	err = s.db.
		Preload("Items.Food").
		Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, nextDay).
		Find(&meals).Error
	if err == nil {
		for _, meal := range meals {
			meal.CalculateTotals()
			ctx.CalorieIntake += meal.TotalCalories
			ctx.ProteinIntake += meal.TotalProtein
		}
	}

	// Get weekly stats
	weekStart := startOfWeekUTC(dayStart)
	var workouts []models.Workout
	err = s.db.
		Where("user_id = ? AND date >= ? AND date < ?", userID, weekStart, nextDay).
		Find(&workouts).Error
	if err == nil {
		ctx.WeeklyWorkouts = len(workouts)
		workoutIDs := make([]uuid.UUID, 0, len(workouts))
		for _, w := range workouts {
			workoutIDs = append(workoutIDs, w.ID)
		}

		var workoutExercises []models.WorkoutExercise
		if len(workoutIDs) > 0 {
			_ = s.db.Preload("WorkoutSets").Where("workout_id IN ?", workoutIDs).Find(&workoutExercises).Error
		}
		for _, we := range workoutExercises {
			for _, set := range we.WorkoutSets {
				if set.Completed {
					ctx.WeeklyVolume += set.Weight * float64(set.Reps)
				}
			}
		}
	}

	return ctx, nil
}

// GetRecommendations returns nutrition recommendations for the day.
func (s *IntegrationRulesService) GetRecommendations(userID uuid.UUID, date time.Time) (*NutritionAdjustment, error) {
	// Get base targets
	nutritionSvc := NewNutritionTargetService(s.db)
	baseTargets, err := nutritionSvc.GetUserNutritionTargets(userID)
	if err != nil {
		return nil, err
	}

	// Get context
	ctx, err := s.GetWorkoutNutritionContext(userID, date)
	if err != nil {
		return nil, err
	}

	// Apply rules
	return s.ApplyIntegrationRules(*ctx, baseTargets), nil
}
