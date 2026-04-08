package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"
	"gorm.io/gorm"
)

type createExerciseRequest struct {
	ExerciseLibID    string `json:"exercise_lib_id"`
	Name             string `json:"name"`
	Force            string `json:"force"`
	Level            string `json:"level"`
	Mechanic         string `json:"mechanic"`
	Equipment        string `json:"equipment"`
	Category         string `json:"category"`
	PrimaryMuscles   string `json:"primary_muscles"`
	SecondaryMuscles string `json:"secondary_muscles"`
	Instructions     string `json:"instructions"`
	ImageURL         string `json:"image_url"`
	AltImageURL      string `json:"alt_image_url"`
}

type updateExerciseRequest struct {
	Name             *string `json:"name"`
	Force            *string `json:"force"`
	Level            *string `json:"level"`
	Mechanic         *string `json:"mechanic"`
	Equipment        *string `json:"equipment"`
	Category         *string `json:"category"`
	PrimaryMuscles   *string `json:"primary_muscles"`
	SecondaryMuscles *string `json:"secondary_muscles"`
	Instructions     *string `json:"instructions"`
	ImageURL         *string `json:"image_url"`
	AltImageURL      *string `json:"alt_image_url"`
}

func normalizeExerciseFilterValue(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func exerciseEquipmentFilterAliases(raw string) []string {
	normalized := normalizeExerciseFilterValue(raw)
	switch normalized {
	case "", "any":
		return nil
	case "body only", "bodyweight", "body weight", "body":
		return []string{"body only", "bodyweight", "body weight"}
	default:
		return []string{normalized}
	}
}

func writeExerciseLibProxyError(w http.ResponseWriter, context string, err error) {
	var upstreamErr *services.ExerciseLibAPIError
	if errors.As(err, &upstreamErr) {
		writeError(w, upstreamErr.StatusCode, fmt.Errorf("%s: %w", context, upstreamErr))
		return
	}

	writeError(w, http.StatusBadGateway, fmt.Errorf("%s: %w", context, err))
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
		ExerciseLibID:    strings.TrimSpace(req.ExerciseLibID),
		Name:             name,
		Force:            strings.TrimSpace(req.Force),
		Level:            strings.TrimSpace(req.Level),
		Mechanic:         strings.TrimSpace(req.Mechanic),
		Equipment:        strings.TrimSpace(req.Equipment),
		Category:         strings.TrimSpace(req.Category),
		PrimaryMuscles:   strings.TrimSpace(req.PrimaryMuscles),
		SecondaryMuscles: strings.TrimSpace(req.SecondaryMuscles),
		Instructions:     strings.TrimSpace(req.Instructions),
		ImageURL:         strings.TrimSpace(req.ImageURL),
		AltImageURL:      strings.TrimSpace(req.AltImageURL),
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
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(name)+"%")
	}

	if level := strings.TrimSpace(r.URL.Query().Get("level")); level != "" {
		query = query.Where("LOWER(level) = ?", normalizeExerciseFilterValue(level))
	}

	if equipment := strings.TrimSpace(r.URL.Query().Get("equipment")); equipment != "" {
		query = query.Where("LOWER(equipment) IN ?", exerciseEquipmentFilterAliases(equipment))
	}

	if category := strings.TrimSpace(r.URL.Query().Get("category")); category != "" {
		query = query.Where("category = ?", category)
	}

	if muscle := strings.TrimSpace(r.URL.Query().Get("muscle")); muscle != "" {
		m := "%" + strings.ToLower(muscle) + "%"
		query = query.Where("LOWER(primary_muscles) LIKE ? OR LOWER(secondary_muscles) LIKE ?", m, m)
	}

	var exercises []models.Exercise
	if err := query.Order("name asc").Find(&exercises).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if exercises == nil {
		exercises = []models.Exercise{}
	}

	writeJSON(w, http.StatusOK, exercises)
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

	if req.Force != nil {
		exercise.Force = strings.TrimSpace(*req.Force)
	}

	if req.Level != nil {
		exercise.Level = strings.TrimSpace(*req.Level)
	}

	if req.Mechanic != nil {
		exercise.Mechanic = strings.TrimSpace(*req.Mechanic)
	}

	if req.Equipment != nil {
		exercise.Equipment = strings.TrimSpace(*req.Equipment)
	}

	if req.Category != nil {
		exercise.Category = strings.TrimSpace(*req.Category)
	}

	if req.PrimaryMuscles != nil {
		exercise.PrimaryMuscles = strings.TrimSpace(*req.PrimaryMuscles)
	}

	if req.SecondaryMuscles != nil {
		exercise.SecondaryMuscles = strings.TrimSpace(*req.SecondaryMuscles)
	}

	if req.Instructions != nil {
		exercise.Instructions = strings.TrimSpace(*req.Instructions)
	}

	if req.ImageURL != nil {
		exercise.ImageURL = strings.TrimSpace(*req.ImageURL)
	}

	if req.AltImageURL != nil {
		exercise.AltImageURL = strings.TrimSpace(*req.AltImageURL)
	}

	if strings.TrimSpace(exercise.ExerciseLibID) == "" {
		exercise.ExerciseLibID = "local-" + exercise.ID.String()
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

type searchExercisesRequest struct {
	Query     string  `json:"query"`
	TopK      int     `json:"top_k"`
	Level     *string `json:"level,omitempty"`
	Equipment *string `json:"equipment,omitempty"`
	Category  *string `json:"category,omitempty"`
	Muscle    *string `json:"muscle,omitempty"`
}

func (s *Server) handleSearchExercises(w http.ResponseWriter, r *http.Request) {
	var req searchExercisesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 8
	}

	libResp, err := s.exerciseLibSvc.Search(services.LibSearchRequest{
		Query:     req.Query,
		TopK:      topK,
		Level:     req.Level,
		Equipment: req.Equipment,
		Category:  req.Category,
		Muscle:    req.Muscle,
	})
	if err != nil {
		writeExerciseLibProxyError(w, "exercise library search", err)
		return
	}

	writeJSON(w, http.StatusOK, libResp)
}

type generateProgramRequest struct {
	Goal             string   `json:"goal"`
	DaysPerWeek      int      `json:"days_per_week"`
	SessionMinutes   int      `json:"session_minutes"`
	Level            string   `json:"level"`
	EquipmentProfile string   `json:"equipment_profile"`
	Focus            []string `json:"focus"`
	Notes            string   `json:"notes"`
}

func (s *Server) handleGenerateProgram(w http.ResponseWriter, r *http.Request) {
	var req generateProgramRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	libResp, err := s.exerciseLibSvc.GetProgram(services.LibProgramRequest{
		Goal:             req.Goal,
		DaysPerWeek:      req.DaysPerWeek,
		SessionMinutes:   req.SessionMinutes,
		Level:            req.Level,
		EquipmentProfile: req.EquipmentProfile,
		Focus:            req.Focus,
		Notes:            req.Notes,
	})
	if err != nil {
		writeExerciseLibProxyError(w, "exercise library program", err)
		return
	}

	writeJSON(w, http.StatusOK, libResp)
}

func (s *Server) handleExerciseLibraryMeta(w http.ResponseWriter, r *http.Request) {
	meta, err := s.exerciseLibSvc.GetMeta()
	if err != nil {
		writeExerciseLibProxyError(w, "exercise library meta", err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleExerciseImageProxy(w http.ResponseWriter, r *http.Request) {
	imagePath := r.PathValue("path")
	if imagePath == "" {
		writeError(w, http.StatusBadRequest, errors.New("missing image path"))
		return
	}

	proxyURL := s.exerciseLibSvc.BaseURL() + "/exercise-images/" + imagePath
	resp, err := http.Get(proxyURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("exercise image proxy: %w", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, resp.StatusCode, errors.New("image not found"))
		return
	}

	for key, values := range resp.Header {
		if strings.EqualFold(key, "Content-Type") || strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Cache-Control") {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}
