package services_test

import (
	"testing"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWorkoutTemplateService(t *testing.T) {
	t.Parallel()

	db := setupTestDB(t)
	svc := services.NewWorkoutTemplateService(db)

	user := createTestUser(t, db, "template@example.com")
	exercise1 := createTestExercise(t, db, "Bench Press", "Chest")
	exercise2 := createTestExercise(t, db, "Squat", "Legs")

	t.Run("apply template creates workout with exercises and sets", func(t *testing.T) {
		template := createTestTemplate(t, db, user.ID, exercise1.ID, exercise2.ID)

		workout, err := svc.ApplyTemplate(template.ID, user.ID, time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if workout.ID == uuid.Nil {
			t.Fatalf("expected workout ID to be set")
		}

		if len(workout.WorkoutExercises) != 2 {
			t.Fatalf("expected 2 workout exercises, got %d", len(workout.WorkoutExercises))
		}

		firstExercise := workout.WorkoutExercises[0]
		if firstExercise.ExerciseID != exercise1.ID {
			t.Fatalf("expected first exercise ID %s, got %s", exercise1.ID, firstExercise.ExerciseID)
		}

		if len(firstExercise.WorkoutSets) != 3 {
			t.Fatalf("expected 3 sets for first exercise, got %d", len(firstExercise.WorkoutSets))
		}

		secondExercise := workout.WorkoutExercises[1]
		if secondExercise.ExerciseID != exercise2.ID {
			t.Fatalf("expected second exercise ID %s, got %s", exercise2.ID, secondExercise.ExerciseID)
		}

		if len(secondExercise.WorkoutSets) != 2 {
			t.Fatalf("expected 2 sets for second exercise, got %d", len(secondExercise.WorkoutSets))
		}
	})

	t.Run("editing template does not affect created workouts", func(t *testing.T) {
		template := createTestTemplate(t, db, user.ID, exercise1.ID, exercise2.ID)

		workout1, err := svc.ApplyTemplate(template.ID, user.ID, time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		template.Name = "Updated Name"
		template.WorkoutTemplateExercises[0].Sets = 10
		if err := db.Save(&template).Error; err != nil {
			t.Fatalf("failed to update template: %v", err)
		}

		if err := db.Save(&template.WorkoutTemplateExercises[0]).Error; err != nil {
			t.Fatalf("failed to update template exercise: %v", err)
		}

		var originalWorkout models.Workout
		if err := db.Preload("WorkoutExercises.WorkoutSets").First(&originalWorkout, "id = ?", workout1.ID).Error; err != nil {
			t.Fatalf("failed to load workout: %v", err)
		}

		if len(originalWorkout.WorkoutExercises[0].WorkoutSets) != 3 {
			t.Fatalf("expected original workout to still have 3 sets, got %d", len(originalWorkout.WorkoutExercises[0].WorkoutSets))
		}
	})

	t.Run("apply preserves exercise order", func(t *testing.T) {
		template := models.WorkoutTemplate{
			OwnerID: user.ID,
			Name:    "Ordered Template",
			Type:    "test",
			WorkoutTemplateExercises: []models.WorkoutTemplateExercise{
				{
					ExerciseID: exercise1.ID,
					Order:      2,
					Sets:       1,
					WorkoutTemplateSets: []models.WorkoutTemplateSet{
						{SetNumber: 1, Reps: 10},
					},
				},
				{
					ExerciseID: exercise2.ID,
					Order:      1,
					Sets:       1,
					WorkoutTemplateSets: []models.WorkoutTemplateSet{
						{SetNumber: 1, Reps: 12},
					},
				},
			},
		}

		if err := db.Create(&template).Error; err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		workout, err := svc.ApplyTemplate(template.ID, user.ID, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(workout.WorkoutExercises) != 2 {
			t.Fatalf("expected 2 workout exercises, got %d", len(workout.WorkoutExercises))
		}

		// First exercise should be the one with order=1 (exercise2)
		if workout.WorkoutExercises[0].Order != 1 {
			t.Fatalf("expected first exercise order 1, got %d", workout.WorkoutExercises[0].Order)
		}

		if workout.WorkoutExercises[0].ExerciseID != exercise2.ID {
			t.Fatalf("expected first exercise to be exercise2 (order=1), got exercise1")
		}

		// Second exercise should be the one with order=2 (exercise1)
		if workout.WorkoutExercises[1].Order != 2 {
			t.Fatalf("expected second exercise order 2, got %d", workout.WorkoutExercises[1].Order)
		}

		if workout.WorkoutExercises[1].ExerciseID != exercise1.ID {
			t.Fatalf("expected second exercise to be exercise1 (order=2), got exercise2")
		}
	})

	t.Run("apply template with nonexistent template returns error", func(t *testing.T) {
		_, err := svc.ApplyTemplate(uuid.New(), user.ID, time.Now())
		if err == nil {
			t.Fatalf("expected error for nonexistent template, got nil")
		}
	})

	t.Run("apply template preserves weight values", func(t *testing.T) {
		template := models.WorkoutTemplate{
			OwnerID: user.ID,
			Name:    "Weight Template",
			Type:    "test",
			WorkoutTemplateExercises: []models.WorkoutTemplateExercise{
				{
					ExerciseID: exercise1.ID,
					Order:      1,
					Sets:       2,
					Weight:     100.5,
					WorkoutTemplateSets: []models.WorkoutTemplateSet{
						{SetNumber: 1, Reps: 10, Weight: 100.5},
						{SetNumber: 2, Reps: 8, Weight: 105.0},
					},
				},
			},
		}

		if err := db.Create(&template).Error; err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		workout, err := svc.ApplyTemplate(template.ID, user.ID, time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if workout.WorkoutExercises[0].Weight != 100.5 {
			t.Fatalf("expected exercise weight 100.5, got %f", workout.WorkoutExercises[0].Weight)
		}

		if workout.WorkoutExercises[0].WorkoutSets[0].Weight != 100.5 {
			t.Fatalf("expected first set weight 100.5, got %f", workout.WorkoutExercises[0].WorkoutSets[0].Weight)
		}

		if workout.WorkoutExercises[0].WorkoutSets[1].Weight != 105.0 {
			t.Fatalf("expected second set weight 105.0, got %f", workout.WorkoutExercises[0].WorkoutSets[1].Weight)
		}
	})

	t.Run("apply template with notes preserves them", func(t *testing.T) {
		template := models.WorkoutTemplate{
			OwnerID: user.ID,
			Name:    "Notes Template",
			Type:    "push",
			Notes:   "Focus on form",
			WorkoutTemplateExercises: []models.WorkoutTemplateExercise{
				{
					ExerciseID: exercise1.ID,
					Order:      1,
					Sets:       1,
					Notes:      "Keep back straight",
					WorkoutTemplateSets: []models.WorkoutTemplateSet{
						{SetNumber: 1, Reps: 10},
					},
				},
			},
		}

		if err := db.Create(&template).Error; err != nil {
			t.Fatalf("failed to create template: %v", err)
		}

		workout, err := svc.ApplyTemplate(template.ID, user.ID, time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if workout.Notes != "Focus on form" {
			t.Fatalf("expected workout notes 'Focus on form', got %s", workout.Notes)
		}

		if workout.WorkoutExercises[0].Notes != "Keep back straight" {
			t.Fatalf("expected exercise notes 'Keep back straight', got %s", workout.WorkoutExercises[0].Notes)
		}
	})
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Exercise{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.WorkoutTemplate{},
		&models.WorkoutTemplateExercise{},
		&models.WorkoutTemplateSet{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func createTestUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()

	user := models.User{
		Email:        email,
		Name:         "Test User",
		PasswordHash: "hashedpassword",
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return &user
}

func createTestExercise(t *testing.T, db *gorm.DB, name, muscleGroup string) *models.Exercise {
	t.Helper()

	exercise := models.Exercise{
		Name:        name,
		MuscleGroup: muscleGroup,
	}

	if err := db.Create(&exercise).Error; err != nil {
		t.Fatalf("failed to create exercise: %v", err)
	}

	return &exercise
}

func createTestTemplate(t *testing.T, db *gorm.DB, userID, exercise1ID, exercise2ID uuid.UUID) *models.WorkoutTemplate {
	t.Helper()

	template := models.WorkoutTemplate{
		OwnerID: userID,
		Name:    "Test Template",
		Type:    "push",
		WorkoutTemplateExercises: []models.WorkoutTemplateExercise{
			{
				ExerciseID: exercise1ID,
				Order:      1,
				Sets:       3,
				Reps:       10,
				Weight:     100,
				RestTime:   90,
				WorkoutTemplateSets: []models.WorkoutTemplateSet{
					{SetNumber: 1, Reps: 10, Weight: 100, RestSeconds: 90},
					{SetNumber: 2, Reps: 10, Weight: 105, RestSeconds: 90},
					{SetNumber: 3, Reps: 8, Weight: 110, RestSeconds: 120},
				},
			},
			{
				ExerciseID: exercise2ID,
				Order:      2,
				Sets:       2,
				Reps:       12,
				Weight:     80,
				WorkoutTemplateSets: []models.WorkoutTemplateSet{
					{SetNumber: 1, Reps: 12, Weight: 80, RestSeconds: 60},
					{SetNumber: 2, Reps: 12, Weight: 85, RestSeconds: 60},
				},
			},
		},
	}

	if err := db.Create(&template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	return &template
}
