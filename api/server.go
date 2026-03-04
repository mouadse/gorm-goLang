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

	// Workout + set-by-set logging
	s.mux.HandleFunc("POST /v1/workouts", s.handleCreateWorkout)
	s.mux.HandleFunc("GET /v1/workouts/{id}", s.handleGetWorkout)
	s.mux.HandleFunc("POST /v1/workouts/{id}/exercises", s.handleAddWorkoutExercise)
	s.mux.HandleFunc("GET /v1/workout-exercises/{id}/sets", s.handleListWorkoutSets)
	s.mux.HandleFunc("POST /v1/workout-exercises/{id}/sets", s.handleCreateWorkoutSet)
	s.mux.HandleFunc("PATCH /v1/workout-sets/{id}", s.handleUpdateWorkoutSet)
	s.mux.HandleFunc("DELETE /v1/workout-sets/{id}", s.handleDeleteWorkoutSet)

	// Program enrollment + progress
	s.mux.HandleFunc("POST /v1/program-enrollments", s.handleCreateProgramEnrollment)
	s.mux.HandleFunc("GET /v1/users/{user_id}/program-enrollments", s.handleListProgramEnrollments)
	s.mux.HandleFunc("GET /v1/program-enrollments/{id}", s.handleGetProgramEnrollment)
	s.mux.HandleFunc("PATCH /v1/program-enrollments/{id}", s.handleUpdateProgramEnrollment)
	s.mux.HandleFunc("POST /v1/program-enrollments/{id}/progress", s.handleCreateProgramProgress)
	s.mux.HandleFunc("GET /v1/program-enrollments/{id}/progress", s.handleListProgramProgress)
	s.mux.HandleFunc("PATCH /v1/program-progress/{id}", s.handleUpdateProgramProgress)

	// Friendship directionality
	s.mux.HandleFunc("POST /v1/friendships/requests", s.handleCreateFriendRequest)
	s.mux.HandleFunc("GET /v1/users/{user_id}/friendships/incoming", s.handleListIncomingFriendRequests)
	s.mux.HandleFunc("GET /v1/users/{user_id}/friendships/outgoing", s.handleListOutgoingFriendRequests)
	s.mux.HandleFunc("GET /v1/users/{user_id}/friends", s.handleListFriends)
	s.mux.HandleFunc("PATCH /v1/friendships/{id}/accept", s.handleAcceptFriendRequest)
	s.mux.HandleFunc("DELETE /v1/friendships/{id}", s.handleDeleteFriendship)

	// Meals (now supports multiple meals of same type/day)
	s.mux.HandleFunc("POST /v1/meals", s.handleCreateMeal)
	s.mux.HandleFunc("GET /v1/users/{user_id}/meals", s.handleListMeals)
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
