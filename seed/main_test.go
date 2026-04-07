package main

import (
	"fmt"
	"strings"
	"testing"

	"fitness-tracker/database"
	"fitness-tracker/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedUsersBackfillsExistingRows(t *testing.T) {
	t.Parallel()

	db := newSeedTestDB(t)

	existing := models.User{
		Email:        "alex@example.com",
		PasswordHash: "stale-password",
		Name:         "Old Alex",
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	users, err := seedUsers(db)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if len(users) != 12 {
		t.Fatalf("expected 12 seeded users, got %d", len(users))
	}

	var alex models.User
	if err := db.First(&alex, "email = ?", "alex@example.com").Error; err != nil {
		t.Fatalf("load seeded alex: %v", err)
	}

	if alex.Name != "Alex Johnson" {
		t.Fatalf("expected seeded name to be updated, got %q", alex.Name)
	}
	if alex.DateOfBirth == nil {
		t.Fatal("expected seeded date_of_birth to be backfilled")
	}
	if alex.Age <= 0 {
		t.Fatalf("expected seeded age to be backfilled, got %d", alex.Age)
	}
	if alex.Goal == "" || alex.ActivityLevel == "" || alex.TDEE == 0 {
		t.Fatalf("expected seeded profile fields to be backfilled, got goal=%q activity_level=%q tdee=%d", alex.Goal, alex.ActivityLevel, alex.TDEE)
	}
}

func TestSeedExercisesIncludesBeginnerHomeOptionsForShouldersAndBack(t *testing.T) {
	t.Parallel()

	db := newSeedTestDB(t)

	if _, err := seedExercises(db); err != nil {
		t.Fatalf("seed exercises: %v", err)
	}

	var shoulderCount int64
	if err := db.Model(&models.Exercise{}).
		Where("muscle_group = ? AND equipment = ? AND difficulty = ?", "Shoulders", "Dumbbell", "Beginner").
		Count(&shoulderCount).Error; err != nil {
		t.Fatalf("count shoulder exercises: %v", err)
	}
	if shoulderCount == 0 {
		t.Fatal("expected at least one beginner dumbbell shoulder exercise")
	}

	var backCount int64
	if err := db.Model(&models.Exercise{}).
		Where("muscle_group = ? AND difficulty = ? AND equipment IN ?", "Back", "Beginner", []string{"Dumbbell", "Bodyweight", "Resistance Band"}).
		Count(&backCount).Error; err != nil {
		t.Fatalf("count back exercises: %v", err)
	}
	if backCount == 0 {
		t.Fatal("expected at least one beginner back exercise for home equipment")
	}
}

func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite db: %v", err)
	}

	return db
}
