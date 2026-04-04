package api_test

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"fitness-tracker/models"
)

func TestWorkoutAnalyticsHandlers(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	userAuth := registerTestUser(t, server, "analytics@example.com", "Analytics User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":         "Squat",
		"muscle_group": "Legs",
		"equipment":    "Barbell",
	}, http.StatusCreated)

	// Log a workout
	today := time.Now().Format("2006-01-02")
	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id":  user.ID,
		"date":     today,
		"duration": 45,
		"type":     "legs",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        3,
				"reps":        10,
				"weight":      100,
				"set_entries": []map[string]any{
					{"reps": 10, "weight": 100, "completed": true},
					{"reps": 10, "weight": 110, "completed": true}, // Personal best weight
					{"reps": 12, "weight": 90, "completed": true},
				},
			},
		},
	}, http.StatusCreated)

	// 1. Test Records
	t.Run("GetRecords", func(t *testing.T) {
		records := requestJSONAuth[[]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/records", nil, http.StatusOK)
		if len(records) == 0 {
			t.Errorf("expected personal records, got none")
		}
	})

	// 2. Test Stats
	t.Run("GetStats", func(t *testing.T) {
		stats := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/workout-stats", nil, http.StatusOK)
		if stats["total_workouts"].(float64) != 1 {
			t.Errorf("expected 1 total workout, got %v", stats["total_workouts"])
		}
	})

	// 3. Test History
	t.Run("GetHistory", func(t *testing.T) {
		history := requestJSONAuth[[]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history", nil, http.StatusOK)
		if len(history) != 1 {
			t.Errorf("expected 1 history entry, got %d", len(history))
		}
	})

	// 4. Test Activity Calendar
	t.Run("GetActivityCalendar", func(t *testing.T) {
		calendar := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar", nil, http.StatusOK)
		if len(calendar) != 1 {
			t.Errorf("expected 1 day in calendar, got %d", len(calendar))
		}
	})

	// 5. Test Streaks
	t.Run("GetStreaks", func(t *testing.T) {
		streaks := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/streaks?date="+today, nil, http.StatusOK)
		s := streaks["streaks"].(map[string]any)
		if s["workout_streak"].(float64) != 1 {
			t.Errorf("expected workout streak 1, got %v", s["workout_streak"])
		}
	})

	// 6. Test Authorization
	t.Run("Authorization", func(t *testing.T) {
		otherUserAuth := registerTestUser(t, server, "other@example.com", "Other User", "password123")
		// Other user tries to access analytics user's records
		expectStatusAuth(t, server, otherUserAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/records", nil, http.StatusForbidden)
	})
}

func TestExerciseHistoryLimitValidation(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "limit-test@example.com", "Limit User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Deadlift",
	}, http.StatusCreated)

	// Create some workout history
	today := time.Now().Format("2006-01-02")
	for i := 0; i < 3; i++ {
		requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    today,
			"duration": 60,
			"type":    "legs",
			"exercises": []map[string]any{
				{
					"exercise_id": exercise.ID,
					"order":       1,
					"sets":        3,
					"reps":        5,
					"weight":      100 + float64(i*10),
					"set_entries": []map[string]any{
						{"reps": 5, "weight": float64(100 + i*10), "completed": true},
					},
				},
			},
		}, http.StatusCreated)
	}

	// Valid limit should work
	t.Run("valid limit", func(t *testing.T) {
		history := requestJSONAuth[[]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history?limit=2", nil, http.StatusOK)
		if len(history) > 2 {
			t.Errorf("expected at most 2 history entries, got %d", len(history))
		}
	})

	// No limit should work (returns all)
	t.Run("no limit returns all", func(t *testing.T) {
		history := requestJSONAuth[[]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history", nil, http.StatusOK)
		if len(history) != 3 {
			t.Errorf("expected 3 history entries, got %d", len(history))
		}
	})

	// Malformed limit (non-numeric) should return 400
	t.Run("non-numeric limit returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history?limit=abc", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed limit")
		}
	})

	// Negative limit should return 400
	t.Run("negative limit returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history?limit=-5", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for negative limit")
		}
	})
}

func TestActivityCalendarDateFilters(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "calendar@example.com", "Calendar User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Bench Press",
	}, http.StatusCreated)

	// Create workout on a specific date
	workoutDate := "2026-01-15"
	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"date":    workoutDate,
		"duration": 45,
		"type":    "push",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        3,
				"reps":        10,
				"weight":      80,
				"set_entries": []map[string]any{
					{"reps": 10, "weight": 80, "completed": true},
				},
			},
		},
	}, http.StatusCreated)

	// Test: Only start date provided - should return from start until today
	t.Run("only start date", func(t *testing.T) {
		calendar := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar?start=2026-01-01", nil, http.StatusOK)
		// Should contain the workout from Jan 15
		if len(calendar) == 0 {
			t.Errorf("expected calendar to contain workout from Jan 15")
		}
	})

	// Test: Only end date provided - should return from earliest to end
	t.Run("only end date", func(t *testing.T) {
		calendar := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar?end=2026-01-31", nil, http.StatusOK)
		// Should contain the workout from Jan 15
		if len(calendar) == 0 {
			t.Errorf("expected calendar to contain workout from Jan 15")
		}
	})

	// Test: Both start and end provided - should use exact range
	t.Run("both start and end", func(t *testing.T) {
		calendar := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar?start=2026-01-10&end=2026-01-20", nil, http.StatusOK)
		// Should contain the workout from Jan 15
		if len(calendar) == 0 {
			t.Errorf("expected calendar to contain workout from Jan 15")
		}
	})

	// Test: Malformed start date should return 400
	t.Run("malformed start date returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar?start=invalid", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed start date")
		}
	})

	// Test: Malformed end date should return 400
	t.Run("malformed end date returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar?end=not-a-date", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed end date")
		}
	})
}

func TestStreaksDateValidation(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "streak@example.com", "Streak User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Squat",
	}, http.StatusCreated)

	// Create workout today
	today := time.Now().Format("2006-01-02")
	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"date":    today,
		"duration": 30,
		"type":    "legs",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        3,
				"reps":        10,
				"weight":      100,
				"set_entries": []map[string]any{
					{"reps": 10, "weight": 100, "completed": true},
				},
			},
		},
	}, http.StatusCreated)

	// Test: Valid date should work
	t.Run("valid date", func(t *testing.T) {
		streaks := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/streaks?date=2026-01-15", nil, http.StatusOK)
		if streaks == nil {
			t.Errorf("expected streaks response")
		}
	})

	// Test: Malformed date should return 400
	t.Run("malformed date returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/streaks?date=invalid", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed date")
		}
	})

	// Test: Malformed date format should return 400
	t.Run("wrong date format returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/streaks?date=01-15-2026", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for wrong date format")
		}
	})
}

func TestActivityCalendarDefaultReturns30Days(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "calendar30@example.com", "Calendar 30 User", "password123")
	user := userAuth.User

	// Create workouts over many days to test the window
	baseDate := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 35; i++ {
		date := baseDate.AddDate(0, 0, -i)
		requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
			"user_id": user.ID,
			"date":    date.Format("2006-01-02"),
			"duration": 30,
			"type":    "legs",
		}, http.StatusCreated)
	}

	// Request calendar with no parameters - should return last 30 days
	calendar := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/activity-calendar", nil, http.StatusOK)

	// Should have exactly30 days of activity (not 31)
	if len(calendar) != 30 {
		t.Errorf("expected exactly 30 days in default calendar window, got %d", len(calendar))
	}
}

func TestStreaksUsesUTCDate(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "utcstreak@example.com", "UTC Streak User", "password123")
	user := userAuth.User

	// Create a workout for "today" in UTC
	todayUTC := time.Now().UTC().Format("2006-01-02")
	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"date":    todayUTC,
		"duration": 30,
		"type":    "push",
	}, http.StatusCreated)

	// Request streaks without date parameter - should use UTC "today"
	streaks := requestJSONAuth[map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/streaks", nil, http.StatusOK)

	// Workout streak should be 1 (or more if there's history)
	streakData := streaks["streaks"].(map[string]any)
	if streakData["workout_streak"].(float64) < 1 {
		t.Errorf("expected workout streak >= 1, got %v", streakData["workout_streak"])
	}
}

func TestMalformedUserIDReturns400(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "malformed@example.com", "Malformed User", "password123")

	// Test: Malformed user_id in records endpoint should return 400, not 403
	t.Run("records with malformed user_id returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/not-a-uuid/records", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed user_id")
		}
	})

	// Test: Malformed user_id in workout-stats endpoint should return 400
	t.Run("workout-stats with malformed user_id returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/not-a-uuid/workout-stats", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed user_id")
		}
	})

	// Test: Malformed user_id in activity-calendar endpoint should return 400
	t.Run("activity-calendar with malformed user_id returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/not-a-uuid/activity-calendar", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed user_id")
		}
	})

	// Test: Malformed user_id in streaks endpoint should return 400
	t.Run("streaks with malformed user_id returns 400", func(t *testing.T) {
		errBody := requestErrorAuth(t, server, userAuth.AccessToken, http.MethodGet, "/v1/users/not-a-uuid/streaks", nil, http.StatusBadRequest)
		if errBody["error"] == "" {
			t.Errorf("expected error message for malformed user_id")
		}
	})
}

func TestExerciseHistorySnakeCaseJSON(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "snakecase@example.com", "SnakeCase User", "password123")
	user := userAuth.User

	exercise := requestJSONAuth[models.Exercise](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name": "Overhead Press",
	}, http.StatusCreated)

	// Create a workout with exercise history
	today := time.Now().Format("2006-01-02")
	requestJSONAuth[models.Workout](t, server, userAuth.AccessToken, http.MethodPost, "/v1/workouts", map[string]any{
		"user_id": user.ID,
		"date":    today,
		"duration": 45,
		"type":    "push",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        3,
				"reps":        10,
				"weight":      50,
				"set_entries": []map[string]any{
					{"reps": 10, "weight": 50, "completed": true},
				},
			},
		},
	}, http.StatusCreated)

	// Get history and check JSON field names
	resp := requestJSONAuthRaw(t, server, userAuth.AccessToken, http.MethodGet, "/v1/exercises/"+exercise.ID.String()+"/history", nil, http.StatusOK)

	// Check that the response uses snake_case for set fields
	if !bytes.Contains(resp, []byte("\"weight\":")) {
		t.Errorf("expected snake_case 'weight' field in sets, got: %s", string(resp))
	}
	if !bytes.Contains(resp, []byte("\"reps\":")) {
		t.Errorf("expected snake_case 'reps' field in sets, got: %s", string(resp))
	}
	if !bytes.Contains(resp, []byte("\"completed\":")) {
		t.Errorf("expected snake_case 'completed' field in sets, got: %s", string(resp))
	}
	// Check that it does NOT contain PascalCase fields
	if bytes.Contains(resp, []byte("\"Weight\":")) {
		t.Errorf("found unexpected PascalCase 'Weight' field in sets, should be snake_case")
	}
	if bytes.Contains(resp, []byte("\"Reps\":")) {
		t.Errorf("found unexpected PascalCase 'Reps' field in sets, should be snake_case")
	}
}