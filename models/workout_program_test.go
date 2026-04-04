package models_test

import (
	"testing"

	"fitness-tracker/models"
	"github.com/google/uuid"
)

func TestWorkoutProgramBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		adminID := uuid.New()
		program := models.WorkoutProgram{
			Name:        "12-Week Strength Program",
			Description: "Progressive strength training",
			CreatedBy:   adminID,
			IsActive:    true,
		}

		if program.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := program.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if program.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		program := models.WorkoutProgram{
			CreatedBy: uuid.New(),
		}

		err := program.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for empty name, got nil")
		}
	})
}

func TestProgramWeekBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		week := models.ProgramWeek{
			ProgramID:  uuid.New(),
			WeekNumber: 1,
			Name:       "Week 1",
		}

		if week.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := week.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if week.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null program_id", func(t *testing.T) {
		week := models.ProgramWeek{
			WeekNumber: 1,
		}

		err := week.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null program_id, got nil")
		}
	})
}

func TestProgramSessionBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		session := models.ProgramSession{
			WeekID:    uuid.New(),
			DayNumber: 1,
		}

		if session.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := session.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if session.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null week_id", func(t *testing.T) {
		session := models.ProgramSession{
			DayNumber: 1,
		}

		err := session.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null week_id, got nil")
		}
	})
}

func TestProgramAssignmentBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		assignment := models.ProgramAssignment{
			UserID:    uuid.New(),
			ProgramID: uuid.New(),
			Status:    "assigned",
		}

		if assignment.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := assignment.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if assignment.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null user_id", func(t *testing.T) {
		assignment := models.ProgramAssignment{
			ProgramID: uuid.New(),
			Status:    "assigned",
		}

		err := assignment.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null user_id, got nil")
		}
	})

	t.Run("rejects null program_id", func(t *testing.T) {
		assignment := models.ProgramAssignment{
			UserID: uuid.New(),
			Status: "assigned",
		}

		err := assignment.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null program_id, got nil")
		}
	})

	t.Run("sets default status to assigned", func(t *testing.T) {
		assignment := models.ProgramAssignment{
			UserID:    uuid.New(),
			ProgramID: uuid.New(),
		}

		if err := assignment.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if assignment.Status != "assigned" {
			t.Fatalf("expected default status 'assigned', got %s", assignment.Status)
		}
	})
}
