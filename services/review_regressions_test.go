package services

import (
	"fmt"
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
