package models_test

import (
	"testing"

	"fitness-tracker/models"
	"github.com/google/uuid"
)

func TestWorkoutTemplateBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		template := models.WorkoutTemplate{
			OwnerID: uuid.New(),
			Name:    "Push Day",
			Type:    "push",
		}

		if template.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := template.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if template.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null owner_id", func(t *testing.T) {
		template := models.WorkoutTemplate{
			Name: "Push Day",
			Type: "push",
		}

		err := template.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null owner_id, got nil")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		template := models.WorkoutTemplate{
			OwnerID: uuid.New(),
			Name:    "",
			Type:    "push",
		}

		err := template.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for empty name, got nil")
		}
	})
}

func TestWorkoutTemplateExerciseBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		exercise := models.WorkoutTemplateExercise{
			TemplateID: uuid.New(),
			ExerciseID: uuid.New(),
			Order:      1,
		}

		if exercise.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := exercise.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if exercise.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null template_id", func(t *testing.T) {
		exercise := models.WorkoutTemplateExercise{
			ExerciseID: uuid.New(),
			Order:      1,
		}

		err := exercise.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null template_id, got nil")
		}
	})

	t.Run("rejects null exercise_id", func(t *testing.T) {
		exercise := models.WorkoutTemplateExercise{
			TemplateID: uuid.New(),
			Order:      1,
		}

		err := exercise.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null exercise_id, got nil")
		}
	})

	t.Run("rejects zero or negative order", func(t *testing.T) {
		exercise := models.WorkoutTemplateExercise{
			TemplateID: uuid.New(),
			ExerciseID: uuid.New(),
			Order:      0,
		}

		err := exercise.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for zero order, got nil")
		}

		exercise.Order = -1
		err = exercise.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative order, got nil")
		}
	})
}

func TestWorkoutTemplateSetBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		set := models.WorkoutTemplateSet{
			TemplateExerciseID: uuid.New(),
			SetNumber:          1,
			Reps:               10,
		}

		if set.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := set.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if set.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null template_exercise_id", func(t *testing.T) {
		set := models.WorkoutTemplateSet{
			SetNumber: 1,
			Reps:      10,
		}

		err := set.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null template_exercise_id, got nil")
		}
	})

	t.Run("rejects zero or negative set_number", func(t *testing.T) {
		set := models.WorkoutTemplateSet{
			TemplateExerciseID: uuid.New(),
			SetNumber:          0,
			Reps:               10,
		}

		err := set.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for zero set_number, got nil")
		}

		set.SetNumber = -1
		err = set.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative set_number, got nil")
		}
	})

	t.Run("rejects negative reps", func(t *testing.T) {
		set := models.WorkoutTemplateSet{
			TemplateExerciseID: uuid.New(),
			SetNumber:          1,
			Reps:               -5,
		}

		err := set.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative reps, got nil")
		}
	})

	t.Run("rejects negative weight", func(t *testing.T) {
		set := models.WorkoutTemplateSet{
			TemplateExerciseID: uuid.New(),
			SetNumber:          1,
			Reps:               10,
			Weight:             -50.0,
		}

		err := set.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative weight, got nil")
		}
	})
}
