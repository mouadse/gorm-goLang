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
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(name)+"%")
	}

	if muscleGroup := strings.TrimSpace(r.URL.Query().Get("muscle_group")); muscleGroup != "" {
		query = applyExerciseListFilter(query, "muscle_group", expandExerciseFilter(muscleGroup, exerciseMuscleGroupAliases))
	}

	if equipment := strings.TrimSpace(r.URL.Query().Get("equipment")); equipment != "" {
		query = applyExerciseListFilter(query, "equipment", expandExerciseFilter(equipment, exerciseEquipmentAliases))
	}

	if difficulty := strings.TrimSpace(r.URL.Query().Get("difficulty")); difficulty != "" {
		query = applyExerciseListFilter(query, "difficulty", expandExerciseFilter(difficulty, exerciseDifficultyAliases))
	}

	var exercises []models.Exercise
	if err := query.Order("name asc").Find(&exercises).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(exercises))
}

var exerciseMuscleGroupAliases = map[string][]string{
	"chest":      {"chest"},
	"back":       {"back"},
	"leg":        {"legs"},
	"legs":       {"legs"},
	"hamstring":  {"hamstrings"},
	"hamstrings": {"hamstrings"},
	"shoulder":   {"shoulders"},
	"shoulders":  {"shoulders"},
}

var exerciseEquipmentAliases = map[string][]string{
	"band":             {"band", "resistance band"},
	"bands":            {"band", "resistance band"},
	"bodyweight":       {"bodyweight"},
	"dumbbell":         {"dumbbell"},
	"dumbbells":        {"dumbbell"},
	"home":             {"bodyweight", "dumbbell", "kettlebell", "band", "resistance band"},
	"home equipment":   {"bodyweight", "dumbbell", "kettlebell", "band", "resistance band"},
	"home equipement":  {"bodyweight", "dumbbell", "kettlebell", "band", "resistance band"},
	"home gym":         {"bodyweight", "dumbbell", "kettlebell", "band", "resistance band"},
	"kettlebell":       {"kettlebell"},
	"resistance band":  {"band", "resistance band"},
	"resistance bands": {"band", "resistance band"},
}

var exerciseDifficultyAliases = map[string][]string{
	"advanced":     {"advanced"},
	"beginner":     {"beginner"},
	"easy":         {"beginner"},
	"expert":       {"advanced"},
	"intermediate": {"intermediate"},
	"moderate":     {"intermediate"},
	"newbie":       {"beginner"},
	"novice":       {"beginner"},
	"starter":      {"beginner"},
}

func applyExerciseListFilter(query *gorm.DB, column string, values []string) *gorm.DB {
	if len(values) == 0 {
		return query
	}

	return query.Where("LOWER("+column+") IN ?", values)
}

func expandExerciseFilter(raw string, aliases map[string][]string) []string {
	normalized := normalizeExerciseFilter(raw)
	if normalized == "" {
		return nil
	}

	candidates := aliases[normalized]
	if len(candidates) == 0 {
		candidates = []string{normalized}
	}

	seen := make(map[string]struct{}, len(candidates))
	expanded := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizeExerciseFilter(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		expanded = append(expanded, candidate)
	}

	return expanded
}

func normalizeExerciseFilter(value string) string {
	replacer := strings.NewReplacer("-", " ", "_", " ")
	return strings.Join(strings.Fields(strings.ToLower(replacer.Replace(strings.TrimSpace(value)))), " ")
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
