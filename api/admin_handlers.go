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
	trends, err := s.adminSvc.GetActivityTrends(r.Context(), 30)
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
	growth, err := s.adminSvc.GetUserGrowth(r.Context(), 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

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

	// Subscribe to push events (user signups, etc.)
	eventCh := s.adminRealtime.Subscribe()
	defer s.adminRealtime.Unsubscribe(eventCh)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send an initial snapshot immediately
	if err := s.sendRealtimeSnapshot(conn, r.Context()); err != nil {
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return

		case eventData, ok := <-eventCh:
			if !ok {
				return
			}
			// Push the event (e.g. user_signup) immediately
			if err := conn.WriteMessage(websocket.TextMessage, eventData); err != nil {
				return
			}
			// Follow up with a fresh metrics snapshot so the dashboard
			// can update counters without waiting for the next tick.
			if err := s.sendRealtimeSnapshot(conn, r.Context()); err != nil {
				return
			}

		case <-ticker.C:
			if err := s.sendRealtimeSnapshot(conn, r.Context()); err != nil {
				return
			}
		}
	}
}

func (s *Server) sendRealtimeSnapshot(conn *websocket.Conn, ctx context.Context) error {
	metrics, err := s.collectRealtimeMetrics(ctx)
	if err != nil {
		log.Printf("collect realtime metrics failed: %v", err)
		return err
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("marshal realtime metrics failed: %v", err)
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (s *Server) collectRealtimeMetrics(ctx context.Context) (map[string]any, error) {
	realtime, err := s.adminSvc.GetRealtimeMetrics(ctx)
	if err != nil {
		return nil, err
	}

	totalUsers, newUsers7d, err := s.adminSvc.GetUserCountMetrics(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"type":           "metrics_update",
		"active_users":   realtime.ActiveUsers,
		"workouts_today": realtime.WorkoutsToday,
		"meals_today":    realtime.MealsToday,
		"total_users":    totalUsers,
		"new_users_7d":   newUsers7d,
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
