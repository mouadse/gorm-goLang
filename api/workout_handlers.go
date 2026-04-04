package api

import (
	"errors"
	"net/http"
	"strings"

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

type updateWorkoutRequest struct {
	UserID   *string `json:"user_id"`
	Date     *string `json:"date"`
	Duration *int    `json:"duration"`
	Notes    *string `json:"notes"`
	Type     *string `json:"type"`
}

type createWorkoutExerciseRequest struct {
	WorkoutID  string                    `json:"workout_id"`
	ExerciseID string                    `json:"exercise_id"`
	Order      int                       `json:"order"`
	Sets       int                       `json:"sets"`
	Reps       int                       `json:"reps"`
	Weight     float64                   `json:"weight"`
	RestTime   int                       `json:"rest_time"`
	Notes      string                    `json:"notes"`
	SetEntries []createWorkoutSetRequest `json:"set_entries"`
}

type updateWorkoutExerciseRequest struct {
	ExerciseID *string  `json:"exercise_id"`
	Order      *int     `json:"order"`
	Sets       *int     `json:"sets"`
	Reps       *int     `json:"reps"`
	Weight     *float64 `json:"weight"`
	RestTime   *int     `json:"rest_time"`
	Notes      *string  `json:"notes"`
}

type createWorkoutSetRequest struct {
	WorkoutExerciseID string  `json:"workout_exercise_id"`
	SetNumber         int     `json:"set_number"`
	Reps              int     `json:"reps"`
	Weight            float64 `json:"weight"`
	RPE               float64 `json:"rpe"`
	RestSeconds       int     `json:"rest_seconds"`
	Completed         *bool   `json:"completed"`
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

	userID, err := resolveScopedUUID(r, "user_id", "user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := authorizeUser(r, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	exists, err := recordExists(s.db, &models.User{}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}

	if req.Duration < 0 {
		writeError(w, http.StatusBadRequest, errors.New("duration cannot be negative"))
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
		Notes:    strings.TrimSpace(req.Notes),
		Type:     strings.TrimSpace(req.Type),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&workout).Error; err != nil {
			return err
		}

		for i, exerciseReq := range req.Exercises {
			workoutExercise, err := s.buildWorkoutExercise(tx, workout.ID, exerciseReq, i+1)
			if err != nil {
				return err
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("exercise not found"))
			return
		}
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

func (s *Server) handleListWorkouts(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	query := s.db.Model(&models.Workout{}).Where("user_id = ?", userID)

	if dateParam := strings.TrimSpace(r.URL.Query().Get("date")); dateParam != "" {
		workoutDate, err := parseDate(dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		query = query.Where("date = ?", workoutDate)
	}

	if workoutType := strings.TrimSpace(r.URL.Query().Get("type")); workoutType != "" {
		query = query.Where("type = ?", workoutType)
	}

	var workouts []models.Workout
	if err := query.Order("date desc, created_at desc").Find(&workouts).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(workouts))
}

func (s *Server) handleGetWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutOwnerID(workoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
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

func (s *Server) handleUpdateWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutOwnerID(workoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateWorkoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var workout models.Workout
	if err := s.db.First(&workout, "id = ?", workoutID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.UserID != nil {
		userID, err := parseRequiredUUID("user_id", *req.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		exists, err := recordExists(s.db, &models.User{}, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		if err := authorizeUser(r, userID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}

		workout.UserID = userID
	}

	if req.Date != nil {
		workoutDate, err := parseDate(*req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		workout.Date = workoutDate
	}

	if req.Duration != nil {
		if *req.Duration < 0 {
			writeError(w, http.StatusBadRequest, errors.New("duration cannot be negative"))
			return
		}
		workout.Duration = *req.Duration
	}

	if req.Notes != nil {
		workout.Notes = strings.TrimSpace(*req.Notes)
	}

	if req.Type != nil {
		workout.Type = strings.TrimSpace(*req.Type)
	}

	if err := s.db.Save(&workout).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loadedWorkout, err := s.loadWorkout(workoutID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, loadedWorkout)
}

func (s *Server) handleDeleteWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutOwnerID(workoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var workout models.Workout
		if err := tx.Select("id").First(&workout, "id = ?", workoutID).Error; err != nil {
			return err
		}

		if err := deleteWorkoutDependencies(tx, []uuid.UUID{workoutID}); err != nil {
			return err
		}

		return tx.Delete(&models.Workout{}, "id = ?", workoutID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	s.handleAddWorkoutExercise(w, r)
}

func (s *Server) handleAddWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	var req createWorkoutExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutID, err := resolveWorkoutID(r, req.WorkoutID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ownerID, err := s.workoutOwnerID(workoutID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout or exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	workoutExercise, err := s.createWorkoutExercise(workoutID, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout or exercise not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, workoutExercise)
}

func (s *Server) handleListWorkoutExercises(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	query := s.db.Model(&models.WorkoutExercise{}).
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workouts.user_id = ?", currentUserID).
		Preload("Exercise").
		Preload("WorkoutSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		})

	if pathWorkoutID := strings.TrimSpace(r.PathValue("id")); pathWorkoutID != "" && strings.Contains(r.URL.Path, "/workouts/") {
		workoutID, err := parseRequiredUUID("workout_id", pathWorkoutID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ownerID, err := s.workoutOwnerID(workoutID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeError(w, http.StatusNotFound, errors.New("workout not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := authorizeUser(r, ownerID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		query = query.Where("workout_id = ?", workoutID)
	} else if workoutIDParam := strings.TrimSpace(r.URL.Query().Get("workout_id")); workoutIDParam != "" {
		workoutID, err := parseRequiredUUID("workout_id", workoutIDParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ownerID, err := s.workoutOwnerID(workoutID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeError(w, http.StatusNotFound, errors.New("workout not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := authorizeUser(r, ownerID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		query = query.Where("workout_id = ?", workoutID)
	}

	if exerciseIDParam := strings.TrimSpace(r.URL.Query().Get("exercise_id")); exerciseIDParam != "" {
		exerciseID, err := parseRequiredUUID("exercise_id", exerciseIDParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		query = query.Where("exercise_id = ?", exerciseID)
	}

	var workoutExercises []models.WorkoutExercise
	if err := query.Order("workout_exercises.\"order\" asc, workout_exercises.created_at asc").Find(&workoutExercises).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(workoutExercises))
}

func (s *Server) handleGetWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	workoutExerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	workoutExercise, err := s.loadWorkoutExercise(workoutExerciseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, workoutExercise)
}

func (s *Server) handleUpdateWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	workoutExerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateWorkoutExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var workoutExercise models.WorkoutExercise
	if err := s.db.First(&workoutExercise, "id = ?", workoutExerciseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.ExerciseID != nil {
		exerciseID, err := parseRequiredUUID("exercise_id", *req.ExerciseID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		exists, err := recordExists(s.db, &models.Exercise{}, exerciseID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, errors.New("exercise not found"))
			return
		}

		workoutExercise.ExerciseID = exerciseID
	}

	if req.Order != nil {
		if *req.Order <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("order must be greater than zero"))
			return
		}
		workoutExercise.Order = *req.Order
	}

	if req.Sets != nil {
		if *req.Sets < 0 {
			writeError(w, http.StatusBadRequest, errors.New("sets cannot be negative"))
			return
		}
		workoutExercise.Sets = *req.Sets
	}

	if req.Reps != nil {
		if *req.Reps < 0 {
			writeError(w, http.StatusBadRequest, errors.New("reps cannot be negative"))
			return
		}
		workoutExercise.Reps = *req.Reps
	}

	if req.Weight != nil {
		if *req.Weight < 0 {
			writeError(w, http.StatusBadRequest, errors.New("weight cannot be negative"))
			return
		}
		workoutExercise.Weight = *req.Weight
	}

	if req.RestTime != nil {
		if *req.RestTime < 0 {
			writeError(w, http.StatusBadRequest, errors.New("rest_time cannot be negative"))
			return
		}
		workoutExercise.RestTime = *req.RestTime
	}

	if req.Notes != nil {
		workoutExercise.Notes = strings.TrimSpace(*req.Notes)
	}

	if err := s.db.Save(&workoutExercise).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loadedWorkoutExercise, err := s.loadWorkoutExercise(workoutExerciseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, loadedWorkoutExercise)
}

func (s *Server) handleDeleteWorkoutExercise(w http.ResponseWriter, r *http.Request) {
	workoutExerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var workoutExercise models.WorkoutExercise
		if err := tx.Select("id").First(&workoutExercise, "id = ?", workoutExerciseID).Error; err != nil {
			return err
		}

		if err := tx.Where("workout_exercise_id = ?", workoutExerciseID).Delete(&models.WorkoutSet{}).Error; err != nil {
			return err
		}

		return tx.Delete(&models.WorkoutExercise{}, "id = ?", workoutExerciseID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWorkoutSets(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	query := s.db.Model(&models.WorkoutSet{}).
		Joins("JOIN workout_exercises ON workout_exercises.id = workout_sets.workout_exercise_id").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workouts.user_id = ?", currentUserID)

	if pathWorkoutExerciseID := strings.TrimSpace(r.PathValue("id")); pathWorkoutExerciseID != "" && strings.Contains(r.URL.Path, "/workout-exercises/") && strings.HasSuffix(r.URL.Path, "/sets") {
		workoutExerciseID, err := parseRequiredUUID("workout_exercise_id", pathWorkoutExerciseID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := authorizeUser(r, ownerID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		query = query.Where("workout_exercise_id = ?", workoutExerciseID)
	} else if workoutExerciseIDParam := strings.TrimSpace(r.URL.Query().Get("workout_exercise_id")); workoutExerciseIDParam != "" {
		workoutExerciseID, err := parseRequiredUUID("workout_exercise_id", workoutExerciseIDParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := authorizeUser(r, ownerID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		query = query.Where("workout_exercise_id = ?", workoutExerciseID)
	}

	var sets []models.WorkoutSet
	if err := query.Order("workout_sets.set_number asc").Find(&sets).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(sets))
}

func (s *Server) handleCreateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	var req createWorkoutSetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutExerciseID, err := resolveWorkoutExerciseID(r, req.WorkoutExerciseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ownerID, err := s.workoutExerciseOwnerID(workoutExerciseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	set, err := s.createWorkoutSet(workoutExerciseID, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout exercise not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, set)
}

func (s *Server) handleGetWorkoutSet(w http.ResponseWriter, r *http.Request) {
	setID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutSetOwnerID(setID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout set not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
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

	writeJSON(w, http.StatusOK, set)
}

func (s *Server) handleUpdateWorkoutSet(w http.ResponseWriter, r *http.Request) {
	setID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.workoutSetOwnerID(setID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout set not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
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
		if *req.SetNumber <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("set_number must be greater than zero"))
			return
		}
		set.SetNumber = *req.SetNumber
	}
	if req.Reps != nil {
		if *req.Reps < 0 {
			writeError(w, http.StatusBadRequest, errors.New("reps cannot be negative"))
			return
		}
		set.Reps = *req.Reps
	}
	if req.Weight != nil {
		if *req.Weight < 0 {
			writeError(w, http.StatusBadRequest, errors.New("weight cannot be negative"))
			return
		}
		set.Weight = *req.Weight
	}
	if req.RPE != nil {
		if *req.RPE < 0 {
			writeError(w, http.StatusBadRequest, errors.New("rpe cannot be negative"))
			return
		}
		set.RPE = *req.RPE
	}
	if req.RestSeconds != nil {
		if *req.RestSeconds < 0 {
			writeError(w, http.StatusBadRequest, errors.New("rest_seconds cannot be negative"))
			return
		}
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

	ownerID, err := s.workoutSetOwnerID(setID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout set not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
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

func (s *Server) loadWorkoutExercise(workoutExerciseID uuid.UUID) (*models.WorkoutExercise, error) {
	var workoutExercise models.WorkoutExercise
	err := s.db.
		Preload("Exercise").
		Preload("WorkoutSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		}).
		First(&workoutExercise, "id = ?", workoutExerciseID).Error
	if err != nil {
		return nil, err
	}
	return &workoutExercise, nil
}

func (s *Server) buildWorkoutExercise(tx *gorm.DB, workoutID uuid.UUID, req createWorkoutExerciseRequest, fallbackOrder int) (models.WorkoutExercise, error) {
	exerciseID, err := parseRequiredUUID("exercise_id", req.ExerciseID)
	if err != nil {
		return models.WorkoutExercise{}, err
	}

	exists, err := recordExists(tx, &models.Exercise{}, exerciseID)
	if err != nil {
		return models.WorkoutExercise{}, err
	}
	if !exists {
		return models.WorkoutExercise{}, gorm.ErrRecordNotFound
	}

	if req.Sets < 0 || req.Reps < 0 || req.Weight < 0 || req.RestTime < 0 {
		return models.WorkoutExercise{}, errors.New("exercise summary values cannot be negative")
	}

	sets := req.Sets
	if len(req.SetEntries) > sets {
		sets = len(req.SetEntries)
	}

	workoutExercise := models.WorkoutExercise{
		WorkoutID:  workoutID,
		ExerciseID: exerciseID,
		Order:      pickPositiveOrDefault(req.Order, fallbackOrder),
		Sets:       sets,
		Reps:       req.Reps,
		Weight:     req.Weight,
		RestTime:   req.RestTime,
		Notes:      strings.TrimSpace(req.Notes),
	}

	return workoutExercise, nil
}

func (s *Server) createWorkoutExercise(workoutID uuid.UUID, req createWorkoutExerciseRequest) (*models.WorkoutExercise, error) {
	var createdID uuid.UUID

	err := s.db.Transaction(func(tx *gorm.DB) error {
		workoutExists, err := recordExists(tx, &models.Workout{}, workoutID)
		if err != nil {
			return err
		}
		if !workoutExists {
			return gorm.ErrRecordNotFound
		}

		order := req.Order
		if order <= 0 {
			order, err = nextWorkoutExerciseOrder(tx, workoutID)
			if err != nil {
				return err
			}
		}

		workoutExercise, err := s.buildWorkoutExercise(tx, workoutID, req, order)
		if err != nil {
			return err
		}

		if err := tx.Create(&workoutExercise).Error; err != nil {
			return err
		}

		if err := createWorkoutSets(tx, workoutExercise.ID, req.SetEntries); err != nil {
			return err
		}

		createdID = workoutExercise.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.loadWorkoutExercise(createdID)
}

func (s *Server) createWorkoutSet(workoutExerciseID uuid.UUID, req createWorkoutSetRequest) (*models.WorkoutSet, error) {
	workoutExerciseExists, err := recordExists(s.db, &models.WorkoutExercise{}, workoutExerciseID)
	if err != nil {
		return nil, err
	}
	if !workoutExerciseExists {
		return nil, gorm.ErrRecordNotFound
	}

	if req.Reps < 0 || req.Weight < 0 || req.RPE < 0 || req.RestSeconds < 0 {
		return nil, errors.New("set values cannot be negative")
	}

	setNumber := req.SetNumber
	if setNumber <= 0 {
		setNumber, err = s.nextSetNumber(workoutExerciseID)
		if err != nil {
			return nil, err
		}
	}

	completed := true
	if req.Completed != nil {
		completed = *req.Completed
	}

	set := &models.WorkoutSet{
		WorkoutExerciseID: workoutExerciseID,
		SetNumber:         setNumber,
		Reps:              req.Reps,
		Weight:            req.Weight,
		RPE:               req.RPE,
		RestSeconds:       req.RestSeconds,
		Completed:         completed,
	}

	if err := s.db.Create(set).Error; err != nil {
		return nil, err
	}

	return set, nil
}

func createWorkoutSets(tx *gorm.DB, workoutExerciseID uuid.UUID, requests []createWorkoutSetRequest) error {
	for i, setReq := range requests {
		if setReq.Reps < 0 || setReq.Weight < 0 || setReq.RPE < 0 || setReq.RestSeconds < 0 {
			return errors.New("set values cannot be negative")
		}

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

func nextWorkoutExerciseOrder(tx *gorm.DB, workoutID uuid.UUID) (int, error) {
	var maxOrder int
	err := tx.Model(&models.WorkoutExercise{}).
		Select("COALESCE(MAX(\"order\"), 0)").
		Where("workout_id = ?", workoutID).
		Scan(&maxOrder).Error
	if err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
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

func deleteWorkoutDependencies(tx *gorm.DB, workoutIDs []uuid.UUID) error {
	if len(workoutIDs) == 0 {
		return nil
	}

	var workoutExerciseIDs []uuid.UUID
	if err := tx.Model(&models.WorkoutExercise{}).
		Where("workout_id IN ?", workoutIDs).
		Pluck("id", &workoutExerciseIDs).Error; err != nil {
		return err
	}

	if len(workoutExerciseIDs) > 0 {
		if err := tx.Where("workout_exercise_id IN ?", workoutExerciseIDs).Delete(&models.WorkoutSet{}).Error; err != nil {
			return err
		}
	}

	if err := tx.Where("workout_id IN ?", workoutIDs).Delete(&models.WorkoutCardioEntry{}).Error; err != nil {
		return err
	}

	return tx.Where("workout_id IN ?", workoutIDs).Delete(&models.WorkoutExercise{}).Error
}

func resolveWorkoutID(r *http.Request, raw string) (uuid.UUID, error) {
	if strings.Contains(r.URL.Path, "/workouts/") {
		return resolveScopedUUID(r, "id", "workout_id", raw)
	}

	return parseRequiredUUID("workout_id", raw)
}

func resolveWorkoutExerciseID(r *http.Request, raw string) (uuid.UUID, error) {
	if strings.Contains(r.URL.Path, "/workout-exercises/") {
		return resolveScopedUUID(r, "id", "workout_exercise_id", raw)
	}

	return parseRequiredUUID("workout_exercise_id", raw)
}

func pickPositiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
