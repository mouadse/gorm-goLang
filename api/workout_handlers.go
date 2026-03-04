package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createWorkoutRequest struct {
	UserID    string                         `json:"user_id"`
	Date      string                         `json:"date"`
	Duration  int                            `json:"duration"`
	Notes     string                         `json:"notes"`
	Type      string                         `json:"type"`
	Exercises []createWorkoutExerciseRequest `json:"exercises"`
}

type createWorkoutExerciseRequest struct {
	ExerciseID string                    `json:"exercise_id"`
	Order      int                       `json:"order"`
	Sets       int                       `json:"sets"`
	Reps       int                       `json:"reps"`
	Weight     float64                   `json:"weight"`
	RestTime   int                       `json:"rest_time"`
	Notes      string                    `json:"notes"`
	SetEntries []createWorkoutSetRequest `json:"set_entries"`
}

type createWorkoutSetRequest struct {
	SetNumber   int     `json:"set_number"`
	Reps        int     `json:"reps"`
	Weight      float64 `json:"weight"`
	RPE         float64 `json:"rpe"`
	RestSeconds int     `json:"rest_seconds"`
	Completed   *bool   `json:"completed"`
}

type updateWorkoutSetRequest struct {
	SetNumber   *int     `json:"set_number"`
	Reps        *int     `json:"reps"`
	Weight      *float64 `json:"weight"`
	RPE         *float64 `json:"rpe"`
	RestSeconds *int     `json:"rest_seconds"`
	Completed   *bool    `json:"completed"`
}

func (s *Server) handleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	var req createWorkoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := parseRequiredUUID("user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutDate, err := parseDateOrDefault(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workout := models.Workout{
		UserID:   userID,
		Date:     workoutDate,
		Duration: req.Duration,
		Notes:    req.Notes,
		Type:     req.Type,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&workout).Error; err != nil {
			return err
		}

		for i, exerciseReq := range req.Exercises {
			exerciseID, err := parseRequiredUUID("exercise_id", exerciseReq.ExerciseID)
			if err != nil {
				return err
			}

			workoutExercise := models.WorkoutExercise{
				WorkoutID:  workout.ID,
				ExerciseID: exerciseID,
				Order:      pickPositiveOrDefault(exerciseReq.Order, i+1),
				Sets:       exerciseReq.Sets,
				Reps:       exerciseReq.Reps,
				Weight:     exerciseReq.Weight,
				RestTime:   exerciseReq.RestTime,
				Notes:      exerciseReq.Notes,
			}
			if err := tx.Create(&workoutExercise).Error; err != nil {
				return err
			}

			if err := createWorkoutSets(tx, workoutExercise.ID, exerciseReq.SetEntries); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loadedWorkout, err := s.loadWorkout(workout.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, loadedWorkout)
}

func (s *Server) handleGetWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workout, err := s.loadWorkout(workoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, workout)
}

func (s *Server) handleAddWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	workoutID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req createWorkoutExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	exerciseID, err := parseRequiredUUID("exercise_id", req.ExerciseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutExercise := models.WorkoutExercise{
		WorkoutID:  workoutID,
		ExerciseID: exerciseID,
		Order:      req.Order,
		Sets:       req.Sets,
		Reps:       req.Reps,
		Weight:     req.Weight,
		RestTime:   req.RestTime,
		Notes:      req.Notes,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&workoutExercise).Error; err != nil {
			return err
		}
		return createWorkoutSets(tx, workoutExercise.ID, req.SetEntries)
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Preload("Exercise").Preload("WorkoutSets").First(&workoutExercise, "id = ?", workoutExercise.ID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, workoutExercise)
}

func (s *Server) handleListWorkoutSets(w http.ResponseWriter, r *http.Request) {
	workoutExerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var sets []models.WorkoutSet
	if err := s.db.Where("workout_exercise_id = ?", workoutExerciseID).Order("set_number asc").Find(&sets).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, sets)
}

func (s *Server) handleCreateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	workoutExerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req createWorkoutSetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	setNumber := req.SetNumber
	if setNumber <= 0 {
		setNumber, err = s.nextSetNumber(workoutExerciseID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	completed := true
	if req.Completed != nil {
		completed = *req.Completed
	}

	set := models.WorkoutSet{
		WorkoutExerciseID: workoutExerciseID,
		SetNumber:         setNumber,
		Reps:              req.Reps,
		Weight:            req.Weight,
		RPE:               req.RPE,
		RestSeconds:       req.RestSeconds,
		Completed:         completed,
	}

	if err := s.db.Create(&set).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, set)
}

func (s *Server) handleUpdateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	setID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateWorkoutSetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var set models.WorkoutSet
	if err := s.db.First(&set, "id = ?", setID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout set not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.SetNumber != nil {
		set.SetNumber = *req.SetNumber
	}
	if req.Reps != nil {
		set.Reps = *req.Reps
	}
	if req.Weight != nil {
		set.Weight = *req.Weight
	}
	if req.RPE != nil {
		set.RPE = *req.RPE
	}
	if req.RestSeconds != nil {
		set.RestSeconds = *req.RestSeconds
	}
	if req.Completed != nil {
		set.Completed = *req.Completed
	}

	if err := s.db.Save(&set).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, set)
}

func (s *Server) handleDeleteWorkoutSet(w http.ResponseWriter, r *http.Request) {
	setID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result := s.db.Delete(&models.WorkoutSet{}, "id = ?", setID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("workout set not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadWorkout(workoutID uuid.UUID) (*models.Workout, error) {
	var workout models.Workout
	err := s.db.
		Preload("WorkoutExercises.Exercise").
		Preload("WorkoutExercises.WorkoutSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		}).
		First(&workout, "id = ?", workoutID).Error
	if err != nil {
		return nil, err
	}
	return &workout, nil
}

func createWorkoutSets(tx *gorm.DB, workoutExerciseID uuid.UUID, requests []createWorkoutSetRequest) error {
	for i, setReq := range requests {
		setNumber := pickPositiveOrDefault(setReq.SetNumber, i+1)
		completed := true
		if setReq.Completed != nil {
			completed = *setReq.Completed
		}

		set := models.WorkoutSet{
			WorkoutExerciseID: workoutExerciseID,
			SetNumber:         setNumber,
			Reps:              setReq.Reps,
			Weight:            setReq.Weight,
			RPE:               setReq.RPE,
			RestSeconds:       setReq.RestSeconds,
			Completed:         completed,
		}

		if err := tx.Create(&set).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) nextSetNumber(workoutExerciseID uuid.UUID) (int, error) {
	var maxSetNumber int
	err := s.db.Model(&models.WorkoutSet{}).
		Select("COALESCE(MAX(set_number), 0)").
		Where("workout_exercise_id = ?", workoutExerciseID).
		Scan(&maxSetNumber).Error
	if err != nil {
		return 0, err
	}
	return maxSetNumber + 1, nil
}

func parseDateOrDefault(raw string) (time.Time, error) {
	if raw == "" {
		now := time.Now().UTC()
		return now, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be YYYY-MM-DD")
	}
	return t.UTC(), nil
}

func pickPositiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
