package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fitness-tracker/api"
	"fitness-tracker/database"
	"fitness-tracker/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCoreCRUDFlow(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	user := requestJSON[models.User](t, server, http.MethodPost, "/v1/users", map[string]any{
		"email": "alex@example.com",
		"name":  "Alex",
	}, http.StatusCreated)

	exercise := requestJSON[models.Exercise](t, server, http.MethodPost, "/v1/exercises", map[string]any{
		"name":         "Bench Press",
		"muscle_group": "Chest",
		"equipment":    "Barbell",
	}, http.StatusCreated)

	workout := requestJSON[models.Workout](t, server, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id":  user.ID,
		"date":     "2026-03-07",
		"duration": 55,
		"type":     "push",
		"notes":    "Heavy day",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        3,
				"reps":        8,
				"weight":      80,
				"rest_time":   90,
				"set_entries": []map[string]any{
					{"set_number": 1, "reps": 8, "weight": 80, "rpe": 8.0},
					{"set_number": 2, "reps": 8, "weight": 80, "rpe": 8.5},
				},
			},
		},
	}, http.StatusCreated)

	loadedWorkout := requestJSON[models.Workout](t, server, http.MethodGet, "/v1/workouts/"+workout.ID.String(), nil, http.StatusOK)
	if len(loadedWorkout.WorkoutExercises) != 1 {
		t.Fatalf("expected 1 workout exercise, got %d", len(loadedWorkout.WorkoutExercises))
	}
	if len(loadedWorkout.WorkoutExercises[0].WorkoutSets) != 2 {
		t.Fatalf("expected 2 workout sets, got %d", len(loadedWorkout.WorkoutExercises[0].WorkoutSets))
	}

	requestJSON[models.Workout](t, server, http.MethodPatch, "/v1/workouts/"+workout.ID.String(), map[string]any{
		"duration": 60,
		"notes":    "Updated heavy day",
	}, http.StatusOK)

	addedWorkoutExercise := requestJSON[models.WorkoutExercise](t, server, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/exercises", map[string]any{
		"exercise_id": exercise.ID,
		"order":       2,
		"sets":        4,
		"reps":        10,
		"weight":      22.5,
		"set_entries": []map[string]any{
			{"reps": 10, "weight": 22.5, "rpe": 7.5},
		},
	}, http.StatusCreated)

	listedWorkoutExercises := requestJSON[[]models.WorkoutExercise](t, server, http.MethodGet, "/v1/workouts/"+workout.ID.String()+"/exercises", nil, http.StatusOK)
	if len(listedWorkoutExercises) != 2 {
		t.Fatalf("expected 2 workout exercises, got %d", len(listedWorkoutExercises))
	}

	updatedWorkoutExercise := requestJSON[models.WorkoutExercise](t, server, http.MethodPatch, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String(), map[string]any{
		"notes": "Accessory volume",
		"sets":  5,
	}, http.StatusOK)
	if updatedWorkoutExercise.Sets != 5 {
		t.Fatalf("expected workout exercise sets to be updated")
	}

	set := requestJSON[models.WorkoutSet](t, server, http.MethodPost, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String()+"/sets", map[string]any{
		"reps":         12,
		"weight":       20,
		"rest_seconds": 60,
	}, http.StatusCreated)
	if set.SetNumber != 2 {
		t.Fatalf("expected auto-assigned set number 2, got %d", set.SetNumber)
	}

	updatedSet := requestJSON[models.WorkoutSet](t, server, http.MethodPatch, "/v1/workout-sets/"+set.ID.String(), map[string]any{
		"completed": false,
	}, http.StatusOK)
	if updatedSet.Completed {
		t.Fatalf("expected updated set to be incomplete")
	}

	meal := requestJSON[models.Meal](t, server, http.MethodPost, "/v1/meals", map[string]any{
		"user_id":   user.ID,
		"meal_type": "dinner",
		"date":      "2026-03-07",
		"notes":     "Post-workout meal",
	}, http.StatusCreated)

	weightEntry := requestJSON[models.WeightEntry](t, server, http.MethodPost, "/v1/weight-entries", map[string]any{
		"user_id": user.ID,
		"weight":  82.4,
		"date":    "2026-03-07",
		"notes":   "Morning weigh-in",
	}, http.StatusCreated)

	loadedMeal := requestJSON[models.Meal](t, server, http.MethodGet, "/v1/meals/"+meal.ID.String(), nil, http.StatusOK)
	if loadedMeal.MealType != "dinner" {
		t.Fatalf("expected meal type dinner, got %q", loadedMeal.MealType)
	}

	listedMeals := requestJSON[[]models.Meal](t, server, http.MethodGet, "/v1/users/"+user.ID.String()+"/meals", nil, http.StatusOK)
	if len(listedMeals) != 1 {
		t.Fatalf("expected 1 meal, got %d", len(listedMeals))
	}

	loadedWeightEntry := requestJSON[models.WeightEntry](t, server, http.MethodGet, "/v1/weight-entries/"+weightEntry.ID.String(), nil, http.StatusOK)
	if loadedWeightEntry.Weight != 82.4 {
		t.Fatalf("expected weight entry to round-trip")
	}

	listedWeightEntries := requestJSON[[]models.WeightEntry](t, server, http.MethodGet, "/v1/users/"+user.ID.String()+"/weight-entries", nil, http.StatusOK)
	if len(listedWeightEntries) != 1 {
		t.Fatalf("expected 1 weight entry, got %d", len(listedWeightEntries))
	}

	requestJSON[models.Meal](t, server, http.MethodPatch, "/v1/meals/"+meal.ID.String(), map[string]any{
		"notes": "Updated meal notes",
	}, http.StatusOK)

	requestJSON[models.WeightEntry](t, server, http.MethodPatch, "/v1/weight-entries/"+weightEntry.ID.String(), map[string]any{
		"weight": 81.9,
	}, http.StatusOK)

	expectStatus(t, server, http.MethodDelete, "/v1/workout-sets/"+set.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/meals/"+meal.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/weight-entries/"+weightEntry.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/workouts/"+workout.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/exercises/"+exercise.ID.String(), nil, http.StatusNoContent)
	expectStatus(t, server, http.MethodDelete, "/v1/users/"+user.ID.String(), nil, http.StatusNoContent)
}

func TestCreateUserRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	expectStatus(t, server, http.MethodPost, "/v1/users", map[string]any{
		"email":       "alex@example.com",
		"name":        "Alex",
		"unknown_key": true,
	}, http.StatusBadRequest)
}

func TestDocsEndpoints(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	specRequest := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	specRecorder := httptest.NewRecorder()
	server.ServeHTTP(specRecorder, specRequest)

	if specRecorder.Code != http.StatusOK {
		t.Fatalf("GET /openapi.yaml: expected status 200, got %d", specRecorder.Code)
	}
	if got := specRecorder.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("GET /openapi.yaml: unexpected content type %q", got)
	}
	if !bytes.Contains(specRecorder.Body.Bytes(), []byte("openapi: 3.0.3")) {
		t.Fatalf("GET /openapi.yaml: expected OpenAPI document")
	}

	docsRequest := httptest.NewRequest(http.MethodGet, "/docs", nil)
	docsRecorder := httptest.NewRecorder()
	server.ServeHTTP(docsRecorder, docsRequest)

	if docsRecorder.Code != http.StatusOK {
		t.Fatalf("GET /docs: expected status 200, got %d", docsRecorder.Code)
	}
	if got := docsRecorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("GET /docs: unexpected content type %q", got)
	}
	if !bytes.Contains(docsRecorder.Body.Bytes(), []byte(`url: "/openapi.yaml"`)) {
		t.Fatalf("GET /docs: expected Swagger UI to reference /openapi.yaml")
	}
}

func TestUserScopedCreateRoutes(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	user := requestJSON[models.User](t, server, http.MethodPost, "/v1/users", map[string]any{
		"email": "scoped@example.com",
		"name":  "Scoped User",
	}, http.StatusCreated)

	exercise := requestJSON[models.Exercise](t, server, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Front Squat",
	}, http.StatusCreated)

	workout := requestJSON[models.Workout](t, server, http.MethodPost, "/v1/users/"+user.ID.String()+"/workouts", map[string]any{
		"date":     "2026-03-08",
		"duration": 42,
		"type":     "legs",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"reps":        5,
				"weight":      100,
			},
		},
	}, http.StatusCreated)
	if workout.UserID != user.ID {
		t.Fatalf("expected scoped workout to inherit user id")
	}

	meal := requestJSON[models.Meal](t, server, http.MethodPost, "/v1/users/"+user.ID.String()+"/meals", map[string]any{
		"meal_type": "lunch",
		"date":      "2026-03-08",
		"notes":     "Scoped meal",
	}, http.StatusCreated)
	if meal.UserID != user.ID {
		t.Fatalf("expected scoped meal to inherit user id")
	}

	weightEntry := requestJSON[models.WeightEntry](t, server, http.MethodPost, "/v1/users/"+user.ID.String()+"/weight-entries", map[string]any{
		"weight": 79.3,
		"date":   "2026-03-08",
	}, http.StatusCreated)
	if weightEntry.UserID != user.ID {
		t.Fatalf("expected scoped weight entry to inherit user id")
	}

	workouts := requestJSON[[]models.Workout](t, server, http.MethodGet, "/v1/users/"+user.ID.String()+"/workouts", nil, http.StatusOK)
	if len(workouts) != 1 {
		t.Fatalf("expected 1 scoped workout, got %d", len(workouts))
	}
}

func TestNestedCreateRoutesRejectMismatchedIDs(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	user := requestJSON[models.User](t, server, http.MethodPost, "/v1/users", map[string]any{
		"email": "mismatch@example.com",
		"name":  "Mismatch User",
	}, http.StatusCreated)
	otherUser := requestJSON[models.User](t, server, http.MethodPost, "/v1/users", map[string]any{
		"email": "other@example.com",
		"name":  "Other User",
	}, http.StatusCreated)

	exercise := requestJSON[models.Exercise](t, server, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Pull Up",
	}, http.StatusCreated)

	workout := requestJSON[models.Workout](t, server, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"type":    "pull",
	}, http.StatusCreated)
	otherWorkout := requestJSON[models.Workout](t, server, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": otherUser.ID,
		"type":    "push",
	}, http.StatusCreated)

	workoutExercise := requestJSON[models.WorkoutExercise](t, server, http.MethodPost, "/v1/workout-exercises", map[string]any{
		"workout_id":  workout.ID,
		"exercise_id": exercise.ID,
		"reps":        8,
	}, http.StatusCreated)

	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{
			name: "user scoped workout",
			path: "/v1/users/" + user.ID.String() + "/workouts",
			body: map[string]any{
				"user_id": otherUser.ID,
				"type":    "legs",
			},
		},
		{
			name: "user scoped meal",
			path: "/v1/users/" + user.ID.String() + "/meals",
			body: map[string]any{
				"user_id":   otherUser.ID,
				"meal_type": "dinner",
			},
		},
		{
			name: "user scoped weight entry",
			path: "/v1/users/" + user.ID.String() + "/weight-entries",
			body: map[string]any{
				"user_id": otherUser.ID,
				"weight":  77.7,
			},
		},
		{
			name: "workout exercise",
			path: "/v1/workouts/" + workout.ID.String() + "/exercises",
			body: map[string]any{
				"workout_id":  otherWorkout.ID,
				"exercise_id": exercise.ID,
				"reps":        10,
			},
		},
		{
			name: "workout set",
			path: "/v1/workout-exercises/" + workoutExercise.ID.String() + "/sets",
			body: map[string]any{
				"workout_exercise_id": "10000000-0000-0000-0000-000000000001",
				"reps":                6,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectStatus(t, server, http.MethodPost, tt.path, tt.body, http.StatusBadRequest)
		})
	}
}

func TestCreateWorkoutReturnsNotFoundForMissingExercise(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	user := requestJSON[models.User](t, server, http.MethodPost, "/v1/users", map[string]any{
		"email": "missing-exercise@example.com",
		"name":  "Missing Exercise",
	}, http.StatusCreated)

	errBody := requestError(t, server, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"type":    "push",
		"exercises": []map[string]any{
			{
				"exercise_id": "10000000-0000-0000-0000-000000000001",
				"reps":        8,
			},
		},
	}, http.StatusNotFound)

	if errBody["error"] != "exercise not found" {
		t.Fatalf("expected exercise not found error, got %q", errBody["error"])
	}
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return api.NewServer(db).Handler()
}

func requestJSON[T any](t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) T {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}

	var result T
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("%s %s: decode response: %v", method, path, err)
	}

	return result
}

func expectStatus(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}
}

func requestError(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) map[string]string {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}

	var result map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("%s %s: decode error response: %v", method, path, err)
	}

	return result
}

func encodeBody(t *testing.T, body any) *bytes.Reader {
	t.Helper()

	if body == nil {
		return bytes.NewReader(nil)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	return bytes.NewReader(payload)
}
