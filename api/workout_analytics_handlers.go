package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleGetUserRecords(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	records, err := s.analyticsSvc.GetUserPersonalRecords(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(records))
}

func (s *Server) handleGetUserWorkoutStats(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	stats, err := s.analyticsSvc.GetUserWorkoutStats(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleGetExerciseHistory(w http.ResponseWriter, r *http.Request) {
	exerciseID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid limit: must be a positive integer"))
			return
		}
		if limit <= 0 {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid limit: must be a positive integer"))
			return
		}
	}

	history, err := s.analyticsSvc.GetExerciseHistory(userID, exerciseID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(history))
}

func (s *Server) handleGetUserActivityCalendar(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time

	// Parse start date if provided
	if startStr != "" {
		var parseErr error
		start, parseErr = time.Parse("2006-01-02", startStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid start date format: must be YYYY-MM-DD"))
			return
		}
	}

	// Parse end date if provided
	if endStr != "" {
		var parseErr error
		end, parseErr = time.Parse("2006-01-02", endStr)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid end date format: must be YYYY-MM-DD"))
			return
		}
	}

	// Fill in missing dates intelligently
	if startStr == "" && endStr == "" {
		// Neither provided: default to last 30 days (inclusive)
		end = time.Now().UTC()
		start = end.AddDate(0, 0, -29) // 30 days inclusive
	} else if startStr == "" {
		// Only end provided: use 29 days before end for 30-day window
		start = end.AddDate(0, 0, -29)
	} else if endStr == "" {
		// Only start provided: use today (UTC) as end
		end = time.Now().UTC()
	}

	calendar, err := s.adherenceSvc.GetActivityCalendar(userID, start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, calendar)
}

func (s *Server) handleGetUserStreaks(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	today := time.Now().UTC()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid date format: must be YYYY-MM-DD"))
			return
		}
		today = d
	}

	streaks, err := s.adherenceSvc.GetUserStreaks(userID, today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, streaks)
}
