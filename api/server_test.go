package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"fitness-tracker/api"
	"fitness-tracker/database"
	"fitness-tracker/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	if err := os.Setenv("JWT_SECRET", "test-jwt-secret"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCoreCRUDFlow(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	userAuth := registerTestUser(t, server, "alex@example.com", "Alex", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Bench Press",
		"primary_muscles": "Chest",
		"equipment":       "Barbell",
	}, http.StatusCreated)

	workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
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

	loadedWorkout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String(), nil, http.StatusOK)
	if len(loadedWorkout.WorkoutExercises) != 1 {
		t.Fatalf("expected 1 workout exercise, got %d", len(loadedWorkout.WorkoutExercises))
	}
	if len(loadedWorkout.WorkoutExercises[0].WorkoutSets) != 2 {
		t.Fatalf("expected 2 workout sets, got %d", len(loadedWorkout.WorkoutExercises[0].WorkoutSets))
	}

	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/workouts/"+workout.ID.String(), map[string]any{
		"duration": 60,
		"notes":    "Updated heavy day",
	}, http.StatusOK)

	addedWorkoutExercise := requestJSONAuth[models.WorkoutExercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts/"+workout.ID.String()+"/exercises", map[string]any{
		"exercise_id": exercise.ID,
		"order":       2,
		"sets":        4,
		"reps":        10,
		"weight":      22.5,
		"set_entries": []map[string]any{
			{"reps": 10, "weight": 22.5, "rpe": 7.5},
		},
	}, http.StatusCreated)

	listedWorkoutExercises := requestJSONAuth[api.PaginatedResponse[models.WorkoutExercise]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String()+"/exercises", nil, http.StatusOK).Data
	if len(listedWorkoutExercises) != 2 {
		t.Fatalf("expected 2 workout exercises, got %d", len(listedWorkoutExercises))
	}

	updatedWorkoutExercise := requestJSONAuth[models.WorkoutExercise](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String(), map[string]any{
		"notes": "Accessory volume",
		"sets":  5,
	}, http.StatusOK)
	if updatedWorkoutExercise.Sets != 5 {
		t.Fatalf("expected workout exercise sets to be updated")
	}

	set := requestJSONAuth[models.WorkoutSet](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String()+"/sets", map[string]any{
		"reps":         12,
		"weight":       20,
		"rest_seconds": 60,
	}, http.StatusCreated)
	if set.SetNumber != 2 {
		t.Fatalf("expected auto-assigned set number 2, got %d", set.SetNumber)
	}

	updatedSet := requestJSONAuth[models.WorkoutSet](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/workout-sets/"+set.ID.String(), map[string]any{
		"completed": false,
	}, http.StatusOK)
	if updatedSet.Completed {
		t.Fatalf("expected updated set to be incomplete")
	}

	meal := requestJSONAuth[models.Meal](t, server, userAuth.AccessToken, http.MethodPost, "/v1/meals", map[string]any{
		"user_id":   user.ID,
		"meal_type": "dinner",
		"date":      "2026-03-07",
		"notes":     "Post-workout meal",
	}, http.StatusCreated)

	weightEntry := requestJSONAuth[models.WeightEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/weight-entries", map[string]any{
		"user_id": user.ID,
		"weight":  82.4,
		"date":    "2026-03-07",
		"notes":   "Morning weigh-in",
	}, http.StatusCreated)

	loadedMeal := requestJSONAuth[models.Meal](t, server, userAuth.AccessToken, http.MethodGet, "/v1/meals/"+meal.ID.String(), nil, http.StatusOK)
	if loadedMeal.MealType != "dinner" {
		t.Fatalf("expected meal type dinner, got %q", loadedMeal.MealType)
	}

	listedMeals := requestJSONAuth[api.PaginatedResponse[models.Meal]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/meals", nil, http.StatusOK).Data
	if len(listedMeals) != 1 {
		t.Fatalf("expected 1 meal, got %d", len(listedMeals))
	}

	loadedWeightEntry := requestJSONAuth[models.WeightEntry](t, server, userAuth.AccessToken, http.MethodGet, "/v1/weight-entries/"+weightEntry.ID.String(), nil, http.StatusOK)
	if loadedWeightEntry.Weight != 82.4 {
		t.Fatalf("expected weight entry to round-trip")
	}

	listedWeightEntries := requestJSONAuth[api.PaginatedResponse[models.WeightEntry]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/weight-entries", nil, http.StatusOK).Data
	if len(listedWeightEntries) != 1 {
		t.Fatalf("expected 1 weight entry, got %d", len(listedWeightEntries))
	}

	requestJSONAuth[models.Meal](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/meals/"+meal.ID.String(), map[string]any{
		"notes": "Updated meal notes",
	}, http.StatusOK)

	requestJSONAuth[models.WeightEntry](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/weight-entries/"+weightEntry.ID.String(), map[string]any{
		"weight": 81.9,
	}, http.StatusOK)

	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/workout-sets/"+set.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/workout-exercises/"+addedWorkoutExercise.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/meals/"+meal.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/weight-entries/"+weightEntry.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/workouts/"+workout.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/exercises/"+exercise.ID.String(), nil, http.StatusNoContent)
	expectStatusAuth(t, server, userAuth.AccessToken, http.MethodDelete, "/v1/users/"+user.ID.String(), nil, http.StatusNoContent)
}

func TestCreateUserRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	expectStatus(t, server, http.MethodPost, "/v1/users", map[string]any{
		"email":       "alex@example.com",
		"password":    "password123",
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

	for _, path := range []string{"/login", "/register"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s: expected status 200, got %d", path, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("GET %s: unexpected content type %q", path, got)
		}
	}
}

func TestCORSPreflightAllowsConfiguredOriginBeforeAuth(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example.com")
	t.Setenv("CORS_ALLOWED_HEADERS", "Authorization, Content-Type, X-Client-Version")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")
	t.Setenv("CORS_MAX_AGE_SECONDS", "")

	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodOptions, "/v1/workouts", nil)
	request.Header.Set("Origin", "https://frontend.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type, X-Client-Version")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /v1/workouts: expected status 204, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("unexpected Access-Control-Allow-Origin %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("unexpected Access-Control-Allow-Credentials %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Client-Version" {
		t.Fatalf("unexpected Access-Control-Allow-Headers %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("unexpected Access-Control-Max-Age %q", got)
	}
}

func TestCORSPreflightRejectsUnconfiguredOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example.com")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodOptions, "/v1/workouts", nil)
	request.Header.Set("Origin", "https://malicious.example.com")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("OPTIONS /v1/workouts: expected status 403, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin for rejected origin %q", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("unexpected Vary for rejected origin %q", got)
	}
}

func TestCORSAddsVaryToRejectedOriginResponse(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example.com")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	request.Header.Set("Origin", "https://malicious.example.com")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /livez: expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin for rejected origin %q", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("unexpected Vary for rejected origin %q", got)
	}
}

func TestCORSAddsHeadersToAllowedOriginResponse(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://frontend.example.com")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

	server := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	request.Header.Set("Origin", "https://frontend.example.com")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /livez: expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("unexpected Access-Control-Allow-Origin %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("unexpected Access-Control-Allow-Credentials %q", got)
	}
}

func TestUserScopedCreateRoutes(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	userAuth := registerTestUser(t, server, "scoped@example.com", "Scoped User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Front Squat",
	}, http.StatusCreated)

	workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/users/"+user.ID.String()+"/workouts", map[string]any{
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

	meal := requestJSONAuth[models.Meal](t, server, userAuth.AccessToken, http.MethodPost, "/v1/users/"+user.ID.String()+"/meals", map[string]any{
		"meal_type": "lunch",
		"date":      "2026-03-08",
		"notes":     "Scoped meal",
	}, http.StatusCreated)
	if meal.UserID != user.ID {
		t.Fatalf("expected scoped meal to inherit user id")
	}

	weightEntry := requestJSONAuth[models.WeightEntry](t, server, userAuth.AccessToken, http.MethodPost, "/v1/users/"+user.ID.String()+"/weight-entries", map[string]any{
		"weight": 79.3,
		"date":   "2026-03-08",
	}, http.StatusCreated)
	if weightEntry.UserID != user.ID {
		t.Fatalf("expected scoped weight entry to inherit user id")
	}

	workouts := requestJSONAuth[api.PaginatedResponse[models.Workout]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/workouts", nil, http.StatusOK).Data
	if len(workouts) != 1 {
		t.Fatalf("expected 1 scoped workout, got %d", len(workouts))
	}
}

func TestNestedCreateRoutesRejectMismatchedIDs(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	userAuth := registerTestUser(t, server, "mismatch@example.com", "Mismatch User", "password123")
	user := userAuth.User
	otherAuth := registerTestUser(t, server, "other@example.com", "Other User", "password123")
	otherUser := otherAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Pull Up",
	}, http.StatusCreated)

	workout := requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"type":    "pull",
	}, http.StatusCreated)
	otherWorkout := requestJSONAuth[models.Workout](t, server, otherAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": otherUser.ID,
		"type":    "push",
	}, http.StatusCreated)

	workoutExercise := requestJSONAuth[models.WorkoutExercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workout-exercises", map[string]any{
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
			expectStatusAuth(t, server, userAuth.AccessToken, http.MethodPost, tt.path, tt.body, http.StatusBadRequest)
		})
	}
}

func TestCreateWorkoutReturnsNotFoundForMissingExercise(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	userAuth := registerTestUser(t, server, "missing-exercise@example.com", "Missing Exercise", "password123")
	user := userAuth.User

	errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
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

func TestProtectedRoutesRequireAuth(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	expectStatus(t, server, http.MethodGet, "/v1/users", nil, http.StatusUnauthorized)
	expectStatus(t, server, http.MethodGet, "/v1/workouts", nil, http.StatusUnauthorized)
	expectStatus(t, server, http.MethodPost, "/v1/exercises", map[string]any{"name": "Unauthorized Exercise"}, http.StatusUnauthorized)
}

func TestExerciseReadRoutesStayPublicWhileWritesRequireAuth(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	expectStatus(t, server, http.MethodGet, "/v1/exercises", nil, http.StatusOK)

	userAuth := registerTestUser(t, server, "exercises@example.com", "Exercise Owner", "password123")
	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Seal Row",
	}, http.StatusCreated)

	expectStatus(t, server, http.MethodGet, "/v1/exercises/"+exercise.ID.String(), nil, http.StatusOK)
	expectStatus(t, server, http.MethodPatch, "/v1/exercises/"+exercise.ID.String(), map[string]any{"level": "Advanced"}, http.StatusUnauthorized)
	expectStatus(t, server, http.MethodDelete, "/v1/exercises/"+exercise.ID.String(), nil, http.StatusUnauthorized)
}

func TestExerciseListFilters(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "filters@example.com", "Filter Owner", "password123")

	requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Dumbbell Shoulder Press",
		"primary_muscles": "Shoulders",
		"equipment":       "Dumbbell",
		"level":           "Beginner",
	}, http.StatusCreated)

	requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Band Pull-Apart",
		"primary_muscles": "Back",
		"equipment":       "Resistance Band",
		"level":           "Beginner",
	}, http.StatusCreated)

	requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Barbell Push Press",
		"primary_muscles": "Shoulders",
		"equipment":       "Barbell",
		"level":           "Advanced",
	}, http.StatusCreated)

	requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Air Squat",
		"primary_muscles": "Legs",
		"equipment":       "Bodyweight",
		"level":           "Beginner",
	}, http.StatusCreated)

	shoulderExercises := requestJSON[api.PaginatedResponse[models.Exercise]](t, server, http.MethodGet, "/v1/exercises?muscle=shoulder&equipment=dumbbell&level=beginner", nil, http.StatusOK).Data
	if len(shoulderExercises) != 1 {
		t.Fatalf("expected 1 beginner home shoulder exercise, got %d", len(shoulderExercises))
	}
	if shoulderExercises[0].Name != "Dumbbell Shoulder Press" {
		t.Fatalf("expected dumbbell shoulder press, got %q", shoulderExercises[0].Name)
	}

	backExercises := requestJSON[api.PaginatedResponse[models.Exercise]](t, server, http.MethodGet, "/v1/exercises?muscle=back&equipment=Resistance%20Band&level=Beginner", nil, http.StatusOK).Data
	if len(backExercises) != 1 {
		t.Fatalf("expected 1 beginner home back exercise, got %d", len(backExercises))
	}
	if backExercises[0].Name != "Band Pull-Apart" {
		t.Fatalf("expected band pull-apart, got %q", backExercises[0].Name)
	}

	bodyweightExercises := requestJSON[api.PaginatedResponse[models.Exercise]](t, server, http.MethodGet, "/v1/exercises?equipment=body%20only&level=beginner", nil, http.StatusOK).Data
	if len(bodyweightExercises) != 1 {
		t.Fatalf("expected 1 bodyweight beginner exercise, got %d", len(bodyweightExercises))
	}
	if bodyweightExercises[0].Name != "Air Squat" {
		t.Fatalf("expected air squat, got %q", bodyweightExercises[0].Name)
	}
}

func TestLoginMatchesLegacyEmailCaseAndBackfillsStoredEmail(t *testing.T) {
	t.Parallel()

	db, server := newTestApp(t)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "Legacy.User@Example.COM",
		PasswordHash: string(passwordHash),
		Name:         "Legacy User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create legacy user: %v", err)
	}

	auth := requestJSON[authEnvelope](t, server, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "legacy.user@example.com",
		"password": "password123",
	}, http.StatusOK)
	if auth.User.ID != user.ID {
		t.Fatalf("expected login to return legacy user %s, got %s", user.ID, auth.User.ID)
	}

	var stored models.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Email != "legacy.user@example.com" {
		t.Fatalf("expected email to be normalized on login, got %q", stored.Email)
	}
}

func TestLoginRejectsLegacyPlaceholderHashesWithMigrationError(t *testing.T) {
	t.Parallel()

	db, server := newTestApp(t)

	user := models.User{
		Email:        "placeholder@example.com",
		PasswordHash: "pending-auth",
		Name:         "Placeholder User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create placeholder user: %v", err)
	}

	errBody := requestError(t, server, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    user.Email,
		"password": "password123",
	}, http.StatusConflict)
	if errBody["error"] != "account requires password reset before login" {
		t.Fatalf("expected migration error, got %q", errBody["error"])
	}
}

func TestProtectedRoutesRejectWrongUser(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ownerAuth := registerTestUser(t, server, "owner@example.com", "Owner", "password123")
	otherAuth := registerTestUser(t, server, "viewer@example.com", "Viewer", "password123")

	workout := requestJSONAuth[models.Workout](t, server, ownerAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": ownerAuth.User.ID,
		"type":    "push",
	}, http.StatusCreated)

	expectStatusAuth(t, server, otherAuth.AccessToken, http.MethodGet, "/v1/users/"+ownerAuth.User.ID.String(), nil, http.StatusForbidden)
	expectStatusAuth(t, server, otherAuth.AccessToken, http.MethodGet, "/v1/workouts/"+workout.ID.String(), nil, http.StatusForbidden)
}

type authEnvelope struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         models.User `json:"user"`
}

func registerTestUser(t *testing.T, handler http.Handler, email, name, password string) authEnvelope {
	t.Helper()

	return requestJSON[authEnvelope](t, handler, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    email,
		"name":     name,
		"password": password,
	}, http.StatusCreated)
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()

	_, handler := newTestApp(t)
	return handler
}

func newTestApp(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return db, api.NewServer(db).Handler()
}

func requestJSON[T any](t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) T {
	t.Helper()

	return requestJSONAuth[T](t, handler, "", method, path, body, wantStatus)
}

func expectStatus(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) {
	t.Helper()

	expectStatusAuth(t, handler, "", method, path, body, wantStatus)
}

func requestError(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) map[string]any {
	t.Helper()

	return requestErrorAuth(t, handler, "", method, path, body, wantStatus)
}

func requestJSONAuth[T any](t *testing.T, handler http.Handler, token, method, path string, body any, wantStatus int) T {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func expectStatusAuth(t *testing.T, handler http.Handler, token, method, path string, body any, wantStatus int) {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}
}

func requestErrorAuth(t *testing.T, handler http.Handler, token, method, path string, body any, wantStatus int) map[string]any {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("%s %s: decode error response: %v", method, path, err)
	}

	return result
}

func errorFieldMap(t *testing.T, body map[string]any) map[string]string {
	t.Helper()

	raw, ok := body["errors"]
	if !ok {
		return nil
	}

	fields, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected errors object, got %T", raw)
	}

	out := make(map[string]string, len(fields))
	for key, value := range fields {
		text, ok := value.(string)
		if !ok {
			t.Fatalf("expected errors[%q] to be a string, got %T", key, value)
		}
		out[key] = text
	}

	return out
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

func requestJSONAuthRaw(t *testing.T, handler http.Handler, token, method, path string, body any, wantStatus int) []byte {
	t.Helper()

	req := httptest.NewRequest(method, path, encodeBody(t, body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d, body=%s", method, path, wantStatus, recorder.Code, recorder.Body.String())
	}

	return recorder.Body.Bytes()
}

func TestListWorkoutsPagination(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "pag@example.com", "Pag", "password123")

	// create 25 workouts
	for i := 0; i < 25; i++ {
		requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id":  userAuth.User.ID.String(),
			"date":     "2026-04-10",
			"duration": 60,
			"type":     "strength",
		}, http.StatusCreated)
	}

	// fetch page 1, limit 10
	resp := requestJSONAuth[api.PaginatedResponse[models.Workout]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts?page=1&limit=10", nil, http.StatusOK)
	if len(resp.Data) != 10 {
		t.Fatalf("expected 10 items, got %d", len(resp.Data))
	}
	if resp.Metadata.TotalCount != 25 {
		t.Fatalf("expected total count 25, got %d", resp.Metadata.TotalCount)
	}
	if resp.Metadata.TotalPages != 3 {
		t.Fatalf("expected total pages 3, got %d", resp.Metadata.TotalPages)
	}
	if !resp.Metadata.HasNext {
		t.Fatalf("expected has_next to be true")
	}

	// fetch page 3, limit 10
	resp3 := requestJSONAuth[api.PaginatedResponse[models.Workout]](t, server, userAuth.AccessToken, http.MethodGet, "/v1/workouts?page=3&limit=10", nil, http.StatusOK)
	if len(resp3.Data) != 5 {
		t.Fatalf("expected 5 items, got %d", len(resp3.Data))
	}
	if resp3.Metadata.HasNext {
		t.Fatalf("expected has_next to be false")
	}
}
