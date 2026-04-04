package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createCardioEntryRequest struct {
	WorkoutID       string   `json:"workout_id"`
	Modality        string   `json:"modality"`
	DurationMinutes int      `json:"duration_minutes"`
	Distance        *float64 `json:"distance,omitempty"`
	DistanceUnit    *string  `json:"distance_unit,omitempty"`
	Pace            *float64 `json:"pace,omitempty"`
	CaloriesBurned  *int     `json:"calories_burned,omitempty"`
	AvgHeartRate    *int     `json:"avg_heart_rate,omitempty"`
	Notes           string   `json:"notes"`
}

type updateCardioEntryRequest struct {
	Modality        *string  `json:"modality,omitempty"`
	DurationMinutes *int     `json:"duration_minutes,omitempty"`
	Distance        *float64 `json:"distance,omitempty"`
	DistanceUnit    *string  `json:"distance_unit,omitempty"`
	Pace            *float64 `json:"pace,omitempty"`
	CaloriesBurned  *int     `json:"calories_burned,omitempty"`
	AvgHeartRate    *int     `json:"avg_heart_rate,omitempty"`
	Notes           *string  `json:"notes,omitempty"`
}

func (s *Server) handleListWorkoutCardio(w http.ResponseWriter, r *http.Request) {
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

	var entries []models.WorkoutCardioEntry
	if err := s.db.Where("workout_id = ?", workoutID).Order("created_at asc").Find(&entries).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(entries))
}

func (s *Server) handleCreateCardioEntry(w http.ResponseWriter, r *http.Request) {
	var req createCardioEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	workoutID, err := resolveScopedUUID(r, "id", "workout_id", req.WorkoutID)
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

	if req.DurationMinutes <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("duration_minutes must be greater than zero"))
		return
	}

	modality := strings.TrimSpace(req.Modality)
	if modality == "" {
		writeError(w, http.StatusBadRequest, errors.New("modality is required"))
		return
	}

	if !models.ValidCardioModalities[modality] {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid modality: %s", modality))
		return
	}

	if req.Distance != nil && *req.Distance < 0 {
		writeError(w, http.StatusBadRequest, errors.New("distance cannot be negative"))
		return
	}

	if req.CaloriesBurned != nil && *req.CaloriesBurned < 0 {
		writeError(w, http.StatusBadRequest, errors.New("calories_burned cannot be negative"))
		return
	}

	if req.AvgHeartRate != nil && *req.AvgHeartRate < 0 {
		writeError(w, http.StatusBadRequest, errors.New("avg_heart_rate cannot be negative"))
		return
	}

	entry := models.WorkoutCardioEntry{
		WorkoutID:       workoutID,
		Modality:        modality,
		DurationMinutes: req.DurationMinutes,
		Distance:        req.Distance,
		DistanceUnit:    req.DistanceUnit,
		Pace:            req.Pace,
		CaloriesBurned:  req.CaloriesBurned,
		AvgHeartRate:    req.AvgHeartRate,
		Notes:           strings.TrimSpace(req.Notes),
	}

	if err := s.db.Create(&entry).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleGetCardioEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.cardioEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var entry models.WorkoutCardioEntry
	if err := s.db.First(&entry, "id = ?", entryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleUpdateCardioEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.cardioEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateCardioEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var entry models.WorkoutCardioEntry
	if err := s.db.First(&entry, "id = ?", entryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.Modality != nil {
		modality := strings.TrimSpace(*req.Modality)
		if modality == "" {
			writeError(w, http.StatusBadRequest, errors.New("modality cannot be blank"))
			return
		}
		if !models.ValidCardioModalities[modality] {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid modality: %s", modality))
			return
		}
		entry.Modality = modality
	}

	if req.DurationMinutes != nil {
		if *req.DurationMinutes <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("duration_minutes must be greater than zero"))
			return
		}
		entry.DurationMinutes = *req.DurationMinutes
	}

	if req.Distance != nil {
		if *req.Distance < 0 {
			writeError(w, http.StatusBadRequest, errors.New("distance cannot be negative"))
			return
		}
		entry.Distance = req.Distance
	}

	if req.DistanceUnit != nil {
		entry.DistanceUnit = req.DistanceUnit
	}

	if req.Pace != nil {
		entry.Pace = req.Pace
	}

	if req.CaloriesBurned != nil {
		if *req.CaloriesBurned < 0 {
			writeError(w, http.StatusBadRequest, errors.New("calories_burned cannot be negative"))
			return
		}
		entry.CaloriesBurned = req.CaloriesBurned
	}

	if req.AvgHeartRate != nil {
		if *req.AvgHeartRate < 0 {
			writeError(w, http.StatusBadRequest, errors.New("avg_heart_rate cannot be negative"))
			return
		}
		entry.AvgHeartRate = req.AvgHeartRate
	}

	if req.Notes != nil {
		entry.Notes = strings.TrimSpace(*req.Notes)
	}

	if err := s.db.Save(&entry).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleDeleteCardioEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.cardioEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	result := s.db.Delete(&models.WorkoutCardioEntry{}, "id = ?", entryID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("cardio entry not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) cardioEntryOwnerID(entryID uuid.UUID) (uuid.UUID, error) {
	var entry models.WorkoutCardioEntry
	if err := s.db.Select("workout_id").First(&entry, "id = ?", entryID).Error; err != nil {
		return uuid.Nil, err
	}
	return s.workoutOwnerID(entry.WorkoutID)
}
