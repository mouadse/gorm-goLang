package services

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthServiceRefreshSessionMatchesStoredHash(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	db := openServicesTestDB(t,
		&models.User{},
		&RefreshToken{},
		&UserSession{},
	)
	user := createTestUser(t, db)
	svc := NewAuthService(db)

	issued, err := svc.CreateSession(user.ID, "agent-a", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	refreshed, err := svc.RefreshSession(issued.RefreshToken, "agent-b", "127.0.0.2")
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}
	if refreshed.RefreshToken == issued.RefreshToken {
		t.Fatalf("expected refresh token rotation")
	}

	var tokens []RefreshToken
	if err := db.Order("created_at asc").Find(&tokens).Error; err != nil {
		t.Fatalf("list refresh tokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 refresh tokens after rotation, got %d", len(tokens))
	}
	if tokens[0].RevokedAt == nil {
		t.Fatalf("expected original refresh token to be revoked")
	}
	if tokens[1].RevokedAt != nil {
		t.Fatalf("expected rotated refresh token to remain active")
	}
}

func TestAuthServiceRevokeSessionMatchesStoredHash(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	db := openServicesTestDB(t,
		&models.User{},
		&RefreshToken{},
		&UserSession{},
	)
	user := createTestUser(t, db)
	svc := NewAuthService(db)

	issued, err := svc.CreateSession(user.ID, "agent-a", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.RevokeSession(issued.RefreshToken); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	var stored RefreshToken
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load refresh token: %v", err)
	}
	if stored.RevokedAt == nil {
		t.Fatalf("expected refresh token to be revoked")
	}
}

func TestAuthServiceRevokeUserSessionRevokesSiblingTokens(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	db := openServicesTestDB(t,
		&models.User{},
		&RefreshToken{},
		&UserSession{},
	)
	user := createTestUser(t, db)
	svc := NewAuthService(db)

	issued, err := svc.CreateSession(user.ID, "agent-a", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	rotated, err := svc.RefreshSession(issued.RefreshToken, "agent-a", "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}

	var active RefreshToken
	if err := db.Where("revoked_at IS NULL").First(&active).Error; err != nil {
		t.Fatalf("load active token: %v", err)
	}

	siblingToken, err := GenerateSecureToken()
	if err != nil {
		t.Fatalf("generate sibling token: %v", err)
	}
	siblingHash, err := HashToken(siblingToken)
	if err != nil {
		t.Fatalf("hash sibling token: %v", err)
	}
	sibling := RefreshToken{
		UserID:    user.ID,
		SessionID: active.SessionID,
		TokenHash: siblingHash,
		UserAgent: "agent-a",
		IPAddress: "127.0.0.1",
		ExpiresAt: time.Now().UTC().Add(RefreshTokenTTL),
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&sibling).Error; err != nil {
		t.Fatalf("create sibling token: %v", err)
	}

	if err := svc.RevokeUserSession(user.ID, rotated.RefreshToken); err != nil {
		t.Fatalf("revoke user session: %v", err)
	}

	var remaining int64
	if err := db.Model(&RefreshToken{}).Where("session_id = ? AND revoked_at IS NULL", active.SessionID).Count(&remaining).Error; err != nil {
		t.Fatalf("count remaining tokens: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected all refresh tokens for the session to be revoked, found %d", remaining)
	}

	var sessions int64
	if err := db.Model(&UserSession{}).Where("session_id = ?", active.SessionID).Count(&sessions).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("expected session row to be deleted, found %d", sessions)
	}
}

func TestLoginRequestStringRedactsSensitiveFields(t *testing.T) {
	req := LoginRequest{
		Email:          "user@example.com",
		Password:       "super-secret-password",
		TOTPCode:       "123456",
		RecoveryCode:   "RECOVERYSECRET",
		TwoFactorToken: "challenge-token",
	}

	formatted := fmt.Sprintf("%+v", req)
	for _, secret := range []string{
		req.Password,
		req.TOTPCode,
		req.RecoveryCode,
		req.TwoFactorToken,
	} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("expected formatted login request to redact %q", secret)
		}
	}
	if !strings.Contains(formatted, req.Email) {
		t.Fatalf("expected formatted login request to retain email")
	}
}

func TestIntegrationRulesContextExcludesFutureWeekWorkouts(t *testing.T) {
	date := time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC) // Wednesday

	db := openServicesTestDB(t,
		&models.User{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.Meal{},
		&models.MealFood{},
		&models.Food{},
	)
	user := createTestUser(t, db)

	workouts := []models.Workout{
		{UserID: user.ID, Date: date.AddDate(0, 0, -2), Type: "push"},
		{UserID: user.ID, Date: date, Type: "pull"},
		{UserID: user.ID, Date: date.AddDate(0, 0, 2), Type: "legs"},
	}
	for _, workout := range workouts {
		if err := db.Create(&workout).Error; err != nil {
			t.Fatalf("create workout: %v", err)
		}
	}

	svc := NewIntegrationRulesService(db)
	ctx, err := svc.GetWorkoutNutritionContext(user.ID, date)
	if err != nil {
		t.Fatalf("get workout nutrition context: %v", err)
	}

	if ctx.WeeklyWorkouts != 2 {
		t.Fatalf("expected weekly workouts through %s to equal 2, got %d", date.Format("2006-01-02"), ctx.WeeklyWorkouts)
	}
}

func TestWorkoutAnalyticsGetWeeklyStatsUsesFullSundayWeeks(t *testing.T) {
	now := time.Now().UTC()
	currentWeekStart := startOfWeekUTC(now)
	previousWeekStart := currentWeekStart.AddDate(0, 0, -7)
	tooOldSaturday := previousWeekStart.AddDate(0, 0, -1)

	db := openServicesTestDB(t,
		&models.User{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
	)
	user := createTestUser(t, db)

	workouts := []models.Workout{
		{UserID: user.ID, Date: currentWeekStart, Type: "push", Duration: 45},
		{UserID: user.ID, Date: previousWeekStart, Type: "pull", Duration: 40},
		{UserID: user.ID, Date: tooOldSaturday, Type: "legs", Duration: 60},
	}
	for _, workout := range workouts {
		if err := db.Create(&workout).Error; err != nil {
			t.Fatalf("create workout: %v", err)
		}
	}

	svc := NewWorkoutAnalyticsService(db)
	stats, err := svc.GetWeeklyStats(user.ID, 2)
	if err != nil {
		t.Fatalf("get weekly stats: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 weekly buckets, got %d", len(stats))
	}
	if !stats[0].WeekStart.Equal(previousWeekStart) {
		t.Fatalf("expected oldest week to start on %s, got %s", previousWeekStart.Format("2006-01-02"), stats[0].WeekStart.Format("2006-01-02"))
	}
	if !stats[1].WeekStart.Equal(currentWeekStart) {
		t.Fatalf("expected newest week to start on %s, got %s", currentWeekStart.Format("2006-01-02"), stats[1].WeekStart.Format("2006-01-02"))
	}
}

func TestNutritionTargetServiceDailySummaryIncludesFiber(t *testing.T) {
	date := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)

	db := openServicesTestDB(t,
		&models.User{},
		&models.Meal{},
		&models.MealFood{},
		&models.Food{},
	)
	user := createTestUser(t, db)

	food := models.Food{
		Name:          "Oats",
		ServingSize:   1,
		ServingUnit:   "cup",
		Calories:      150,
		Protein:       5,
		Carbohydrates: 27,
		Fat:           3,
		Fiber:         4,
	}
	if err := db.Create(&food).Error; err != nil {
		t.Fatalf("create food: %v", err)
	}

	meal := models.Meal{
		UserID:   user.ID,
		MealType: "breakfast",
		Date:     date,
	}
	if err := db.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}

	item := models.MealFood{
		MealID:   meal.ID,
		FoodID:   food.ID,
		Quantity: 2,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create meal item: %v", err)
	}

	svc := NewNutritionTargetService(db)
	summary, err := svc.GetDailyNutritionSummary(user.ID, date)
	if err != nil {
		t.Fatalf("get daily nutrition summary: %v", err)
	}

	if summary.TotalFiber != 8 {
		t.Fatalf("expected total fiber to equal 8, got %v", summary.TotalFiber)
	}
}

func TestIntegrationRulesFormatNumericAdjustmentsAsDecimals(t *testing.T) {
	baseTargets := &NutritionTargets{
		Calories: 2000,
		Protein:  160,
		Carbs:    220,
		Fat:      70,
	}
	ctx := WorkoutNutritionContext{
		Goal:            "muscle_gain",
		WorkoutType:     "cardio",
		WorkoutDuration: 30,
		HasWorkout:      true,
		ProteinIntake:   50,
		CalorieIntake:   2100,
	}

	adjustment := NewIntegrationRulesService(nil).ApplyIntegrationRules(ctx, baseTargets)

	var cardioRule, proteinRule string
	for _, rule := range adjustment.Rules {
		switch rule.ID {
		case "cardio_bonus":
			cardioRule = rule.Adjustment
		case "muscle_gain_protein_warning":
			proteinRule = rule.Adjustment
		}
	}

	if cardioRule != "+300 calories for 30 min cardio" {
		t.Fatalf("unexpected cardio adjustment text: %q", cardioRule)
	}
	if proteinRule != "Aim for 160g+ protein for muscle growth" {
		t.Fatalf("unexpected protein warning text: %q", proteinRule)
	}
}

func TestAdminDashboardSQLiteUsesBaseTables(t *testing.T) {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	db := openServicesTestDB(t,
		&models.User{},
		&models.Workout{},
		&models.Meal{},
		&models.WeightEntry{},
	)

	users := []models.User{
		{
			Email:         "recent-admin-metrics@example.com",
			PasswordHash:  "hash",
			Name:          "Recent User",
			Goal:          "maintain",
			ActivityLevel: "active",
			CreatedAt:     today.AddDate(0, 0, -2),
		},
		{
			Email:         "older-admin-metrics@example.com",
			PasswordHash:  "hash",
			Name:          "Older User",
			Goal:          "lose_fat",
			ActivityLevel: "active",
			CreatedAt:     today.AddDate(0, 0, -20),
		},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	records := []interface{}{
		&models.Workout{UserID: users[0].ID, Date: today, Type: "push"},
		&models.Meal{UserID: users[0].ID, Date: today, MealType: "lunch"},
		&models.Meal{UserID: users[1].ID, Date: today.AddDate(0, 0, -3), MealType: "dinner"},
		&models.WeightEntry{UserID: users[1].ID, Date: today.AddDate(0, 0, -10), Weight: 82.5},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	svc := NewAdminDashboardService(db, nil)

	requireInt64 := func(t *testing.T, stats map[string]any, key string) int64 {
		t.Helper()
		value, ok := stats[key].(int64)
		if !ok {
			t.Fatalf("expected %s to be int64, got %T", key, stats[key])
		}
		return value
	}

	t.Run("executive summary", func(t *testing.T) {
		summary, err := svc.GetExecutiveSummary(context.Background())
		if err != nil {
			t.Fatalf("get executive summary: %v", err)
		}

		if summary.TotalUsers != 2 {
			t.Fatalf("expected total users to equal 2, got %d", summary.TotalUsers)
		}
		if summary.DAU != 1 {
			t.Fatalf("expected DAU to equal 1, got %d", summary.DAU)
		}
		if summary.MAU != 2 {
			t.Fatalf("expected MAU to equal 2, got %d", summary.MAU)
		}
		if summary.DAUMAU_Ratio != 50 {
			t.Fatalf("expected DAU/MAU ratio to equal 50, got %v", summary.DAUMAU_Ratio)
		}
		if summary.NewUsers7d != 1 {
			t.Fatalf("expected new users in 7 days to equal 1, got %d", summary.NewUsers7d)
		}
		if summary.WorkoutsToday != 1 {
			t.Fatalf("expected workouts today to equal 1, got %d", summary.WorkoutsToday)
		}
		if summary.MealsToday != 1 {
			t.Fatalf("expected meals today to equal 1, got %d", summary.MealsToday)
		}
	})

	t.Run("user analytics", func(t *testing.T) {
		stats, err := svc.GetUserAnalytics(context.Background())
		if err != nil {
			t.Fatalf("get user analytics: %v", err)
		}

		if got := requireInt64(t, stats, "total_users"); got != 2 {
			t.Fatalf("expected total_users to equal 2, got %d", got)
		}
		if got := requireInt64(t, stats, "active_users_7d"); got != 2 {
			t.Fatalf("expected active_users_7d to equal 2, got %d", got)
		}
		if got := requireInt64(t, stats, "mau"); got != 2 {
			t.Fatalf("expected mau to equal 2, got %d", got)
		}

		goalBreakdown, ok := stats["goal_breakdown"].([]map[string]any)
		if !ok {
			t.Fatalf("expected goal_breakdown to be []map[string]any, got %T", stats["goal_breakdown"])
		}

		goals := make(map[string]int64, len(goalBreakdown))
		for _, bucket := range goalBreakdown {
			goal, ok := bucket["goal"].(string)
			if !ok {
				t.Fatalf("expected goal bucket to expose a goal string, got %#v", bucket)
			}
			count, ok := bucket["count"].(int64)
			if !ok {
				t.Fatalf("expected goal bucket count to be int64, got %#v", bucket["count"])
			}
			if _, exists := bucket["type"]; exists {
				t.Fatalf("expected goal bucket to omit type alias, got %#v", bucket)
			}
			goals[goal] = count
		}

		if goals["maintain"] != 1 {
			t.Fatalf("expected maintain goal count to equal 1, got %d", goals["maintain"])
		}
		if goals["lose_fat"] != 1 {
			t.Fatalf("expected lose_fat goal count to equal 1, got %d", goals["lose_fat"])
		}
	})
}

func TestProcessDeletionRequestHardDeletesSoftDeletedDependents(t *testing.T) {
	date := time.Now().UTC().Truncate(24 * time.Hour)

	db := openServicesTestDB(t,
		&models.User{},
		&DeletionRequest{},
		&ExportJob{},
		&RefreshToken{},
		&UserSession{},
		&models.TwoFactorSecret{},
		&models.RecoveryCode{},
		&models.Notification{},
		&models.Exercise{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.WorkoutCardioEntry{},
		&models.Food{},
		&models.Meal{},
		&models.MealFood{},
		&models.FavoriteFood{},
		&models.WeightEntry{},
		&models.Recipe{},
		&models.RecipeItem{},
		&models.WorkoutTemplate{},
		&models.WorkoutTemplateExercise{},
		&models.WorkoutTemplateSet{},
		&models.WorkoutProgram{},
		&models.ProgramWeek{},
		&models.ProgramSession{},
		&models.ProgramAssignment{},
	)
	user := createTestUser(t, db)

	exercise := models.Exercise{Name: "Deletion Test Exercise"}
	if err := db.Create(&exercise).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	food := models.Food{
		Name:          "Deletion Test Food",
		Source:        "user",
		ServingSize:   100,
		ServingUnit:   "g",
		Calories:      250,
		Protein:       20,
		Carbohydrates: 15,
		Fat:           10,
	}
	if err := db.Create(&food).Error; err != nil {
		t.Fatalf("create food: %v", err)
	}

	workout := models.Workout{UserID: user.ID, Date: date, Type: "push", Duration: 45}
	if err := db.Create(&workout).Error; err != nil {
		t.Fatalf("create workout: %v", err)
	}
	workoutExercise := models.WorkoutExercise{
		WorkoutID:  workout.ID,
		ExerciseID: exercise.ID,
		Order:      1,
		Sets:       3,
		Reps:       10,
	}
	if err := db.Create(&workoutExercise).Error; err != nil {
		t.Fatalf("create workout exercise: %v", err)
	}
	workoutSet := models.WorkoutSet{
		WorkoutExerciseID: workoutExercise.ID,
		SetNumber:         1,
		Reps:              10,
		Weight:            50,
	}
	if err := db.Create(&workoutSet).Error; err != nil {
		t.Fatalf("create workout set: %v", err)
	}
	distance := 3.0
	distanceUnit := "km"
	caloriesBurned := 200
	avgHeartRate := 145
	cardioEntry := models.WorkoutCardioEntry{
		WorkoutID:       workout.ID,
		Modality:        "running",
		DurationMinutes: 20,
		Distance:        &distance,
		DistanceUnit:    &distanceUnit,
		CaloriesBurned:  &caloriesBurned,
		AvgHeartRate:    &avgHeartRate,
	}
	if err := db.Create(&cardioEntry).Error; err != nil {
		t.Fatalf("create workout cardio entry: %v", err)
	}

	meal := models.Meal{UserID: user.ID, MealType: "lunch", Date: date}
	if err := db.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}
	mealFood := models.MealFood{MealID: meal.ID, FoodID: food.ID, Quantity: 1.5}
	if err := db.Create(&mealFood).Error; err != nil {
		t.Fatalf("create meal food: %v", err)
	}

	weightEntry := models.WeightEntry{UserID: user.ID, Date: date, Weight: 82.3}
	if err := db.Create(&weightEntry).Error; err != nil {
		t.Fatalf("create weight entry: %v", err)
	}

	recipe := models.Recipe{UserID: user.ID, Name: "Deletion Test Recipe", Servings: 2}
	if err := db.Create(&recipe).Error; err != nil {
		t.Fatalf("create recipe: %v", err)
	}
	recipeItem := models.RecipeItem{RecipeID: recipe.ID, FoodID: food.ID, Quantity: 2}
	if err := db.Create(&recipeItem).Error; err != nil {
		t.Fatalf("create recipe item: %v", err)
	}

	template := models.WorkoutTemplate{OwnerID: user.ID, Name: "Deletion Template", Type: "push"}
	if err := db.Create(&template).Error; err != nil {
		t.Fatalf("create workout template: %v", err)
	}
	templateExercise := models.WorkoutTemplateExercise{
		TemplateID: template.ID,
		ExerciseID: exercise.ID,
		Order:      1,
		Sets:       3,
		Reps:       8,
	}
	if err := db.Create(&templateExercise).Error; err != nil {
		t.Fatalf("create workout template exercise: %v", err)
	}
	templateSet := models.WorkoutTemplateSet{
		TemplateExerciseID: templateExercise.ID,
		SetNumber:          1,
		Reps:               8,
		Weight:             40,
	}
	if err := db.Create(&templateSet).Error; err != nil {
		t.Fatalf("create workout template set: %v", err)
	}

	program := models.WorkoutProgram{Name: "Deletion Program", CreatedBy: user.ID, IsActive: true}
	if err := db.Create(&program).Error; err != nil {
		t.Fatalf("create workout program: %v", err)
	}
	programWeek := models.ProgramWeek{ProgramID: program.ID, WeekNumber: 1, Name: "Week 1"}
	if err := db.Create(&programWeek).Error; err != nil {
		t.Fatalf("create program week: %v", err)
	}
	programSession := models.ProgramSession{WeekID: programWeek.ID, DayNumber: 1, WorkoutTemplateID: &template.ID}
	if err := db.Create(&programSession).Error; err != nil {
		t.Fatalf("create program session: %v", err)
	}
	programAssignment := models.ProgramAssignment{
		UserID:     user.ID,
		ProgramID:  program.ID,
		AssignedAt: date,
		Status:     "assigned",
	}
	if err := db.Create(&programAssignment).Error; err != nil {
		t.Fatalf("create program assignment: %v", err)
	}

	request := DeletionRequest{
		UserID:      user.ID,
		RequestedAt: date,
		Status:      "pending",
	}
	if err := db.Create(&request).Error; err != nil {
		t.Fatalf("create deletion request: %v", err)
	}

	svc := NewExportService(db, nil)
	if err := svc.ProcessDeletionRequest(user.ID); err != nil {
		t.Fatalf("process deletion request: %v", err)
	}

	assertDeleted := func(name string, model any, query string, args ...any) {
		t.Helper()

		var remaining int64
		if err := db.Unscoped().Model(model).Where(query, args...).Count(&remaining).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if remaining != 0 {
			t.Fatalf("expected %s to be hard-deleted, found %d rows", name, remaining)
		}
	}

	assertDeleted("user", &models.User{}, "id = ?", user.ID)
	assertDeleted("workout", &models.Workout{}, "id = ?", workout.ID)
	assertDeleted("workout_exercise", &models.WorkoutExercise{}, "id = ?", workoutExercise.ID)
	assertDeleted("workout_set", &models.WorkoutSet{}, "id = ?", workoutSet.ID)
	assertDeleted("workout_cardio_entry", &models.WorkoutCardioEntry{}, "id = ?", cardioEntry.ID)
	assertDeleted("meal", &models.Meal{}, "id = ?", meal.ID)
	assertDeleted("meal_food", &models.MealFood{}, "id = ?", mealFood.ID)
	assertDeleted("weight_entry", &models.WeightEntry{}, "id = ?", weightEntry.ID)
	assertDeleted("recipe", &models.Recipe{}, "id = ?", recipe.ID)
	assertDeleted("recipe_item", &models.RecipeItem{}, "id = ?", recipeItem.ID)
	assertDeleted("workout_template", &models.WorkoutTemplate{}, "id = ?", template.ID)
	assertDeleted("workout_template_exercise", &models.WorkoutTemplateExercise{}, "id = ?", templateExercise.ID)
	assertDeleted("workout_template_set", &models.WorkoutTemplateSet{}, "id = ?", templateSet.ID)
	assertDeleted("workout_program", &models.WorkoutProgram{}, "id = ?", program.ID)
	assertDeleted("program_week", &models.ProgramWeek{}, "id = ?", programWeek.ID)
	assertDeleted("program_session", &models.ProgramSession{}, "id = ?", programSession.ID)
	assertDeleted("program_assignment", &models.ProgramAssignment{}, "id = ?", programAssignment.ID)

	var processed DeletionRequest
	if err := db.Where("id = ?", request.ID).First(&processed).Error; err != nil {
		t.Fatalf("load processed deletion request: %v", err)
	}
	if processed.ProcessedAt == nil {
		t.Fatalf("expected deletion request to be marked processed")
	}
	if processed.Status != "processed" {
		t.Fatalf("expected deletion request status to be processed, got %q", processed.Status)
	}
}

func openServicesTestDB(t *testing.T, modelsToMigrate ...interface{}) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	if len(modelsToMigrate) > 0 {
		if err := db.AutoMigrate(modelsToMigrate...); err != nil {
			t.Fatalf("auto migrate test schema: %v", err)
		}
	}

	return db
}

func createTestUser(t *testing.T, db *gorm.DB) models.User {
	t.Helper()

	user := models.User{
		ID:            uuid.New(),
		Email:         fmt.Sprintf("%s@example.com", t.Name()),
		PasswordHash:  "hashed-password",
		Name:          "Test User",
		Age:           30,
		Weight:        80,
		Height:        180,
		Goal:          "maintain",
		ActivityLevel: "active",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}
