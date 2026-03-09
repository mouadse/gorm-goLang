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

	err := db.AutoMigrate(
		&models.User{},
		&models.Exercise{},
		&models.WeightEntry{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.Meal{},
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
