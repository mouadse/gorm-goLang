package models_test

import (
	"testing"

	"fitness-tracker/models"
	"github.com/google/uuid"
)

func TestWorkoutCardioEntryBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
		}

		if entry.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := entry.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if entry.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("keeps UUID when provided", func(t *testing.T) {
		existingID := uuid.New()
		entry := models.WorkoutCardioEntry{
			ID:              existingID,
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
		}

		if err := entry.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if entry.ID != existingID {
			t.Fatalf("expected ID to remain %v, got %v", existingID, entry.ID)
		}
	})

	t.Run("rejects null workout_id", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			Modality:        "running",
			DurationMinutes: 30,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null workout_id, got nil")
		}
	})

	t.Run("rejects zero duration", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 0,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for zero duration, got nil")
		}
	})

	t.Run("rejects negative duration", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: -5,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative duration, got nil")
		}
	})

	t.Run("rejects negative distance", func(t *testing.T) {
		distance := -5.0
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
			Distance:        &distance,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative distance, got nil")
		}
	})

	t.Run("rejects negative calories", func(t *testing.T) {
		calories := -100
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
			CaloriesBurned:  &calories,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative calories, got nil")
		}
	})

	t.Run("rejects negative heart rate", func(t *testing.T) {
		heartRate := -60
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
			AvgHeartRate:    &heartRate,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for negative heart rate, got nil")
		}
	})

	t.Run("accepts valid entry with all fields", func(t *testing.T) {
		distance := 5.5
		unit := "km"
		pace := 6.2
		calories := 300
		heartRate := 145
		notes := "Good run"

		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "running",
			DurationMinutes: 30,
			Distance:        &distance,
			DistanceUnit:    &unit,
			Pace:            &pace,
			CaloriesBurned:  &calories,
			AvgHeartRate:    &heartRate,
			Notes:           notes,
		}

		if err := entry.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error for valid entry, got %v", err)
		}

		if entry.ID == uuid.Nil {
			t.Fatalf("expected ID to be set")
		}
	})

	t.Run("accepts valid entry with minimal fields", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "cycling",
			DurationMinutes: 45,
		}

		if err := entry.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error for valid minimal entry, got %v", err)
		}
	})

	t.Run("rejects blank modality", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "",
			DurationMinutes: 30,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for blank modality, got nil")
		}
		if err.Error() != "modality is required" {
			t.Fatalf("expected 'modality is required' error, got %v", err)
		}
	})

	t.Run("rejects whitespace-only modality", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "   ",
			DurationMinutes: 30,
		}

		err := entry.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for whitespace-only modality, got nil")
		}
		if err.Error() != "modality is required" {
			t.Fatalf("expected 'modality is required' error, got %v", err)
		}
	})

	t.Run("accepts custom modality", func(t *testing.T) {
		entry := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "swimming",
			DurationMinutes: 30,
		}

		if err := entry.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error for valid modality 'swimming', got %v", err)
		}

		entry2 := models.WorkoutCardioEntry{
			WorkoutID:       uuid.New(),
			Modality:        "trail-run-intervals",
			DurationMinutes: 30,
		}

		if err := entry2.BeforeCreate(nil); err != nil {
			t.Fatalf("expected custom modality to be accepted, got %v", err)
		}
	})

	t.Run("accepts all valid modalities", func(t *testing.T) {
		validModalities := []string{"running", "cycling", "walking", "swimming", "rowing", "cardio", "elliptical", "stairmaster", "hiking", "other"}

		for _, modality := range validModalities {
			entry := models.WorkoutCardioEntry{
				WorkoutID:       uuid.New(),
				Modality:        modality,
				DurationMinutes: 30,
			}

			if err := entry.BeforeCreate(nil); err != nil {
				t.Fatalf("expected no error for valid modality '%s', got %v", modality, err)
			}
		}
	})
}
