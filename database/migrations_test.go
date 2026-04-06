package database_test

import (
	"fmt"
	"testing"

	"fitness-tracker/database"
	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
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
		"two_factor_secrets",
		"recovery_codes",
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

func TestMigrateAddsNullableSessionIDToExistingRefreshTokens(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	if err := db.Exec(`CREATE TABLE refresh_tokens (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		token_hash TEXT NOT NULL,
		user_agent TEXT,
		ip_address TEXT,
		expires_at DATETIME NOT NULL,
		revoked_at DATETIME,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy refresh_tokens table: %v", err)
	}

	if err := db.Exec(`INSERT INTO refresh_tokens (id, user_id, token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ('rt-1', 'user-1', 'hash-1', 'agent', '127.0.0.1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("seed legacy refresh_tokens row: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database with legacy refresh tokens: %v", err)
	}

	if !db.Migrator().HasColumn(&services.RefreshToken{}, "session_id") {
		t.Fatalf("expected migrated refresh_tokens table to contain session_id")
	}
}

func TestMigrateDropsLegacySQLiteNotificationsTable(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)

	if err := db.Exec(`CREATE TABLE notifications (
		id INTEGER PRIMARY KEY,
		user_id TEXT NOT NULL,
		title TEXT NOT NULL,
		message TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create legacy notifications table: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database with legacy notifications: %v", err)
	}

	columnTypes, err := db.Migrator().ColumnTypes("notifications")
	if err != nil {
		t.Fatalf("inspect notifications columns: %v", err)
	}

	var idType string
	for _, columnType := range columnTypes {
		if columnType.Name() == "id" {
			idType = columnType.DatabaseTypeName()
			break
		}
	}
	if idType == "" {
		t.Fatal("expected notifications.id column to exist after migration")
	}
	if idType == "INTEGER" {
		t.Fatalf("expected migrated notifications.id to no longer use INTEGER, got %q", idType)
	}

	user := models.User{
		ID:           uuid.New(),
		Email:        "notifications@example.com",
		PasswordHash: "hash",
		Name:         "Notifications User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	notification := models.Notification{
		UserID:  user.ID,
		Type:    models.NotificationExportReady,
		Title:   "Export ready",
		Message: "Your export is ready",
	}
	if err := db.Create(&notification).Error; err != nil {
		t.Fatalf("create notification after migration: %v", err)
	}
}

func legacyTables() []string {
	return []string{
		"friendships",
		"messages",
		"weekly_adjustments",
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
