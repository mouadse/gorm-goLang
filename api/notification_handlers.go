package api

import (
	"errors"
	"net/http"
	"strconv"

	"fitness-tracker/models"

	"github.com/google/uuid"
)

type notificationResponse struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	PayloadJSON string `json:"payload_json,omitempty"`
	ReadAt      string `json:"read_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func notificationToResponse(n models.Notification) notificationResponse {
	resp := notificationResponse{
		ID:        n.ID.String(),
		UserID:    n.UserID.String(),
		Type:      string(n.Type),
		Title:     n.Title,
		Message:   n.Message,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if n.PayloadJSON != "" {
		resp.PayloadJSON = n.PayloadJSON
	}

	if n.ReadAt != nil {
		resp.ReadAt = n.ReadAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return resp
}

func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	notifications, err := s.notificationSvc.ListNotifications(userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	responses := make([]notificationResponse, len(notifications))
	for i, n := range notifications {
		responses[i] = notificationToResponse(n)
	}

	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	notificationIDStr := r.PathValue("id")
	notificationID, err := uuid.Parse(notificationIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid notification ID"))
		return
	}

	err = s.notificationSvc.MarkAsRead(userID, notificationID)
	if err != nil {
		if err.Error() == "notification not found" {
			writeError(w, http.StatusNotFound, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}

	notification, err := s.notificationSvc.GetNotification(userID, notificationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, notificationToResponse(*notification))
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	err = s.notificationSvc.MarkAllAsRead(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (s *Server) handleGetUnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	count, err := s.notificationSvc.GetUnreadCount(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"unread_count": count})
}
