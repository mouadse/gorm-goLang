package services

import (
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCoachServiceDispatchFunctionUsesUserDataTools(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Exercise{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.Food{},
		&models.Meal{},
		&models.MealFood{},
		&models.WeightEntry{},
	)
	if err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	userID := uuid.New()
	user := models.User{
		ID:            userID,
		Email:         "alex@example.com",
		PasswordHash:  "hash",
		Name:          "Alex Johnson",
		Age:           30,
		Weight:        80,
		Height:        180,
		Goal:          "build_muscle",
		ActivityLevel: "moderately_active",
		TDEE:          2600,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	exercise := models.Exercise{
		ID:             uuid.New(),
		Name:           "Barbell Row",
		Level:          "intermediate",
		Equipment:      "barbell",
		Category:       "strength",
		PrimaryMuscles: "back",
		Instructions:   "Row the bar with control.",
	}
	if err := db.Create(&exercise).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	workoutDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	olderWorkout := models.Workout{
		ID:        uuid.New(),
		UserID:    userID,
		Date:      workoutDate.AddDate(0, 0, -2),
		Duration:  40,
		Type:      "push",
		Notes:     "Push day",
		CreatedAt: workoutDate.Add(-48 * time.Hour),
	}
	latestWorkout := models.Workout{
		ID:        uuid.New(),
		UserID:    userID,
		Date:      workoutDate,
		Duration:  55,
		Type:      "pull",
		Notes:     "Pull day",
		CreatedAt: workoutDate,
	}
	if err := db.Create(&olderWorkout).Error; err != nil {
		t.Fatalf("create older workout: %v", err)
	}
	if err := db.Create(&latestWorkout).Error; err != nil {
		t.Fatalf("create latest workout: %v", err)
	}

	workoutExercise := models.WorkoutExercise{
		ID:         uuid.New(),
		WorkoutID:  latestWorkout.ID,
		ExerciseID: exercise.ID,
		Order:      1,
		Sets:       3,
		Reps:       8,
		Weight:     70,
		RestTime:   90,
	}
	if err := db.Create(&workoutExercise).Error; err != nil {
		t.Fatalf("create workout exercise: %v", err)
	}

	workoutSet := models.WorkoutSet{
		ID:                uuid.New(),
		WorkoutExerciseID: workoutExercise.ID,
		SetNumber:         1,
		Reps:              8,
		Weight:            70,
		Completed:         true,
	}
	if err := db.Create(&workoutSet).Error; err != nil {
		t.Fatalf("create workout set: %v", err)
	}

	food := models.Food{
		ID:            uuid.New(),
		Name:          "Chicken Breast",
		Source:        "user",
		ServingSize:   1,
		ServingUnit:   "serving",
		Calories:      220,
		Protein:       31,
		Carbohydrates: 0,
		Fat:           5,
	}
	if err := db.Create(&food).Error; err != nil {
		t.Fatalf("create food: %v", err)
	}

	meal := models.Meal{
		ID:        uuid.New(),
		UserID:    userID,
		MealType:  "dinner",
		Date:      workoutDate,
		Notes:     "Post workout dinner",
		CreatedAt: workoutDate,
	}
	if err := db.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	mealFood := models.MealFood{
		ID:       uuid.New(),
		MealID:   meal.ID,
		FoodID:   food.ID,
		Quantity: 1,
	}
	if err := db.Create(&mealFood).Error; err != nil {
		t.Fatalf("create meal food: %v", err)
	}

	weightEntry := models.WeightEntry{
		ID:        uuid.New(),
		UserID:    userID,
		Weight:    79.5,
		Date:      workoutDate,
		CreatedAt: workoutDate,
	}
	if err := db.Create(&weightEntry).Error; err != nil {
		t.Fatalf("create weight entry: %v", err)
	}

	svc := NewCoachService(
		db,
		NewWorkoutAnalyticsService(db),
		NewAdherenceService(db),
		NewNutritionTargetService(db),
		NewIntegrationRulesService(db),
		NewNotificationService(db),
		nil,
	)

	profileResult, err := svc.DispatchFunction("get_user", "{}", userID)
	if err != nil {
		t.Fatalf("dispatch get_user: %v", err)
	}
	profile, ok := profileResult.(*models.User)
	if !ok {
		t.Fatalf("expected *models.User, got %T", profileResult)
	}
	if profile.Name != "Alex Johnson" {
		t.Fatalf("expected name Alex Johnson, got %q", profile.Name)
	}
	if profile.TDEE != 2600 {
		t.Fatalf("expected TDEE 2600, got %d", profile.TDEE)
	}

	workoutsResult, err := svc.DispatchFunction("get_user_workouts", `{"limit":1}`, userID)
	if err != nil {
		t.Fatalf("dispatch get_user_workouts: %v", err)
	}
	workouts, ok := workoutsResult.([]models.Workout)
	if !ok {
		t.Fatalf("expected []models.Workout, got %T", workoutsResult)
	}
	if len(workouts) != 1 || workouts[0].ID != latestWorkout.ID {
		t.Fatalf("expected latest workout first, got %+v", workouts)
	}

	workoutResult, err := svc.DispatchFunction("get_workout", `{"workout_id":"`+latestWorkout.ID.String()+`"}`, userID)
	if err != nil {
		t.Fatalf("dispatch get_workout: %v", err)
	}
	workout, ok := workoutResult.(*models.Workout)
	if !ok {
		t.Fatalf("expected *models.Workout, got %T", workoutResult)
	}
	if len(workout.WorkoutExercises) != 1 {
		t.Fatalf("expected 1 workout exercise, got %d", len(workout.WorkoutExercises))
	}
	if workout.WorkoutExercises[0].Exercise.Name != "Barbell Row" {
		t.Fatalf("expected Barbell Row, got %q", workout.WorkoutExercises[0].Exercise.Name)
	}

	mealsResult, err := svc.DispatchFunction("get_user_meals", `{"limit":1}`, userID)
	if err != nil {
		t.Fatalf("dispatch get_user_meals: %v", err)
	}
	meals, ok := mealsResult.([]models.Meal)
	if !ok {
		t.Fatalf("expected []models.Meal, got %T", mealsResult)
	}
	if len(meals) != 1 || meals[0].TotalProtein != 31 {
		t.Fatalf("expected meal total protein 31, got %+v", meals)
	}

	weightEntriesResult, err := svc.DispatchFunction("get_user_weight_entries", `{"limit":1}`, userID)
	if err != nil {
		t.Fatalf("dispatch get_user_weight_entries: %v", err)
	}
	entries, ok := weightEntriesResult.([]models.WeightEntry)
	if !ok {
		t.Fatalf("expected []models.WeightEntry, got %T", weightEntriesResult)
	}
	if len(entries) != 1 || entries[0].Weight != 79.5 {
		t.Fatalf("expected latest weight entry, got %+v", entries)
	}

	weeklySummaryResult, err := svc.DispatchFunction("get_weekly_summary", `{"date":"2026-04-14"}`, userID)
	if err != nil {
		t.Fatalf("dispatch get_weekly_summary: %v", err)
	}
	weeklySummary, ok := weeklySummaryResult.(*CoachWeeklySummary)
	if !ok {
		t.Fatalf("expected *CoachWeeklySummary, got %T", weeklySummaryResult)
	}
	if weeklySummary.WorkoutCount != 2 {
		t.Fatalf("expected 2 workouts in weekly summary, got %d", weeklySummary.WorkoutCount)
	}
	if weeklySummary.MealCount != 1 {
		t.Fatalf("expected 1 meal in weekly summary, got %d", weeklySummary.MealCount)
	}
	if weeklySummary.TargetCalories <= 0 {
		t.Fatalf("expected positive weekly calorie target, got %d", weeklySummary.TargetCalories)
	}
}

func TestCoachServiceGetToolsIncludesProfileAndHistoryTools(t *testing.T) {
	svc := &CoachService{}
	tools := svc.GetTools()

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}

	required := []string{
		"get_user",
		"get_user_workouts",
		"get_workout",
		"get_user_meals",
		"get_user_weight_entries",
		"get_weekly_summary",
	}

	for _, name := range required {
		if !names[name] {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
}

func TestCoachServiceRejectsMalformedToolDates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Workout{},
		&models.Meal{},
		&models.WeightEntry{},
	); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	userID := uuid.New()
	user := models.User{
		ID:           userID,
		Email:        "coach@example.com",
		PasswordHash: "hash",
		Name:         "Coach Test",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	svc := NewCoachService(
		db,
		NewWorkoutAnalyticsService(db),
		NewAdherenceService(db),
		NewNutritionTargetService(db),
		NewIntegrationRulesService(db),
		NewNotificationService(db),
		nil,
	)

	tests := []struct {
		name    string
		fn      string
		args    string
		wantErr string
	}{
		{name: "daily summary rejects natural language", fn: "get_daily_summary", args: `{"date":"today"}`, wantErr: "date must be 2006-01-02"},
		{name: "weekly summary rejects slash date", fn: "get_weekly_summary", args: `{"date":"2026/04/15"}`, wantErr: "date must be 2006-01-02"},
		{name: "user workouts rejects malformed filter", fn: "get_user_workouts", args: `{"date":"15-04-2026"}`, wantErr: "date must be 2006-01-02"},
		{name: "weight entries rejects malformed range", fn: "get_user_weight_entries", args: `{"start_date":"tomorrow"}`, wantErr: "date must be 2006-01-02"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.DispatchFunction(tt.fn, tt.args, userID)
			if err == nil {
				t.Fatalf("expected error for %s", tt.fn)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
