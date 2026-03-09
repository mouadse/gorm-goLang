package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

type createExerciseRequest struct {
	Name         string `json:"name"`
	MuscleGroup  string `json:"muscle_group"`
	Equipment    string `json:"equipment"`
	Difficulty   string `json:"difficulty"`
	Instructions string `json:"instructions"`
	VideoURL     string `json:"video_url"`
}

type updateExerciseRequest struct {
	Name         *string `json:"name"`
	MuscleGroup  *string `json:"muscle_group"`
	Equipment    *string `json:"equipment"`
	Difficulty   *string `json:"difficulty"`
	Instructions *string `json:"instructions"`
	VideoURL     *string `json:"video_url"`
}

func (s *Server) handleCreateExercise(w http.ResponseWriter, r *http.Request) {
	var req createExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	exercise := models.Exercise{
		Name:         name,
		MuscleGroup:  strings.TrimSpace(req.MuscleGroup),
		Equipment:    strings.TrimSpace(req.Equipment),
		Difficulty:   strings.TrimSpace(req.Difficulty),
		Instructions: strings.TrimSpace(req.Instructions),
		VideoURL:     strings.TrimSpace(req.VideoURL),
	}

	if err := s.db.Create(&exercise).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, exercise)
}

func (s *Server) handleListExercises(w http.ResponseWriter, r *http.Request) {
	query := s.db.Model(&models.Exercise{})

	if name := strings.TrimSpace(r.URL.Query().Get("name")); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if muscleGroup := strings.TrimSpace(r.URL.Query().Get("muscle_group")); muscleGroup != "" {
		query = query.Where("muscle_group = ?", muscleGroup)
	}

	if equipment := strings.TrimSpace(r.URL.Query().Get("equipment")); equipment != "" {
		query = query.Where("equipment = ?", equipment)
	}

	if difficulty := strings.TrimSpace(r.URL.Query().Get("difficulty")); difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}

	var exercises []models.Exercise
	if err := query.Order("name asc").Find(&exercises).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(exercises))
}

func (s *Server) handleGetExercise(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var exercise models.Exercise
	if err := s.db.First(&exercise, "id = ?", exerciseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, exercise)
}

func (s *Server) handleUpdateExercise(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateExerciseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var exercise models.Exercise
	if err := s.db.First(&exercise, "id = ?", exerciseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.Name != nil {
		name, err := requireNonBlank("name", *req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		exercise.Name = name
	}

	if req.MuscleGroup != nil {
		exercise.MuscleGroup = strings.TrimSpace(*req.MuscleGroup)
	}

	if req.Equipment != nil {
		exercise.Equipment = strings.TrimSpace(*req.Equipment)
	}

	if req.Difficulty != nil {
		exercise.Difficulty = strings.TrimSpace(*req.Difficulty)
	}

	if req.Instructions != nil {
		exercise.Instructions = strings.TrimSpace(*req.Instructions)
	}

	if req.VideoURL != nil {
		exercise.VideoURL = strings.TrimSpace(*req.VideoURL)
	}

	if err := s.db.Save(&exercise).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, exercise)
}

func (s *Server) handleDeleteExercise(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var exercise models.Exercise
	if err := s.db.Select("id").First(&exercise, "id = ?", exerciseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("exercise not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var workoutExerciseCount int64
	if err := s.db.Model(&models.WorkoutExercise{}).
		Where("exercise_id = ?", exerciseID).
		Count(&workoutExerciseCount).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if workoutExerciseCount > 0 {
		writeError(w, http.StatusConflict, errors.New("exercise is used by existing workouts"))
		return
	}

	if err := s.db.Delete(&models.Exercise{}, "id = ?", exerciseID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
