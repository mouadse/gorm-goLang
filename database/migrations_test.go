package database_test

import (
	"fmt"
	"testing"

	"fitness-tracker/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateCreatesRequiredTables(t *testing.T) {
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
		"foods",
		"meal_foods",
		"refresh_tokens",
		"user_sessions",
		"export_jobs",
		"deletion_requests",
	}

	for _, table := range expectedTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	for _, table := range legacyTables() {
		if db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q to be absent", table)
		}
	}
}

func TestMigrateDropsLegacyTablesOnExistingDatabase(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	for _, table := range legacyTables() {
		if err := db.Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY)").Error; err != nil {
			t.Fatalf("create legacy table %q: %v", table, err)
		}
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	for _, table := range legacyTables() {
		if db.Migrator().HasTable(table) {
			t.Fatalf("expected legacy table %q to be dropped", table)
		}
	}
}

func legacyTables() []string {
	return []string{
		"friendships",
		"messages",
		"notifications",
		"weekly_adjustments",
		"workout_programs",
		"program_enrollments",
		"program_progresses",
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
