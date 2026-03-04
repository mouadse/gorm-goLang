package database

import (
	"fmt"
	"log"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

// Migrate runs all schema migrations required by the application.
func Migrate(db *gorm.DB) error {
	log.Println("running database migrations...")

	if err := migrateFriendshipRequesterID(db); err != nil {
		return err
	}

	err := db.AutoMigrate(
		&models.User{},
		&models.Exercise{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.WorkoutProgram{},
		&models.ProgramEnrollment{},
		&models.ProgramProgress{},
		&models.Food{},
		&models.Meal{},
		&models.MealFood{},
		&models.WeightEntry{},
		&models.Friendship{},
		&models.Message{},
		&models.Notification{},
		&models.WeeklyAdjustment{},
	)
	if err != nil {
		return err
	}

	if err := migrateMealIndexes(db); err != nil {
		return err
	}

	log.Println("all migrations completed successfully")
	return nil
}

func migrateFriendshipRequesterID(db *gorm.DB) error {
	if !db.Migrator().HasTable(&models.Friendship{}) {
		return nil
	}

	if !db.Migrator().HasColumn(&models.Friendship{}, "requester_id") {
		if err := db.Exec("ALTER TABLE friendships ADD COLUMN requester_id uuid").Error; err != nil {
			return fmt.Errorf("add friendships.requester_id column: %w", err)
		}
	}

	if err := db.Exec("UPDATE friendships SET requester_id = user_id WHERE requester_id IS NULL").Error; err != nil {
		return fmt.Errorf("backfill friendships.requester_id: %w", err)
	}

	if err := db.Exec("ALTER TABLE friendships ALTER COLUMN requester_id SET NOT NULL").Error; err != nil {
		return fmt.Errorf("set friendships.requester_id NOT NULL: %w", err)
	}

	return nil
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
