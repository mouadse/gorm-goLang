package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"fitness-tracker/database"
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

type adminClient struct {
	conn *websocket.Conn
	send chan []byte
}

type adminHub struct {
	clients    map[*adminClient]bool
	broadcast  chan []byte
	register   chan *adminClient
	unregister chan *adminClient
	mu         sync.Mutex
}

var hub = adminHub{
	broadcast:  make(chan []byte),
	register:   make(chan *adminClient),
	unregister: make(chan *adminClient),
	clients:    make(map[*adminClient]bool),
}

func (h *adminHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func init() {
	go hub.run()
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
	err := s.db.Table("daily_user_stats").
		Select("stat_date, total_workouts, total_meals, total_weights").
		Order("stat_date DESC").
		Limit(30).
		Scan(&trends).Error
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
	s.db.Raw(`
		SELECT DATE_TRUNC('day', created_at) as date, COUNT(*) as new_users
		FROM users
		WHERE deleted_at IS NULL
		GROUP BY date
		ORDER BY date DESC
		LIMIT 30
	`).Scan(&growth)
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
	var popular []map[string]any
	err := s.db.Table("exercise_popularity").
		Select("exercise_name, usage_count, unique_users").
		Order("usage_count DESC").
		Limit(20).
		Scan(&popular).Error
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
	var logs []models.AuditLog
	if err := s.db.Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) handleAdminRealtimeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	client := &adminClient{conn: conn, send: make(chan []byte, 256)}
	hub.register <- client

	go client.writePump()
}

func (c *adminClient) writePump() {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

// StartBackgroundTasks starts all recurring background tasks.
func (s *Server) StartBackgroundTasks() {
	// Real-time metrics broadcast
	tickerWS := time.NewTicker(5 * time.Second)
	go func() {
		for range tickerWS.C {
			metrics := s.collectRealtimeMetrics()
			data, _ := json.Marshal(metrics)
			hub.broadcast <- data
		}
	}()

	// Refresh materialized views every hour
	tickerMV := time.NewTicker(1 * time.Hour)
	go func() {
		for range tickerMV.C {
			log.Println("refreshing admin materialized views...")
			if err := database.RefreshAdminViews(s.db); err != nil {
				log.Printf("⚠️ failed to refresh admin views: %v", err)
			} else {
				log.Println("✅ admin views refreshed")
			}
		}
	}()
}

func (s *Server) collectRealtimeMetrics() map[string]any {
	today := time.Now().Truncate(24 * time.Hour)
	var activeUsers, workoutsToday, mealsToday int64

	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM (
		SELECT user_id FROM workouts WHERE date = ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM meals WHERE date = ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM weight_entries WHERE date = ? AND deleted_at IS NULL
	) activities`, today, today, today).Scan(&activeUsers)

	s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", today).Count(&workoutsToday)
	s.db.Model(&models.Meal{}).Where("date = ? AND deleted_at IS NULL", today).Count(&mealsToday)

	return map[string]any{
		"active_users":   activeUsers,
		"workouts_today": workoutsToday,
		"meals_today":    mealsToday,
		"timestamp":      time.Now(),
	}
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
