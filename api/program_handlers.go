package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createProgramEnrollmentRequest struct {
	UserID           string `json:"user_id"`
	WorkoutProgramID string `json:"workout_program_id"`
	Status           string `json:"status"`
	StartedOn        string `json:"started_on"`
	CurrentWeek      int    `json:"current_week"`
}

type updateProgramEnrollmentRequest struct {
	Status        *string `json:"status"`
	CurrentWeek   *int    `json:"current_week"`
	CompletedDays *int    `json:"completed_days"`
}

type createProgramProgressRequest struct {
	WeekNumber int     `json:"week_number"`
	DayNumber  int     `json:"day_number"`
	WorkoutID  *string `json:"workout_id"`
	Completed  bool    `json:"completed"`
	Notes      string  `json:"notes"`
}

type updateProgramProgressRequest struct {
	WeekNumber *int    `json:"week_number"`
	DayNumber  *int    `json:"day_number"`
	WorkoutID  *string `json:"workout_id"`
	Completed  *bool   `json:"completed"`
	Notes      *string `json:"notes"`
}

func (s *Server) handleCreateProgramEnrollment(w http.ResponseWriter, r *http.Request) {
	var req createProgramEnrollmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := parseRequiredUUID("user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	programID, err := parseRequiredUUID("workout_program_id", req.WorkoutProgramID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	startedOn, err := parseDateOrDefault(req.StartedOn)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	enrollment := models.ProgramEnrollment{
		UserID:           userID,
		WorkoutProgramID: programID,
		Status:           strings.TrimSpace(req.Status),
		StartedOn:        startedOn,
		CurrentWeek:      req.CurrentWeek,
	}

	if err := s.db.Create(&enrollment).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Preload("WorkoutProgram").Preload("ProgramProgress").First(&enrollment, "id = ?", enrollment.ID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, enrollment)
}

func (s *Server) handleListProgramEnrollments(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var enrollments []models.ProgramEnrollment
	if err := s.db.
		Where("user_id = ?", userID).
		Preload("WorkoutProgram").
		Preload("ProgramProgress", func(db *gorm.DB) *gorm.DB {
			return db.Order("week_number asc, day_number asc")
		}).
		Order("created_at desc").
		Find(&enrollments).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, enrollments)
}

func (s *Server) handleGetProgramEnrollment(w http.ResponseWriter, r *http.Request) {
	enrollmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var enrollment models.ProgramEnrollment
	err = s.db.
		Preload("WorkoutProgram").
		Preload("ProgramProgress", func(db *gorm.DB) *gorm.DB {
			return db.Order("week_number asc, day_number asc")
		}).
		First(&enrollment, "id = ?", enrollmentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program enrollment not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, enrollment)
}

func (s *Server) handleUpdateProgramEnrollment(w http.ResponseWriter, r *http.Request) {
	enrollmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramEnrollmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var enrollment models.ProgramEnrollment
	if err := s.db.First(&enrollment, "id = ?", enrollmentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program enrollment not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.Status != nil {
		enrollment.Status = strings.TrimSpace(*req.Status)
	}
	if req.CurrentWeek != nil {
		enrollment.CurrentWeek = *req.CurrentWeek
	}
	if req.CompletedDays != nil {
		enrollment.CompletedDays = *req.CompletedDays
	}

	if err := s.db.Save(&enrollment).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, enrollment)
}

func (s *Server) handleCreateProgramProgress(w http.ResponseWriter, r *http.Request) {
	enrollmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req createProgramProgressRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutID, err := parseOptionalUUID(req.WorkoutID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	progress := models.ProgramProgress{
		ProgramEnrollmentID: enrollmentID,
		WeekNumber:          req.WeekNumber,
		DayNumber:           req.DayNumber,
		WorkoutID:           workoutID,
		Completed:           req.Completed,
		Notes:               req.Notes,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&progress).Error; err != nil {
			return err
		}

		if progress.Completed {
			if err := tx.Model(&models.ProgramEnrollment{}).
				Where("id = ?", enrollmentID).
				Update("completed_days", gorm.Expr("completed_days + 1")).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, progress)
}

func (s *Server) handleListProgramProgress(w http.ResponseWriter, r *http.Request) {
	enrollmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var progress []models.ProgramProgress
	if err := s.db.Where("program_enrollment_id = ?", enrollmentID).
		Order("week_number asc, day_number asc").
		Find(&progress).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

func (s *Server) handleUpdateProgramProgress(w http.ResponseWriter, r *http.Request) {
	progressID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramProgressRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var progress models.ProgramProgress
	if err := s.db.First(&progress, "id = ?", progressID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program progress not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	wasCompleted := progress.Completed

	if req.WeekNumber != nil {
		progress.WeekNumber = *req.WeekNumber
	}
	if req.DayNumber != nil {
		progress.DayNumber = *req.DayNumber
	}
	if req.WorkoutID != nil {
		parsedWorkoutID, err := parseOptionalUUID(req.WorkoutID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		progress.WorkoutID = parsedWorkoutID
	}
	if req.Notes != nil {
		progress.Notes = *req.Notes
	}
	if req.Completed != nil {
		progress.Completed = *req.Completed
		if *req.Completed {
			now := time.Now().UTC()
			progress.CompletedAt = &now
		} else {
			progress.CompletedAt = nil
		}
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&progress).Error; err != nil {
			return err
		}

		if req.Completed != nil && wasCompleted != progress.Completed {
			var completedCount int64
			if err := tx.Model(&models.ProgramProgress{}).
				Where("program_enrollment_id = ?", progress.ProgramEnrollmentID).
				Where("completed = ?", true).
				Count(&completedCount).Error; err != nil {
				return err
			}

			if err := tx.Model(&models.ProgramEnrollment{}).
				Where("id = ?", progress.ProgramEnrollmentID).
				Update("completed_days", int(completedCount)).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

func parseOptionalUUID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, errors.New("invalid workout_id")
	}
	return &parsed, nil
}
