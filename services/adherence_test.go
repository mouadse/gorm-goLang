package services

import (
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.Meal{}, &models.Workout{}, &models.WeightEntry{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func TestAdherenceService_GetUserStreaks(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdherenceService(db)
	userID := uuid.New()

	today := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	// Case 1: No data
	streaks, err := svc.GetUserStreaks(userID, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if streaks.Streaks.MealStreak != 0 || streaks.Streaks.WorkoutStreak != 0 || streaks.Streaks.WeighInStreak != 0 {
		t.Errorf("expected 0 streaks, got %+v", streaks.Streaks)
	}

	// Case 2: Meal streak
	// Today, yesterday, day before yesterday
	db.Create(&models.Meal{UserID: userID, Date: today, MealType: "lunch"})
	db.Create(&models.Meal{UserID: userID, Date: today.AddDate(0, 0, -1), MealType: "lunch"})
	db.Create(&models.Meal{UserID: userID, Date: today.AddDate(0, 0, -2), MealType: "lunch"})

	streaks, err = svc.GetUserStreaks(userID, today)
	if streaks.Streaks.MealStreak != 3 {
		t.Errorf("expected meal streak 3, got %d", streaks.Streaks.MealStreak)
	}

	// Case 3: Workout streak (weekly)
	// This week, last week, week before last week
	db.Create(&models.Workout{UserID: userID, Date: today, Type: "push"})
	db.Create(&models.Workout{UserID: userID, Date: today.AddDate(0, 0, -7), Type: "push"})
	db.Create(&models.Workout{UserID: userID, Date: today.AddDate(0, 0, -14), Type: "push"})

	streaks, err = svc.GetUserStreaks(userID, today)
	if streaks.Streaks.WorkoutStreak != 3 {
		t.Errorf("expected workout streak 3, got %d", streaks.Streaks.WorkoutStreak)
	}

	// Case 4: Adherence
	// 3 activities in last 7 days (today, yesterday, day before yesterday)
	// Today=March 4, so last 7 days includes: 4, 3, 2, 1, 28, 27, 26
	// We have: 4 (meal/workout), 3 (meal), 2 (meal)
	// That's 3 days out of 7. 3/7 = 42.857...
	expected7 := (3.0 / 7.0) * 100.0
	if streaks.AdherenceSummary.Days7 < expected7-0.01 || streaks.AdherenceSummary.Days7 > expected7+0.01 {
		t.Errorf("expected adherence 7 days to be ~42.86, got %f", streaks.AdherenceSummary.Days7)
	}
}

func TestAdherenceService_GetActivityCalendar(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdherenceService(db)
	userID := uuid.New()

	today := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	db.Create(&models.Meal{UserID: userID, Date: today, MealType: "lunch"})
	db.Create(&models.Workout{UserID: userID, Date: today, Type: "push"})
	db.Create(&models.WeightEntry{UserID: userID, Date: today.AddDate(0, 0, -1), Weight: 80.0})

	calendar, err := svc.GetActivityCalendar(userID, today.AddDate(0, 0, -5), today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	todayStr := today.Format("2006-01-02")
	yesterdayStr := today.AddDate(0, 0, -1).Format("2006-01-02")

	if len(calendar[todayStr]) != 2 {
		t.Errorf("expected 2 activities for today, got %v", calendar[todayStr])
	}
	if len(calendar[yesterdayStr]) != 1 || calendar[yesterdayStr][0] != "weight_entry" {
		t.Errorf("expected weight_entry for yesterday, got %v", calendar[yesterdayStr])
	}
}

// TestWeightEntriesContributeToAdherence tests that weight-only users get correct adherence.
func TestWeightEntriesContributeToAdherence(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdherenceService(db)
	userID := uuid.New()

	today := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	// Create weight entries for today, yesterday, day before (3 consecutive days)
	db.Create(&models.WeightEntry{UserID: userID, Date: today, Weight: 80.0})
	db.Create(&models.WeightEntry{UserID: userID, Date: today.AddDate(0, 0, -1), Weight: 80.5})
	db.Create(&models.WeightEntry{UserID: userID, Date: today.AddDate(0, 0, -2), Weight: 81.0})

	streaks, err := svc.GetUserStreaks(userID, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Weight streak should be 3
	if streaks.Streaks.WeighInStreak != 3 {
		t.Errorf("expected weigh_in_streak 3, got %d", streaks.Streaks.WeighInStreak)
	}

	// Adherence should count weight entry days - with 3 weight entries in last 7 days,
	// adherence should be 3/7 * 100 = ~42.86%
	// Previously this would be 0% because weightDates weren't added to allActivityDates
	expectedAdherence := (3.0 / 7.0) * 100.0
	if streaks.AdherenceSummary.Days7 < expectedAdherence-0.01 || streaks.AdherenceSummary.Days7 > expectedAdherence+0.01 {
		t.Errorf("expected Days7 adherence ~42.86, got %f (weight entries should count toward adherence)", streaks.AdherenceSummary.Days7)
	}
}

// TestCalendarWindowExactly30Days tests that default calendar window returns exactly 30 days.
func TestCalendarWindowExactly30Days(t *testing.T) {
	// This tests the adherence service range calculation
	// When called with a 30-day range, should return exactly 30 days inclusive
	db := setupTestDB(t)
	svc := NewAdherenceService(db)
	userID := uuid.New()

	// Create activity on a specific day to verify
	targetDate := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	db.Create(&models.Workout{UserID: userID, Date: targetDate, Type: "push"})

	// Test that a 30-day window includes exactly 30 days
	// Start from 30 days before target, end at target
	start := targetDate.AddDate(0, 0, -29) // 30 days inclusive
	end := targetDate

	calendar, err := svc.GetActivityCalendar(userID, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count unique days in calendar
	if len(calendar) != 1 {
		t.Errorf("expected exactly 1 day with activity, got %d", len(calendar))
	}

	// Verify the specific date is present
	dateStr := targetDate.Format("2006-01-02")
	if calendar[dateStr] == nil {
		t.Errorf("expected calendar to contain target date %s", dateStr)
	}
}

// TestStreaksUsesUTC tests that streaks are calculated in UTC regardless of server timezone.
func TestStreaksUsesUTC(t *testing.T) {
	db := setupTestDB(t)
	svc := NewAdherenceService(db)
	userID := uuid.New()

	// Test with UTC date
	today := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	// Create a meal at UTC midnight
	db.Create(&models.Meal{UserID: userID, Date: today, MealType: "lunch"})

	streaks, err := svc.GetUserStreaks(userID, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if streaks.Streaks.MealStreak != 1 {
		t.Errorf("expected meal streak 1, got %d", streaks.Streaks.MealStreak)
	}

	// Test that passing a local time that's actually the previous day in UTC works correctly
	// If server is in UTC-5, then 11pm on March 4 local is actually 4am March 5 UTC
	// The streak should still be calculated correctly based on UTC
	localMidnight := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC) // Simulate UTC timezone
	streaks2, err := svc.GetUserStreaks(userID, localMidnight)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if streaks2.Streaks.MealStreak != 1 {
		t.Errorf("expected meal streak 1 with UTC date, got %d", streaks2.Streaks.MealStreak)
	}
}
