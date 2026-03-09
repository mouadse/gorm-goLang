package main

import (
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
	if len(users) != 4 {
		t.Fatalf("expected 4 seeded users, got %d", len(users))
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

func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite db: %v", err)
	}

	return db
}
