package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, replace with proper origin check
	},
}

func (s *Server) handleAdminDashboardSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.adminSvc.GetExecutiveSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleAdminDashboardTrends(w http.ResponseWriter, r *http.Request) {
	var trends []map[string]any
	var err error

	if s.db.Dialector.Name() == "sqlite" {
		err = s.db.Raw(`
			WITH all_activities AS (
				SELECT user_id, date, 'workout' as type, id FROM workouts WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date, 'meal' as type, id FROM meals WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date, 'weight' as type, id FROM weight_entries WHERE deleted_at IS NULL
			)
			SELECT 
				date as stat_date,
				SUM(CASE WHEN type = 'workout' THEN 1 ELSE 0 END) as total_workouts,
				SUM(CASE WHEN type = 'meal' THEN 1 ELSE 0 END) as total_meals,
				SUM(CASE WHEN type = 'weight' THEN 1 ELSE 0 END) as total_weights
			FROM all_activities
			GROUP BY date
			ORDER BY stat_date DESC
			LIMIT 30
		`).Scan(&trends).Error
	} else {
		err = s.db.Table("daily_user_stats").
			Select("stat_date, total_workouts, total_meals, total_weights").
			Order("stat_date DESC").
			Limit(30).
			Scan(&trends).Error
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, trends)
}

func (s *Server) handleAdminUserStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.adminSvc.GetUserAnalytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAdminUserGrowth(w http.ResponseWriter, r *http.Request) {
	var growth []map[string]any

	query := `
		SELECT DATE_TRUNC('day', created_at) as date, COUNT(*) as new_users
		FROM users
		WHERE deleted_at IS NULL
		GROUP BY date
		ORDER BY date DESC
		LIMIT 30
	`
	if s.db.Dialector.Name() == "sqlite" {
		query = `
			SELECT DATE(created_at) as date, COUNT(*) as new_users
			FROM users
			WHERE deleted_at IS NULL
			GROUP BY date
			ORDER BY date DESC
			LIMIT 30
		`
	}
	s.db.Raw(query).Scan(&growth)

	writeJSON(w, http.StatusOK, growth)
}

func (s *Server) handleAdminWorkoutStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.adminSvc.GetWorkoutAnalytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAdminPopularExercises(w http.ResponseWriter, r *http.Request) {
	popular, err := s.adminSvc.GetPopularExercises(r.Context(), 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, popular)
}

func (s *Server) handleAdminNutritionStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.adminSvc.GetNutritionAnalytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAdminModerationStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.adminSvc.GetModerationAnalytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAdminSystemHealth(w http.ResponseWriter, r *http.Request) {
	health, err := s.adminSvc.GetSystemHealth(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (s *Server) handleAdminListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)
	var logs []models.AuditLog
	paginated, err := paginate(s.db.Order("created_at DESC"), page, limit, &logs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated)
}

func (s *Server) handleAdminRealtimeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		metrics, err := s.collectRealtimeMetrics(r.Context())
		if err != nil {
			log.Printf("collect realtime metrics failed: %v", err)
			return
		}
		data, err := json.Marshal(metrics)
		if err != nil {
			log.Printf("marshal realtime metrics failed: %v", err)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) collectRealtimeMetrics(ctx context.Context) (map[string]any, error) {
	realtime, err := s.adminSvc.GetRealtimeMetrics(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"active_users":   realtime.ActiveUsers,
		"workouts_today": realtime.WorkoutsToday,
		"meals_today":    realtime.MealsToday,
		"timestamp":      realtime.Timestamp,
	}, nil
}

func (s *Server) logAdminAction(r *http.Request, action, entityType string, entityID uuid.UUID, oldValue, newValue any) {
	adminID, _ := authenticatedUserID(r)

	oldJSON, _ := json.Marshal(oldValue)
	newJSON, _ := json.Marshal(newValue)

	log := models.AuditLog{
		ID:         uuid.New(),
		AdminID:    adminID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldJSON,
		NewValue:   newJSON,
		IPAddress:  r.RemoteAddr,
		UserAgent:  r.UserAgent(),
		CreatedAt:  time.Now(),
	}

	if err := s.db.Create(&log).Error; err != nil {
		fmt.Printf("failed to create audit log: %v\n", err)
	}
}
