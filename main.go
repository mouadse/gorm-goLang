package main

import (
	"log"

	"fitness-tracker/database"
	"fitness-tracker/models"
)

func main() {
	// Connect to the database
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}

	// AutoMigrate all models — creates/updates tables to match struct definitions
	log.Println("🔄 Running database migrations...")

	err = db.AutoMigrate(
		&models.User{},
		&models.Exercise{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutProgram{},
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
		log.Fatalf("❌ Migration failed: %v", err)
	}

	log.Println("✅ All migrations completed successfully!")
	log.Println("📋 Tables created:")
	log.Println("   - users")
	log.Println("   - exercises")
	log.Println("   - workouts")
	log.Println("   - workout_exercises")
	log.Println("   - workout_programs")
	log.Println("   - foods")
	log.Println("   - meals")
	log.Println("   - meal_foods")
	log.Println("   - weight_entries")
	log.Println("   - friendships")
	log.Println("   - messages")
	log.Println("   - notifications")
	log.Println("   - weekly_adjustments")
}
