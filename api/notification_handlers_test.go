package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fitness-tracker/models"
	"fitness-tracker/services"
)

func TestNotifications(t *testing.T) {
	t.Parallel()

	t.Run("create and list notifications", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif@example.com", "NotifUser", "password123")
		svc := services.NewNotificationService(db)

		notification, err := svc.CreateNotification(
			userAuth.User.ID,
			models.NotificationLowProtein,
			"Low Protein Warning",
			"Your protein intake is below target",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		if notification == nil {
			t.Fatal("expected notification to be created")
		}

		notifications := requestJSONAuth[notificationList](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications", nil, http.StatusOK)

		if len(notifications) == 0 {
			t.Fatal("expected at least one notification")
		}
	})

	t.Run("list notifications with pagination", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif2@example.com", "NotifUser2", "password123")
		svc := services.NewNotificationService(db)

		for i := 0; i < 15; i++ {
			_, err := svc.CreateNotification(
				userAuth.User.ID,
				models.NotificationWorkoutReminder,
				"Reminder",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		page1 := requestJSONAuth[notificationList](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications?limit=10&offset=0", nil, http.StatusOK)
		if len(page1) != 10 {
			t.Fatalf("expected 10 notifications on page 1, got %d", len(page1))
		}

		page2 := requestJSONAuth[notificationList](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications?limit=10&offset=10", nil, http.StatusOK)
		if len(page2) != 5 {
			t.Fatalf("expected 5 notifications on page 2, got %d", len(page2))
		}
	})

	t.Run("mark notification as read", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif3@example.com", "NotifUser3", "password123")
		svc := services.NewNotificationService(db)

		notification, err := svc.CreateNotification(
			userAuth.User.ID,
			models.NotificationExportReady,
			"Export Ready",
			"Your export is ready for download",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		if notification.ReadAt != nil {
			t.Fatal("expected notification to start as unread")
		}

		updated := requestJSONAuth[notificationResponse](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/notifications/"+notification.ID.String()+"/read", nil, http.StatusOK)

		if updated.ReadAt == "" {
			t.Fatal("expected notification to be marked as read")
		}
	})

	t.Run("mark all notifications as read", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif4@example.com", "NotifUser4", "password123")
		svc := services.NewNotificationService(db)

		for i := 0; i < 5; i++ {
			_, err := svc.CreateNotification(
				userAuth.User.ID,
				models.NotificationLowProtein,
				"Test",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		result := requestJSONAuth[statusResponse](t, server, userAuth.AccessToken, http.MethodPatch, "/v1/notifications/read-all", nil, http.StatusOK)
		if result.Status != "success" {
			t.Fatalf("expected status success, got %s", result.Status)
		}

		count := requestJSONAuth[unreadCountResponse](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications/unread-count", nil, http.StatusOK)
		if count.UnreadCount != 0 {
			t.Fatalf("expected 0 unread notifications, got %d", count.UnreadCount)
		}
	})

	t.Run("get unread count", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif5@example.com", "NotifUser5", "password123")
		svc := services.NewNotificationService(db)

		for i := 0; i < 7; i++ {
			_, err := svc.CreateNotification(
				userAuth.User.ID,
				models.NotificationLowProtein,
				"Test",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		count := requestJSONAuth[unreadCountResponse](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications/unread-count", nil, http.StatusOK)
		if count.UnreadCount != 7 {
			t.Fatalf("expected 7 unread notifications, got %d", count.UnreadCount)
		}
	})

	t.Run("require authentication", func(t *testing.T) {
		server := newTestServer(t)

		req, _ := http.NewRequest(http.MethodGet, "/v1/notifications", nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("return 404 for non-existent notification", func(t *testing.T) {
		_, handler := newTestApp(t)
		server := handler
		userAuth := registerTestUser(t, server, "notif6@example.com", "NotifUser6", "password123")

		fakeID := "00000000-0000-0000-0000-000000000000"
		req, _ := http.NewRequest(http.MethodPatch, "/v1/notifications/"+fakeID+"/read", nil)
		req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("prevent cross-user access", func(t *testing.T) {
		db, handler := newTestApp(t)
		server := handler
		user1 := registerTestUser(t, server, "user1@example.com", "User1", "password123")
		user2 := registerTestUser(t, server, "user2@example.com", "User2", "password123")
		svc := services.NewNotificationService(db)

		notification, err := svc.CreateNotification(
			user1.User.ID,
			models.NotificationLowProtein,
			"Test",
			"Test message",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		req, _ := http.NewRequest(http.MethodPatch, "/v1/notifications/"+notification.ID.String()+"/read", nil)
		req.Header.Set("Authorization", "Bearer "+user2.AccessToken)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status %d for cross-user access, got %d", http.StatusNotFound, w.Code)
		}
	})
}

type notificationList []notificationResponse

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

type statusResponse struct {
	Status string `json:"status"`
}

type unreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}
