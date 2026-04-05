package database

import (
	"fmt"
	"log"

	"fitness-tracker/models"
	"fitness-tracker/services"
	"gorm.io/gorm"
)

var obsoleteTables = []string{
	"friendships",
	"messages",
	"notifications",
	"weekly_adjustments",
	"program_progresses",
	"program_enrollments",
}

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
	)
	if err != nil {
		return err
	}

	if err := migrateMealIndexes(db); err != nil {
		return err
	}

	if err := migrateNutrientIndexes(db); err != nil {
		return err
	}

	if err := dropObsoleteTables(db); err != nil {
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
