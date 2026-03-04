package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"fitness-tracker/api"
	"fitness-tracker/database"
	"fitness-tracker/models"
	"gorm.io/gorm"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := runMigrations(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	port := getEnvOrDefault("PORT", "8080")
	addr := fmt.Sprintf(":%s", port)

	server := api.NewServer(db)
	log.Printf("fitness-tracker API listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func runMigrations(db *gorm.DB) error {
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

func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
