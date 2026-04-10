package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createTemplateRequest struct {
	OwnerID   string                          `json:"owner_id"`
	Name      string                          `json:"name"`
	Type      string                          `json:"type"`
	Notes     string                          `json:"notes"`
	Exercises []createTemplateExerciseRequest `json:"exercises"`
}

type createTemplateExerciseRequest struct {
	ExerciseID string                     `json:"exercise_id"`
	Order      int                        `json:"order"`
	Sets       int                        `json:"sets"`
	Reps       int                        `json:"reps"`
	Weight     float64                    `json:"weight"`
	RestTime   int                        `json:"rest_time"`
	Notes      string                     `json:"notes"`
	SetEntries []createTemplateSetRequest `json:"set_entries"`
}

type createTemplateSetRequest struct {
	SetNumber   int     `json:"set_number"`
	Reps        int     `json:"reps"`
	Weight      float64 `json:"weight"`
	RestSeconds int     `json:"rest_seconds"`
}

type updateTemplateRequest struct {
	Name  *string `json:"name"`
	Type  *string `json:"type"`
	Notes *string `json:"notes"`
}

type applyTemplateRequest struct {
	UserID string `json:"user_id"`
	Date   string `json:"date"`
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := resolveScopedUUID(r, "user_id", "owner_id", req.OwnerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	exists, err := recordExists(s.db, &models.User{}, ownerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return
	}

	template := models.WorkoutTemplate{
		OwnerID: ownerID,
		Name:    strings.TrimSpace(req.Name),
		Type:    strings.TrimSpace(req.Type),
		Notes:   strings.TrimSpace(req.Notes),
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&template).Error; err != nil {
			return err
		}

		for i, exerciseReq := range req.Exercises {
			templateExercise, err := s.buildTemplateExercise(tx, template.ID, exerciseReq, i+1)
			if err != nil {
				return err
			}
			if err := tx.Create(&templateExercise).Error; err != nil {
				return err
			}
			if err := createTemplateSets(tx, templateExercise.ID, exerciseReq.SetEntries); err != nil {
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

	loadedTemplate, err := s.loadTemplate(template.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, loadedTemplate)
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	page, limit := parsePagination(r)
	var templates []models.WorkoutTemplate
	paginated, err := paginate(s.db.Where("owner_id = ?", userID).Order("created_at desc"), page, limit, &templates)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.templateOwnerID(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	template, err := s.loadTemplate(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, template)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.templateOwnerID(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var template models.WorkoutTemplate
	if err := s.db.First(&template, "id = ?", templateID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			writeError(w, http.StatusBadRequest, errors.New("name cannot be empty"))
			return
		}
		template.Name = strings.TrimSpace(*req.Name)
	}

	if req.Type != nil {
		template.Type = strings.TrimSpace(*req.Type)
	}

	if req.Notes != nil {
		template.Notes = strings.TrimSpace(*req.Notes)
	}

	if err := s.db.Save(&template).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loadedTemplate, err := s.loadTemplate(templateID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, loadedTemplate)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.templateOwnerID(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	result := s.db.Delete(&models.WorkoutTemplate{}, "id = ?", templateID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("template not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleApplyTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.templateOwnerID(templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req applyTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := parseRequiredUUID("user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := authorizeUser(r, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	workoutDate, err := parseDateOrDefault(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	templateSvc := services.NewWorkoutTemplateService(s.db)
	workout, err := templateSvc.ApplyTemplate(templateID, userID, workoutDate)
	if err != nil {
		if err.Error() == "template not found" {
			writeError(w, http.StatusNotFound, errors.New("template not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, workout)
}

func (s *Server) buildTemplateExercise(tx *gorm.DB, templateID uuid.UUID, req createTemplateExerciseRequest, fallbackOrder int) (models.WorkoutTemplateExercise, error) {
	exerciseID, err := parseRequiredUUID("exercise_id", req.ExerciseID)
	if err != nil {
		return models.WorkoutTemplateExercise{}, err
	}

	exists, err := recordExists(tx, &models.Exercise{}, exerciseID)
	if err != nil {
		return models.WorkoutTemplateExercise{}, err
	}
	if !exists {
		return models.WorkoutTemplateExercise{}, gorm.ErrRecordNotFound
	}

	if req.Sets < 0 || req.Reps < 0 || req.Weight < 0 || req.RestTime < 0 {
		return models.WorkoutTemplateExercise{}, errors.New("exercise summary values cannot be negative")
	}

	sets := req.Sets
	if len(req.SetEntries) > sets {
		sets = len(req.SetEntries)
	}

	templateExercise := models.WorkoutTemplateExercise{
		TemplateID: templateID,
		ExerciseID: exerciseID,
		Order:      pickPositiveOrDefault(req.Order, fallbackOrder),
		Sets:       sets,
		Reps:       req.Reps,
		Weight:     req.Weight,
		RestTime:   req.RestTime,
		Notes:      strings.TrimSpace(req.Notes),
	}

	return templateExercise, nil
}

func createTemplateSets(tx *gorm.DB, templateExerciseID uuid.UUID, requests []createTemplateSetRequest) error {
	for i, setReq := range requests {
		if setReq.Reps < 0 || setReq.Weight < 0 || setReq.RestSeconds < 0 {
			return errors.New("set values cannot be negative")
		}

		setNumber := pickPositiveOrDefault(setReq.SetNumber, i+1)

		set := models.WorkoutTemplateSet{
			TemplateExerciseID: templateExerciseID,
			SetNumber:          setNumber,
			Reps:               setReq.Reps,
			Weight:             setReq.Weight,
			RestSeconds:        setReq.RestSeconds,
		}

		if err := tx.Create(&set).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) loadTemplate(templateID uuid.UUID) (*models.WorkoutTemplate, error) {
	var template models.WorkoutTemplate
	err := s.db.
		Preload("WorkoutTemplateExercises.Exercise").
		Preload("WorkoutTemplateExercises.WorkoutTemplateSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		}).
		Preload("WorkoutTemplateExercises", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc")
		}).
		First(&template, "id = ?", templateID).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (s *Server) templateOwnerID(templateID uuid.UUID) (uuid.UUID, error) {
	var template models.WorkoutTemplate
	if err := s.db.Select("owner_id").First(&template, "id = ?", templateID).Error; err != nil {
		return uuid.Nil, err
	}
	return template.OwnerID, nil
}
