package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createProgramRequest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	IsActive    *bool                      `json:"is_active"`
	Weeks       []createProgramWeekRequest `json:"weeks"`
}

type updateProgramRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type createProgramWeekRequest struct {
	WeekNumber int                           `json:"week_number"`
	Name       string                        `json:"name"`
	Sessions   []createProgramSessionRequest `json:"sessions"`
}

type updateProgramWeekRequest struct {
	WeekNumber *int    `json:"week_number"`
	Name       *string `json:"name"`
}

type createProgramSessionRequest struct {
	DayNumber         int    `json:"day_number"`
	WorkoutTemplateID string `json:"workout_template_id"`
	Notes             string `json:"notes"`
}

type updateProgramSessionRequest struct {
	DayNumber         *int                 `json:"day_number"`
	WorkoutTemplateID optionalNullableText `json:"workout_template_id"`
	Notes             *string              `json:"notes"`
}

type createProgramAssignmentRequest struct {
	UserID      string `json:"user_id"`
	AssignedAt  string `json:"assigned_at"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	Status      string `json:"status"`
}

type updateProgramAssignmentRequest struct {
	Status      *string `json:"status"`
	StartedAt   *string `json:"started_at"`
	CompletedAt *string `json:"completed_at"`
}

type updateOwnProgramAssignmentStatusRequest struct {
	Status string `json:"status"`
}

type applyProgramSessionRequest struct {
	UserID string `json:"user_id"`
	Date   string `json:"date"`
}

var activeProgramAssignmentStatuses = []string{"assigned", "in_progress"}

type optionalNullableText struct {
	Set   bool
	Value *string
}

func (o *optionalNullableText) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func (s *Server) handleCreateProgram(w http.ResponseWriter, r *http.Request) {
	adminID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req createProgramRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	program := models.WorkoutProgram{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   adminID,
		IsActive:    true,
	}
	if req.IsActive != nil {
		program.IsActive = *req.IsActive
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&program).Error; err != nil {
			return err
		}

		for i, weekReq := range req.Weeks {
			week, err := s.buildProgramWeek(program.ID, weekReq, i+1)
			if err != nil {
				return err
			}
			if err := tx.Create(&week).Error; err != nil {
				return err
			}

			for j, sessionReq := range weekReq.Sessions {
				session, err := s.buildProgramSession(tx, week.ID, sessionReq, j+1)
				if err != nil {
					return err
				}
				if err := tx.Create(&session).Error; err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
			err = errors.New("workout template not found")
		}
		writeError(w, status, err)
		return
	}

	loaded, err := s.loadProgram(program.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, loaded)
}

func (s *Server) handleListPrograms(w http.ResponseWriter, r *http.Request) {
	query := s.db.Model(&models.WorkoutProgram{})
	if active := strings.TrimSpace(r.URL.Query().Get("active")); active != "" {
		switch strings.ToLower(active) {
		case "true", "1":
			query = query.Where("is_active = ?", true)
		case "false", "0":
			query = query.Where("is_active = ?", false)
		default:
			writeError(w, http.StatusBadRequest, errors.New("active must be true or false"))
			return
		}
	}

	page, limit := parsePagination(r)
	var programs []models.WorkoutProgram
	paginated, err := paginate(query.Order("created_at desc"), page, limit, &programs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleGetProgram(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	program, err := s.loadProgram(programID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, program)
}

func (s *Server) handleUpdateProgram(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var program models.WorkoutProgram
	if err := s.db.First(&program, "id = ?", programID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program not found"))
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
		program.Name = name
	}
	if req.Description != nil {
		program.Description = strings.TrimSpace(*req.Description)
	}
	if req.IsActive != nil {
		program.IsActive = *req.IsActive
	}

	if err := s.db.Save(&program).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loaded, err := s.loadProgram(programID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, loaded)
}

func (s *Server) handleDeleteProgram(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var program models.WorkoutProgram
		if err := tx.Select("id").First(&program, "id = ?", programID).Error; err != nil {
			return err
		}

		var weekIDs []uuid.UUID
		if err := tx.Model(&models.ProgramWeek{}).Where("program_id = ?", programID).Pluck("id", &weekIDs).Error; err != nil {
			return err
		}
		if len(weekIDs) > 0 {
			if err := tx.Where("week_id IN ?", weekIDs).Delete(&models.ProgramSession{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", weekIDs).Delete(&models.ProgramWeek{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("program_id = ?", programID).Delete(&models.ProgramAssignment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.WorkoutProgram{}, "id = ?", programID).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateProgramWeek(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if exists, err := recordExists(s.db, &models.WorkoutProgram{}, programID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if !exists {
		writeError(w, http.StatusNotFound, errors.New("program not found"))
		return
	}

	var req createProgramWeekRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	week, err := s.buildProgramWeek(programID, req, 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&week).Error; err != nil {
			return err
		}
		for i, sessionReq := range req.Sessions {
			session, err := s.buildProgramSession(tx, week.ID, sessionReq, i+1)
			if err != nil {
				return err
			}
			if err := tx.Create(&session).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
			err = errors.New("workout template not found")
		}
		writeError(w, status, err)
		return
	}

	loaded, err := s.loadProgramWeek(week.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, loaded)
}

func (s *Server) handleGetProgramWeek(w http.ResponseWriter, r *http.Request) {
	weekID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	week, err := s.loadProgramWeek(weekID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program week not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, week)
}

func (s *Server) handleUpdateProgramWeek(w http.ResponseWriter, r *http.Request) {
	weekID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramWeekRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var week models.ProgramWeek
	if err := s.db.First(&week, "id = ?", weekID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program week not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if req.WeekNumber != nil {
		if *req.WeekNumber <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("week_number must be greater than zero"))
			return
		}
		week.WeekNumber = *req.WeekNumber
	}
	if req.Name != nil {
		week.Name = strings.TrimSpace(*req.Name)
	}
	if err := s.db.Save(&week).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loaded, err := s.loadProgramWeek(weekID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, loaded)
}

func (s *Server) handleDeleteProgramWeek(w http.ResponseWriter, r *http.Request) {
	weekID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var week models.ProgramWeek
		if err := tx.Select("id").First(&week, "id = ?", weekID).Error; err != nil {
			return err
		}
		if err := tx.Where("week_id = ?", weekID).Delete(&models.ProgramSession{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.ProgramWeek{}, "id = ?", weekID).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program week not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateProgramSession(w http.ResponseWriter, r *http.Request) {
	weekID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if exists, err := recordExists(s.db, &models.ProgramWeek{}, weekID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if !exists {
		writeError(w, http.StatusNotFound, errors.New("program week not found"))
		return
	}

	var req createProgramSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	session, err := s.buildProgramSession(s.db, weekID, req, 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("workout template not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.db.Create(&session).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loaded, err := s.loadProgramSession(session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, loaded)
}

func (s *Server) handleGetProgramSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	session, err := s.loadProgramSession(sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program session not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleUpdateProgramSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var session models.ProgramSession
	if err := s.db.First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program session not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if req.DayNumber != nil {
		if *req.DayNumber <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("day_number must be greater than zero"))
			return
		}
		session.DayNumber = *req.DayNumber
	}
	if req.WorkoutTemplateID.Set {
		rawTemplateID := ""
		if req.WorkoutTemplateID.Value != nil {
			rawTemplateID = *req.WorkoutTemplateID.Value
		}
		templateID, err := parseOptionalUUID("workout_template_id", rawTemplateID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if templateID != nil {
			if exists, err := recordExists(s.db, &models.WorkoutTemplate{}, *templateID); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			} else if !exists {
				writeError(w, http.StatusNotFound, errors.New("workout template not found"))
				return
			}
		}
		session.WorkoutTemplateID = templateID
	}
	if req.Notes != nil {
		session.Notes = strings.TrimSpace(*req.Notes)
	}
	if err := s.db.Save(&session).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loaded, err := s.loadProgramSession(sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, loaded)
}

func (s *Server) handleDeleteProgramSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result := s.db.Delete(&models.ProgramSession{}, "id = ?", sessionID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("program session not found"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateProgramAssignment(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if exists, err := recordExists(s.db, &models.WorkoutProgram{}, programID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if !exists {
		writeError(w, http.StatusNotFound, errors.New("program not found"))
		return
	}

	var req createProgramAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := parseRequiredUUID("user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if exists, err := recordExists(s.db, &models.User{}, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	} else if !exists {
		writeError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "assigned"
	}
	if err := validateProgramAssignmentStatus(status); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var existing models.ProgramAssignment
	err = s.db.Where("user_id = ? AND program_id = ? AND status IN ?", userID, programID, activeProgramAssignmentStatuses).First(&existing).Error
	if err == nil {
		writeError(w, http.StatusConflict, errors.New("active program assignment already exists"))
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	assignedAt := time.Now().UTC()
	if strings.TrimSpace(req.AssignedAt) != "" {
		assignedAt, err = parseDateTimeOrDate(req.AssignedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	startedAt, err := parseOptionalDateTimeOrDate("started_at", req.StartedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	completedAt, err := parseOptionalDateTimeOrDate("completed_at", req.CompletedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	assignment := models.ProgramAssignment{
		UserID:      userID,
		ProgramID:   programID,
		AssignedAt:  assignedAt,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Status:      status,
	}
	normalizeProgramAssignmentState(&assignment, time.Now().UTC())

	if err := s.db.Create(&assignment).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	loaded, err := s.loadProgramAssignment(assignment.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, loaded)
}

func (s *Server) handleListProgramAssignmentsForProgram(w http.ResponseWriter, r *http.Request) {
	programID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	page, limit := parsePagination(r)
	var assignments []models.ProgramAssignment
	paginated, err := paginate(s.programAssignmentBaseQuery().Where("program_id = ?", programID).Order("assigned_at desc"), page, limit, &assignments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleListProgramAssignments(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	query := s.programAssignmentBaseQuery().Where("user_id = ?", userID)
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		if err := validateProgramAssignmentStatus(status); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		query = query.Where("status = ?", status)
	}

	page, limit := parsePagination(r)
	var assignments []models.ProgramAssignment
	paginated, err := paginate(query.Order("assigned_at desc"), page, limit, &assignments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleGetProgramAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	assignment, err := s.loadProgramAssignment(assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program assignment not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, assignment.UserID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (s *Server) handleAdminUpdateProgramAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateProgramAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	assignment, err := s.updateProgramAssignment(assignmentID, req)
	if err != nil {
		writeProgramAssignmentUpdateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignment)
}

func (s *Server) handleUpdateOwnProgramAssignmentStatus(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var assignment models.ProgramAssignment
	if err := s.db.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program assignment not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, assignment.UserID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateOwnProgramAssignmentStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	updated, err := s.updateProgramAssignment(assignmentID, updateProgramAssignmentRequest{Status: &req.Status})
	if err != nil {
		writeProgramAssignmentUpdateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProgramAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result := s.db.Delete(&models.ProgramAssignment{}, "id = ?", assignmentID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("program assignment not found"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleApplyProgramSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req applyProgramSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	if strings.TrimSpace(req.UserID) != "" {
		requestedUserID, err := parseRequiredUUID("user_id", req.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := authorizeUser(r, requestedUserID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		userID = requestedUserID
	}

	workoutDate, err := parseDateOrDefault(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var session models.ProgramSession
	if err := s.db.Preload("Week").First(&session, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("program session not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var assignment models.ProgramAssignment
	if err := s.db.
		Where("user_id = ? AND program_id = ? AND status <> ?", userID, session.Week.ProgramID, "cancelled").
		Order("assigned_at desc").
		First(&assignment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusForbidden, errors.New("program session is not assigned to user"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var workout *models.Workout
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if session.WorkoutTemplateID != nil {
			templateSvc := services.NewWorkoutTemplateService(tx)
			workout, err = templateSvc.ApplyTemplate(*session.WorkoutTemplateID, userID, workoutDate)
			if err != nil {
				return err
			}
			if strings.TrimSpace(session.Notes) != "" {
				notes := strings.TrimSpace(workout.Notes)
				if notes == "" {
					notes = strings.TrimSpace(session.Notes)
				} else {
					notes = notes + "\n\nProgram notes: " + strings.TrimSpace(session.Notes)
				}
				if err := tx.Model(&models.Workout{}).Where("id = ?", workout.ID).Update("notes", notes).Error; err != nil {
					return err
				}
				workout.Notes = notes
			}
		} else {
			created := models.Workout{
				UserID: userID,
				Date:   workoutDate,
				Type:   "program",
				Notes:  strings.TrimSpace(session.Notes),
			}
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			workout = &created
		}

		if assignment.StartedAt == nil || assignment.Status == "assigned" {
			now := time.Now().UTC()
			assignment.StartedAt = &now
			assignment.Status = "in_progress"
			if err := tx.Save(&assignment).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		if err.Error() == "template not found" {
			writeError(w, http.StatusNotFound, errors.New("workout template not found"))
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, workout)
}

func (s *Server) buildProgramWeek(programID uuid.UUID, req createProgramWeekRequest, fallbackWeek int) (models.ProgramWeek, error) {
	weekNumber := pickPositiveOrDefault(req.WeekNumber, fallbackWeek)
	if weekNumber <= 0 {
		return models.ProgramWeek{}, errors.New("week_number must be greater than zero")
	}
	return models.ProgramWeek{
		ProgramID:  programID,
		WeekNumber: weekNumber,
		Name:       strings.TrimSpace(req.Name),
	}, nil
}

func (s *Server) buildProgramSession(db *gorm.DB, weekID uuid.UUID, req createProgramSessionRequest, fallbackDay int) (models.ProgramSession, error) {
	dayNumber := pickPositiveOrDefault(req.DayNumber, fallbackDay)
	if dayNumber <= 0 {
		return models.ProgramSession{}, errors.New("day_number must be greater than zero")
	}
	templateID, err := parseOptionalUUID("workout_template_id", req.WorkoutTemplateID)
	if err != nil {
		return models.ProgramSession{}, err
	}
	if templateID != nil {
		exists, err := recordExists(db, &models.WorkoutTemplate{}, *templateID)
		if err != nil {
			return models.ProgramSession{}, err
		}
		if !exists {
			return models.ProgramSession{}, gorm.ErrRecordNotFound
		}
	}
	return models.ProgramSession{
		WeekID:            weekID,
		DayNumber:         dayNumber,
		WorkoutTemplateID: templateID,
		Notes:             strings.TrimSpace(req.Notes),
	}, nil
}

func (s *Server) loadProgram(programID uuid.UUID) (*models.WorkoutProgram, error) {
	var program models.WorkoutProgram
	err := s.db.
		Preload("Weeks.Sessions.Template").
		Preload("Weeks.Sessions", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_number asc")
		}).
		Preload("Weeks", func(db *gorm.DB) *gorm.DB {
			return db.Order("week_number asc")
		}).
		First(&program, "id = ?", programID).Error
	if err != nil {
		return nil, err
	}
	return &program, nil
}

func (s *Server) loadProgramWeek(weekID uuid.UUID) (*models.ProgramWeek, error) {
	var week models.ProgramWeek
	err := s.db.
		Preload("Sessions.Template").
		Preload("Sessions", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_number asc")
		}).
		First(&week, "id = ?", weekID).Error
	if err != nil {
		return nil, err
	}
	return &week, nil
}

func (s *Server) loadProgramSession(sessionID uuid.UUID) (*models.ProgramSession, error) {
	var session models.ProgramSession
	err := s.db.Preload("Template").First(&session, "id = ?", sessionID).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *Server) programAssignmentBaseQuery() *gorm.DB {
	return s.db.
		Preload("Program.Weeks.Sessions.Template").
		Preload("Program.Weeks.Sessions", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_number asc")
		}).
		Preload("Program.Weeks", func(db *gorm.DB) *gorm.DB {
			return db.Order("week_number asc")
		})
}

func (s *Server) loadProgramAssignment(assignmentID uuid.UUID) (*models.ProgramAssignment, error) {
	var assignment models.ProgramAssignment
	err := s.programAssignmentBaseQuery().First(&assignment, "id = ?", assignmentID).Error
	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (s *Server) updateProgramAssignment(assignmentID uuid.UUID, req updateProgramAssignmentRequest) (*models.ProgramAssignment, error) {
	var assignment models.ProgramAssignment
	if err := s.db.First(&assignment, "id = ?", assignmentID).Error; err != nil {
		return nil, err
	}

	if req.StartedAt != nil {
		startedAt, err := parseOptionalDateTimeOrDate("started_at", *req.StartedAt)
		if err != nil {
			return nil, err
		}
		assignment.StartedAt = startedAt
	}
	if req.CompletedAt != nil {
		completedAt, err := parseOptionalDateTimeOrDate("completed_at", *req.CompletedAt)
		if err != nil {
			return nil, err
		}
		assignment.CompletedAt = completedAt
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if err := validateProgramAssignmentStatus(status); err != nil {
			return nil, err
		}
		assignment.Status = status
		normalizeProgramAssignmentState(&assignment, time.Now().UTC())
	}

	if err := s.db.Save(&assignment).Error; err != nil {
		return nil, err
	}
	return s.loadProgramAssignment(assignmentID)
}

func validateProgramAssignmentStatus(status string) error {
	switch strings.TrimSpace(status) {
	case "assigned", "in_progress", "completed", "cancelled":
		return nil
	default:
		return errors.New("status must be assigned, in_progress, completed, or cancelled")
	}
}

func normalizeProgramAssignmentState(assignment *models.ProgramAssignment, now time.Time) {
	switch assignment.Status {
	case "assigned":
		assignment.CompletedAt = nil
	case "in_progress":
		if assignment.StartedAt == nil {
			assignment.StartedAt = &now
		}
		assignment.CompletedAt = nil
	case "completed":
		if assignment.StartedAt == nil {
			assignment.StartedAt = &now
		}
		if assignment.CompletedAt == nil {
			assignment.CompletedAt = &now
		}
	case "cancelled":
		assignment.CompletedAt = nil
	}
}

func parseDateTimeOrDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("date-time value is required")
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := parseDate(raw)
	if err != nil {
		return time.Time{}, errors.New("date-time value must be RFC3339 or YYYY-MM-DD")
	}
	return parsed, nil
}

func parseOptionalDateTimeOrDate(field, raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := parseDateTimeOrDate(raw)
	if err != nil {
		return nil, errors.New(field + " must be RFC3339 or YYYY-MM-DD")
	}
	return &parsed, nil
}

func writeProgramAssignmentUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, errors.New("program assignment not found"))
	case strings.Contains(err.Error(), "status must be"):
		writeError(w, http.StatusBadRequest, err)
	case strings.Contains(err.Error(), "must be RFC3339"):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
