package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"fitness-tracker/models"
)

func TestCardioEntryCRUD(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "cardio@example.com", "CardioUser", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":         "Bench Press",
		"primary_muscles": "Chest",
		"equipment":    "Barbell",
	}, http.StatusCreated)

	t.Run("create cardio entry for workout", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id":  user.ID,
			"date":     "2026-03-07",
			"duration": 45,
			"type":     "cardio",
			"notes":    "Morning cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
			"distance":         5.5,
			"distance_unit":    "km",
			"calories_burned":  300,
			"notes":            "Good morning run",
		}, http.StatusCreated)

		if cardio.Modality != "running" {
			t.Fatalf("expected modality running, got %s", cardio.Modality)
		}
		if cardio.DurationMinutes != 30 {
			t.Fatalf("expected duration 30, got %d", cardio.DurationMinutes)
		}
		if cardio.Distance == nil || *cardio.Distance != 5.5 {
			t.Fatalf("expected distance 5.5, got %v", cardio.Distance)
		}
		if cardio.Notes != "Good morning run" {
			t.Fatalf("expected notes 'Good morning run', got %s", cardio.Notes)
		}
	})

	t.Run("create cardio entry with minimal fields", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-08",
			"type":    "cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "cycling",
			"duration_minutes": 45,
		}, http.StatusCreated)

		if cardio.Modality != "cycling" {
			t.Fatalf("expected modality cycling, got %s", cardio.Modality)
		}
		if cardio.Distance != nil {
			t.Fatalf("expected distance to be nil, got %v", cardio.Distance)
		}
	})

	t.Run("reject negative duration", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-09",
			"type":    "cardio",
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": -5,
		}, http.StatusBadRequest)
	})

	t.Run("reject negative distance", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-10",
			"type":    "cardio",
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
			"distance":         -5.0,
		}, http.StatusBadRequest)
	})

	t.Run("reject negative calories", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-11",
			"type":    "cardio",
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
			"calories_burned":  -100,
		}, http.StatusBadRequest)
	})

	t.Run("update cardio entry", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-12",
			"type":    "cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "swimming",
			"duration_minutes": 30,
		}, http.StatusCreated)

		distance := 1.5
		unit := "km"
		updated := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/workout-cardio/"+cardio.ID.String(), map[string]any{
			"distance":      distance,
			"distance_unit": unit,
			"notes":         "Pool session",
		}, http.StatusOK)

		if updated.Distance == nil || *updated.Distance != 1.5 {
			t.Fatalf("expected distance 1.5, got %v", updated.Distance)
		}
		if updated.Notes != "Pool session" {
			t.Fatalf("expected notes 'Pool session', got %s", updated.Notes)
		}
	})

	t.Run("delete cardio entry", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-13",
			"type":    "cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "rowing",
			"duration_minutes": 20,
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/workout-cardio/"+cardio.ID.String(), nil, http.StatusNoContent)
		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/workout-cardio/"+cardio.ID.String(), nil, http.StatusNotFound)
	})

	t.Run("list cardio entries for workout", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-14",
			"type":    "cardio",
		}, http.StatusCreated)

		_ = requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
		}, http.StatusCreated)

		_ = requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "cycling",
			"duration_minutes": 20,
		}, http.StatusCreated)

		entries := requestJSONAuth[[]models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String()+"/cardio", nil, http.StatusOK)

		if len(entries) != 2 {
			t.Fatalf("expected 2 cardio entries, got %d", len(entries))
		}
	})

	t.Run("mixed resistance and cardio workout", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-15",
			"type":    "mixed",
			"exercises": []map[string]any{
				{
					"exercise_id": exercise.ID,
					"order":       1,
					"sets":        3,
					"reps":        10,
					"weight":      50,
				},
			},
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 15,
		}, http.StatusCreated)

		loadedWorkout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String(), nil, http.StatusOK)

		if len(loadedWorkout.WorkoutExercises) != 1 {
			t.Fatalf("expected 1 workout exercise, got %d", len(loadedWorkout.WorkoutExercises))
		}

		cardioEntries := requestJSONAuth[[]models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String()+"/cardio", nil, http.StatusOK)

		if len(cardioEntries) != 1 {
			t.Fatalf("expected 1 cardio entry, got %d", len(cardioEntries))
		}
		if cardioEntries[0].ID != cardio.ID {
			t.Fatalf("expected cardio entry ID %s, got %s", cardio.ID, cardioEntries[0].ID)
		}
	})

	t.Run("authorization check - user cannot access other user's cardio", func(t *testing.T) {
		otherAuth := registerTestUser(t, server, "othercardio@example.com", "OtherCardioUser", "password123")

		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-16",
			"type":    "cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
		}, http.StatusCreated)

		expectStatusAuth(t, server, otherAuth.AccessToken, http.MethodGet, "/v1/workout-cardio/"+cardio.ID.String(), nil, http.StatusForbidden)
	})

	t.Run("reject blank modality", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-23",
			"type":    "cardio",
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "",
			"duration_minutes": 30,
		}, http.StatusBadRequest)
	})

	t.Run("reject invalid modality", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-24",
			"type":    "cardio",
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "invalid_exercise",
			"duration_minutes": 30,
		}, http.StatusBadRequest)
	})

	t.Run("accept valid modalities", func(t *testing.T) {
		validModalities := []string{"running", "cycling", "walking"}

		for i, modality := range validModalities {
			day := 20 + i
			workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
				"user_id": user.ID,
				"date":    fmt.Sprintf("2026-03-%d", day),
				"type":    "cardio",
			}, http.StatusCreated)

			cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
				"modality":         modality,
				"duration_minutes": 30,
			}, http.StatusCreated)

			if cardio.Modality != modality {
				t.Fatalf("expected modality %s, got %s", modality, cardio.Modality)
			}
		}
	})

	t.Run("cascade delete - deleting workout deletes cardio entries", func(t *testing.T) {
		workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    "2026-03-17",
			"type":    "cardio",
		}, http.StatusCreated)

		cardio := requestJSONAuth[models.WorkoutCardioEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/cardio", map[string]any{
			"modality":         "running",
			"duration_minutes": 30,
		}, http.StatusCreated)

		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/workouts/"+workout.ID.String(), nil, http.StatusNoContent)

		// Verify cardio entry is also deleted (soft-deleted)
		expectStatusAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/workout-cardio/"+cardio.ID.String(), nil, http.StatusNotFound)
	})
}
