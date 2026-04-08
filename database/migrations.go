package database

import (
	"fmt"
	"log"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var obsoleteTables = []string{
	"friendships",
	"messages",
	"weekly_adjustments",
	"program_progresses",
	"program_enrollments",
}

// Migrate runs all schema migrations required by the application.
func Migrate(db *gorm.DB) error {
	log.Println("running database migrations...")

	// Drop obsolete tables BEFORE AutoMigrate to avoid schema conflicts
	// This ensures legacy tables with incompatible schemas are removed first
	if err := dropObsoleteTables(db); err != nil {
		return err
	}

	if err := migrateLegacyExercises(db); err != nil {
		return err
	}

	// Migrate legacy notifications table to new UUID schema if needed
	// This is a ONE-TIME migration that only runs if the old schema exists
	if err := migrateLegacyNotifications(db); err != nil {
		return err
	}

	err := db.AutoMigrate(
		&models.User{},
		&models.TwoFactorSecret{},
		&models.RecoveryCode{},
		&models.Exercise{},
		&models.WeightEntry{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.WorkoutCardioEntry{},
		&models.WorkoutTemplate{},
		&models.WorkoutTemplateExercise{},
		&models.WorkoutTemplateSet{},
		&models.WorkoutProgram{},
		&models.ProgramWeek{},
		&models.ProgramSession{},
		&models.ProgramAssignment{},
		&models.Meal{},
		&models.Food{},
		&models.MealFood{},
		&models.Nutrient{},
		&models.FoodNutrient{},
		&services.RefreshToken{},
		&services.UserSession{},
		&services.ExportJob{},
		&services.DeletionRequest{},
		&models.FavoriteFood{},
		&models.Recipe{},
		&models.RecipeItem{},
		&models.Notification{},
		&models.AuditLog{},
		&models.FoodImportLog{},
		&models.Conversation{},
		&models.ConversationMessage{},
	)
	if err != nil {
		return err
	}

	if err := backfillLegacyExerciseLibIDs(db); err != nil {
		return err
	}

	if err := EnsureAdminViews(db); err != nil {
		return err
	}

	if err := migrateMealIndexes(db); err != nil {
		return err
	}

	if err := migrateNutrientIndexes(db); err != nil {
		return err
	}

	log.Println("all migrations completed successfully")
	return nil
}

func dropObsoleteTables(db *gorm.DB) error {
	for _, table := range obsoleteTables {
		if db.Migrator().HasTable(table) {
			if err := db.Migrator().DropTable(table); err != nil {
				return fmt.Errorf("drop obsolete table %s: %w", table, err)
			}
		}
	}

	return nil
}

// migrateLegacyNotifications handles the one-time migration from legacy notifications table
// to the new UUID-based schema. It only drops the table if it has the old INTEGER primary key.
func migrateLegacyExercises(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Exercise{}) {
		return nil
	}

	// Make sure the new columns exist first so we can backfill
	// AutoMigrate is safe to run multiple times
	if err := db.AutoMigrate(&models.Exercise{}); err != nil {
		return fmt.Errorf("pre-migrate exercises table: %w", err)
	}

	if err := backfillLegacyExerciseLibIDs(db); err != nil {
		return err
	}

	if db.Migrator().HasColumn(&models.Exercise{}, "muscle_group") {
		log.Println("backfilling legacy muscle_group to primary_muscles")
		if err := db.Exec("UPDATE exercises SET primary_muscles = muscle_group WHERE primary_muscles IS NULL OR primary_muscles = ''").Error; err != nil {
			return err
		}
	}

	if db.Migrator().HasColumn(&models.Exercise{}, "difficulty") {
		log.Println("backfilling legacy difficulty to level")
		if err := db.Exec("UPDATE exercises SET level = difficulty WHERE level IS NULL OR level = ''").Error; err != nil {
			return err
		}
	}

	if db.Dialector.Name() == "postgres" {
		log.Println("dropping materialized view exercise_popularity to allow column schema changes")
		if err := db.Exec("DROP MATERIALIZED VIEW IF EXISTS exercise_popularity CASCADE").Error; err != nil {
			return fmt.Errorf("drop materialized view exercise_popularity: %w", err)
		}
	}

	cols := []string{"muscle_group", "difficulty", "video_url"}
	for _, col := range cols {
		if db.Migrator().HasColumn(&models.Exercise{}, col) {
			log.Printf("dropping legacy column %s from exercises table", col)
			if err := db.Migrator().DropColumn(&models.Exercise{}, col); err != nil {
				return err
			}
		}
	}
	return nil
}

func backfillLegacyExerciseLibIDs(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.Exercise{}, "exercise_lib_id") {
		return nil
	}

	rows, err := db.Table("exercises").
		Select("CAST(id AS TEXT) AS id").
		Where("exercise_lib_id IS NULL OR TRIM(exercise_lib_id) = ''").
		Rows()
	if err != nil {
		return fmt.Errorf("list legacy exercises missing exercise_lib_id: %w", err)
	}
	defer rows.Close()

	var exerciseIDs []string
	for rows.Next() {
		var exerciseID string
		if err := rows.Scan(&exerciseID); err != nil {
			return fmt.Errorf("scan legacy exercise id: %w", err)
		}
		exerciseIDs = append(exerciseIDs, strings.TrimSpace(exerciseID))
	}

	if len(exerciseIDs) == 0 {
		return nil
	}

	log.Printf("backfilling exercise_lib_id for %d legacy exercises", len(exerciseIDs))

	for _, exerciseID := range exerciseIDs {
		if exerciseID == "" {
			exerciseID = uuid.NewString()
		}

		if err := db.Table("exercises").
			Where("CAST(id AS TEXT) = ?", exerciseID).
			UpdateColumn("exercise_lib_id", "local-"+exerciseID).Error; err != nil {
			return fmt.Errorf("backfill exercise_lib_id for exercise %q: %w", exerciseID, err)
		}
	}

	return nil
}

func migrateLegacyNotifications(db *gorm.DB) error {
	if !db.Migrator().HasTable("notifications") {
		return nil
	}

	columnTypes, err := db.Migrator().ColumnTypes("notifications")
	if err != nil {
		return fmt.Errorf("inspect notifications table schema: %w", err)
	}

	for _, columnType := range columnTypes {
		if !strings.EqualFold(columnType.Name(), "id") {
			continue
		}

		if strings.Contains(strings.ToUpper(columnType.DatabaseTypeName()), "INT") {
			log.Println("dropping legacy notifications table with INTEGER primary key")
			if err := db.Migrator().DropTable("notifications"); err != nil {
				return fmt.Errorf("drop legacy notifications table: %w", err)
			}
			return nil
		}

		log.Println("notifications table already has non-integer primary key schema, skipping migration")
		return nil
	}

	return fmt.Errorf("inspect notifications table schema: id column not found")
}

func migrateMealIndexes(db *gorm.DB) error {
	if db.Migrator().HasIndex(&models.Meal{}, "idx_user_date_type") {
		if err := db.Migrator().DropIndex(&models.Meal{}, "idx_user_date_type"); err != nil {
			return fmt.Errorf("drop legacy meal unique index: %w", err)
		}
	}

	if !db.Migrator().HasIndex(&models.Meal{}, "idx_meals_user_date_type") {
		if err := db.Migrator().CreateIndex(&models.Meal{}, "idx_meals_user_date_type"); err != nil {
			return fmt.Errorf("create meal search index: %w", err)
		}
	}

	return nil
}

func migrateNutrientIndexes(db *gorm.DB) error {
	// Ensure unique constraint on food_nutrients (food_id, nutrient_id) for idempotency
	if !db.Migrator().HasIndex(&models.FoodNutrient{}, "idx_food_nutrients_unique") {
		if err := db.Migrator().CreateIndex(&models.FoodNutrient{}, "idx_food_nutrients_unique"); err != nil {
			return fmt.Errorf("create food_nutrients unique index: %w", err)
		}
	}

	return nil
}
