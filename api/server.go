package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Server struct {
	db  *gorm.DB
	mux *http.ServeMux
}

func NewServer(db *gorm.DB) *Server {
	server := &Server{
		db:  db,
		mux: http.NewServeMux(),
	}
	server.registerRoutes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPISpec)
	s.mux.HandleFunc("GET /docs", s.handleSwaggerUI)
	s.mux.HandleFunc("GET /docs/", s.handleSwaggerUI)

	// Users
	s.mux.HandleFunc("POST /v1/users", s.handleCreateUser)
	s.mux.HandleFunc("GET /v1/users", s.handleListUsers)
	s.mux.HandleFunc("GET /v1/users/{id}", s.handleGetUser)
	s.mux.HandleFunc("PATCH /v1/users/{id}", s.handleUpdateUser)
	s.mux.HandleFunc("DELETE /v1/users/{id}", s.handleDeleteUser)

	// Exercises
	s.mux.HandleFunc("POST /v1/exercises", s.handleCreateExercise)
	s.mux.HandleFunc("GET /v1/exercises", s.handleListExercises)
	s.mux.HandleFunc("GET /v1/exercises/{id}", s.handleGetExercise)
	s.mux.HandleFunc("PATCH /v1/exercises/{id}", s.handleUpdateExercise)
	s.mux.HandleFunc("DELETE /v1/exercises/{id}", s.handleDeleteExercise)

	// Weight entries
	s.mux.HandleFunc("POST /v1/weight-entries", s.handleCreateWeightEntry)
	s.mux.HandleFunc("GET /v1/weight-entries", s.handleListWeightEntries)
	s.mux.HandleFunc("GET /v1/users/{user_id}/weight-entries", s.handleListWeightEntries)
	s.mux.HandleFunc("POST /v1/users/{user_id}/weight-entries", s.handleCreateWeightEntry)
	s.mux.HandleFunc("GET /v1/weight-entries/{id}", s.handleGetWeightEntry)
	s.mux.HandleFunc("PATCH /v1/weight-entries/{id}", s.handleUpdateWeightEntry)
	s.mux.HandleFunc("DELETE /v1/weight-entries/{id}", s.handleDeleteWeightEntry)

	// Workouts
	s.mux.HandleFunc("POST /v1/workouts", s.handleCreateWorkout)
	s.mux.HandleFunc("GET /v1/workouts", s.handleListWorkouts)
	s.mux.HandleFunc("GET /v1/users/{user_id}/workouts", s.handleListWorkouts)
	s.mux.HandleFunc("POST /v1/users/{user_id}/workouts", s.handleCreateWorkout)
	s.mux.HandleFunc("GET /v1/workouts/{id}", s.handleGetWorkout)
	s.mux.HandleFunc("PATCH /v1/workouts/{id}", s.handleUpdateWorkout)
	s.mux.HandleFunc("DELETE /v1/workouts/{id}", s.handleDeleteWorkout)

	// Workout exercises
	s.mux.HandleFunc("POST /v1/workout-exercises", s.handleCreateWorkoutExercise)
	s.mux.HandleFunc("GET /v1/workout-exercises", s.handleListWorkoutExercises)
	s.mux.HandleFunc("GET /v1/workouts/{id}/exercises", s.handleListWorkoutExercises)
	s.mux.HandleFunc("POST /v1/workouts/{id}/exercises", s.handleAddWorkoutExercise)
	s.mux.HandleFunc("GET /v1/workout-exercises/{id}", s.handleGetWorkoutExercise)
	s.mux.HandleFunc("PATCH /v1/workout-exercises/{id}", s.handleUpdateWorkoutExercise)
	s.mux.HandleFunc("DELETE /v1/workout-exercises/{id}", s.handleDeleteWorkoutExercise)

	// Workout sets
	s.mux.HandleFunc("POST /v1/workout-sets", s.handleCreateWorkoutSet)
	s.mux.HandleFunc("GET /v1/workout-sets", s.handleListWorkoutSets)
	s.mux.HandleFunc("GET /v1/workout-exercises/{id}/sets", s.handleListWorkoutSets)
	s.mux.HandleFunc("POST /v1/workout-exercises/{id}/sets", s.handleCreateWorkoutSet)
	s.mux.HandleFunc("GET /v1/workout-sets/{id}", s.handleGetWorkoutSet)
	s.mux.HandleFunc("PATCH /v1/workout-sets/{id}", s.handleUpdateWorkoutSet)
	s.mux.HandleFunc("DELETE /v1/workout-sets/{id}", s.handleDeleteWorkoutSet)

	// Meals
	s.mux.HandleFunc("POST /v1/meals", s.handleCreateMeal)
	s.mux.HandleFunc("GET /v1/meals", s.handleListMeals)
	s.mux.HandleFunc("GET /v1/users/{user_id}/meals", s.handleListMeals)
	s.mux.HandleFunc("POST /v1/users/{user_id}/meals", s.handleCreateMeal)
	s.mux.HandleFunc("GET /v1/meals/{id}", s.handleGetMeal)
	s.mux.HandleFunc("PATCH /v1/meals/{id}", s.handleUpdateMeal)
	s.mux.HandleFunc("DELETE /v1/meals/{id}", s.handleDeleteMeal)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	if decoder.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func parsePathUUID(r *http.Request, field string) (uuid.UUID, error) {
	value := strings.TrimSpace(r.PathValue(field))
	if value == "" {
		return uuid.Nil, fmt.Errorf("missing path parameter: %s", field)
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid for %s", field)
	}
	return parsed, nil
}

func parseRequiredUUID(field, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid for %s", field)
	}
	return parsed, nil
}
