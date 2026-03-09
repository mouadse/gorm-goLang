package database_test

import (
	"fmt"
	"testing"

	"fitness-tracker/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateCreatesOnlyCoreTables(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	expectedTables := []string{
		"users",
		"exercises",
		"weight_entries",
		"workouts",
		"workout_exercises",
		"workout_sets",
		"meals",
	}

	for _, table := range expectedTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	removedTables := []string{
		"foods",
		"meal_foods",
		"friendships",
		"messages",
		"notifications",
		"weekly_adjustments",
		"workout_programs",
		"program_enrollments",
		"program_progresses",
	}

	for _, table := range removedTables {
		if db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to be absent", table)
		}
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	return db
}
