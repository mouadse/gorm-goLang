package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "weekly" // default
	}

	pillar := r.URL.Query().Get("pillar")
	if pillar == "" {
		pillar = "all" // default
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100 // cap
	}

	// Fetch leaderboard
	res, err := s.leaderboardSvc.GetLeaderboard(period, pillar, offset, limit, time.Now())
	if err != nil {
		http.Error(w, `{"error":"failed to fetch leaderboard"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
