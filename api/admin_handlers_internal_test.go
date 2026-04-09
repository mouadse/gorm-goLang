package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCollectRealtimeMetricsSQLiteUsesBaseTables(t *testing.T) {
	db, server := newAdminTestServer(t)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	user := models.User{
		Email:        "realtime-admin@example.com",
		PasswordHash: "hash",
		Name:         "Realtime Admin",
		Role:         "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	for _, record := range []interface{}{
		&models.Workout{UserID: user.ID, Date: today, Type: "push"},
		&models.Meal{UserID: user.ID, Date: today, MealType: "dinner"},
	} {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
	}

	metrics, err := server.collectRealtimeMetrics(context.Background())
	if err != nil {
		t.Fatalf("collect realtime metrics: %v", err)
	}

	if got, ok := metrics["active_users"].(int); !ok || got != 1 {
		t.Fatalf("expected active_users to equal 1, got %v (%T)", metrics["active_users"], metrics["active_users"])
	}
	if got, ok := metrics["workouts_today"].(int); !ok || got != 1 {
		t.Fatalf("expected workouts_today to equal 1, got %v (%T)", metrics["workouts_today"], metrics["workouts_today"])
	}
	if got, ok := metrics["meals_today"].(int); !ok || got != 1 {
		t.Fatalf("expected meals_today to equal 1, got %v (%T)", metrics["meals_today"], metrics["meals_today"])
	}
}

func TestHandleAdminPopularExercisesSQLiteUsesBaseTables(t *testing.T) {
	db, server := newAdminTestServer(t)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	user := models.User{
		Email:        "popular-admin@example.com",
		PasswordHash: "hash",
		Name:         "Popular Admin",
		Role:         "admin",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	exercise := models.Exercise{Name: "Bench Press", PrimaryMuscles: "Chest"}
	if err := db.Create(&exercise).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	workout := models.Workout{UserID: user.ID, Date: today, Type: "push"}
	if err := db.Create(&workout).Error; err != nil {
		t.Fatalf("create workout: %v", err)
	}

	workoutExercise := models.WorkoutExercise{
		WorkoutID:  workout.ID,
		ExerciseID: exercise.ID,
		Order:      1,
		Sets:       3,
		Reps:       8,
	}
	if err := db.Create(&workoutExercise).Error; err != nil {
		t.Fatalf("create workout exercise: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/workouts/exercises/popular", nil)
	recorder := httptest.NewRecorder()
	server.handleAdminPopularExercises(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var popular []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &popular); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(popular) == 0 {
		t.Fatal("expected at least one popular exercise")
	}
	if popular[0]["exercise_name"] != "Bench Press" {
		t.Fatalf("expected Bench Press, got %v", popular[0]["exercise_name"])
	}
	if popular[0]["usage_count"] != float64(1) {
		t.Fatalf("expected usage_count to equal 1, got %v", popular[0]["usage_count"])
	}
	if popular[0]["unique_users"] != float64(1) {
		t.Fatalf("expected unique_users to equal 1, got %v", popular[0]["unique_users"])
	}
}

func newAdminTestServer(t *testing.T) (*gorm.DB, *Server) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return db, NewServer(db)
}
