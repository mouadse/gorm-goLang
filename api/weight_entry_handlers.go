package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"fitness-tracker/models"

	"gorm.io/gorm"
)

type createWeightEntryRequest struct {
	UserID string  `json:"user_id"`
	Weight float64 `json:"weight"`
	Date   string  `json:"date"`
	Notes  string  `json:"notes"`
}

type updateWeightEntryRequest struct {
	UserID *string  `json:"user_id"`
	Weight *float64 `json:"weight"`
	Date   *string  `json:"date"`
	Notes  *string  `json:"notes"`
}

func (s *Server) handleCreateWeightEntry(w http.ResponseWriter, r *http.Request) {
	var req createWeightEntryRequest
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

	if req.Weight <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("weight must be greater than zero"))
		return
	}

	entryDate, err := parseDateOrDefault(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	entry := models.WeightEntry{
		UserID: userID,
		Weight: req.Weight,
		Date:   entryDate,
		Notes:  strings.TrimSpace(req.Notes),
	}

	if err := s.db.Create(&entry).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.metrics.WeightEntriesLogged.Inc()

	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleListWeightEntries(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	query := s.db.Model(&models.WeightEntry{}).Where("user_id = ?", userID)

	if dateParam := strings.TrimSpace(r.URL.Query().Get("date")); dateParam != "" {
		parsedDate, err := parseDate(dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		query = query.Where("date = ?", parsedDate)
	}

	if startDate := strings.TrimSpace(r.URL.Query().Get("start_date")); startDate != "" {
		parsedStart, err := parseDate(startDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("start_date must be %s", dateLayout))
			return
		}
		query = query.Where("date >= ?", parsedStart)
	}

	if endDate := strings.TrimSpace(r.URL.Query().Get("end_date")); endDate != "" {
		parsedEnd, err := parseDate(endDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("end_date must be %s", dateLayout))
			return
		}
		query = query.Where("date <= ?", parsedEnd)
	}

	page, limit := parsePagination(r)
	var entries []models.WeightEntry
	paginated, err := paginate(query.Order("date desc, created_at desc"), page, limit, &entries)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleGetWeightEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.weightEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var entry models.WeightEntry
	if err := s.db.First(&entry, "id = ?", entryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleUpdateWeightEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.weightEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateWeightEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var entry models.WeightEntry
	if err := s.db.First(&entry, "id = ?", entryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
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

		entry.UserID = userID
	}

	if req.Weight != nil {
		if *req.Weight <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("weight must be greater than zero"))
			return
		}
		entry.Weight = *req.Weight
	}

	if req.Date != nil {
		entryDate, err := parseDate(*req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		entry.Date = entryDate
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

func (s *Server) handleDeleteWeightEntry(w http.ResponseWriter, r *http.Request) {
	entryID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.weightEntryOwnerID(entryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	result := s.db.Delete(&models.WeightEntry{}, "id = ?", entryID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("weight entry not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
