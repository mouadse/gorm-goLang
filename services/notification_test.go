package services

import (
	"fmt"
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNotificationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.Notification{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func createNotificationTestUser(t *testing.T, db *gorm.DB) uuid.UUID {
	userID := uuid.New()
	user := models.User{
		ID:           userID,
		Email:        fmt.Sprintf("test-%s@example.com", userID.String()[:8]),
		Name:         "Test User",
		PasswordHash: "hash",
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return userID
}

func TestNotificationService_CreateNotification(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("creates notification successfully", func(t *testing.T) {
		notification, err := svc.CreateNotification(
			userID,
			models.NotificationLowProtein,
			"Low Protein Warning",
			"Your protein intake is below target",
			nil,
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.ID == uuid.Nil {
			t.Fatal("expected notification ID to be set")
		}

		if notification.UserID != userID {
			t.Fatalf("expected user ID %s, got %s", userID, notification.UserID)
		}

		if notification.Type != models.NotificationLowProtein {
			t.Fatalf("expected type %s, got %s", models.NotificationLowProtein, notification.Type)
		}

		if notification.Title != "Low Protein Warning" {
			t.Fatalf("expected title 'Low Protein Warning', got %s", notification.Title)
		}

		if notification.ReadAt != nil {
			t.Fatal("expected notification to be unread")
		}
	})

	t.Run("creates notification with payload", func(t *testing.T) {
		payload := map[string]interface{}{
			"protein_target": 150,
			"protein_intake": 80,
		}

		notification, err := svc.CreateNotification(
			userID,
			models.NotificationRecoveryWarning,
			"Recovery Warning",
			"Training load is high but protein intake is low",
			payload,
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.PayloadJSON == "" {
			t.Fatal("expected payload JSON to be set")
		}
	})
}

func TestNotificationService_GetNotification(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("gets notification successfully", func(t *testing.T) {
		created, err := svc.CreateNotification(
			userID,
			models.NotificationWorkoutReminder,
			"Workout Reminder",
			"Don't forget to log your workout",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		notification, err := svc.GetNotification(userID, created.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.ID != created.ID {
			t.Fatalf("expected notification ID %s, got %s", created.ID, notification.ID)
		}
	})

	t.Run("returns error when notification not found", func(t *testing.T) {
		_, err := svc.GetNotification(userID, uuid.New())
		if err == nil {
			t.Fatal("expected error when notification not found")
		}

		if err.Error() != "notification not found" {
			t.Fatalf("expected 'notification not found' error, got %v", err)
		}
	})

	t.Run("returns error when user does not own notification", func(t *testing.T) {
		otherUserID := createNotificationTestUser(t, db)
		created, err := svc.CreateNotification(
			userID,
			models.NotificationExportReady,
			"Export Ready",
			"Your export is ready for download",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		_, err = svc.GetNotification(otherUserID, created.ID)
		if err == nil {
			t.Fatal("expected error when user does not own notification")
		}

		if err.Error() != "notification not found" {
			t.Fatalf("expected 'notification not found' error, got %v", err)
		}
	})
}

func TestNotificationService_ListNotifications(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("lists notifications for user", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			_, err := svc.CreateNotification(
				userID,
				models.NotificationLowProtein,
				"Warning",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		notifications, err := svc.ListNotifications(userID, 10, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(notifications) != 5 {
			t.Fatalf("expected 5 notifications, got %d", len(notifications))
		}
	})

	t.Run("orders notifications by created_at desc", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		first, err := svc.CreateNotification(
			userID,
			models.NotificationLowProtein,
			"First",
			"First message",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		time.Sleep(10 * time.Millisecond)

		second, err := svc.CreateNotification(
			userID,
			models.NotificationLowProtein,
			"Second",
			"Second message",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		notifications, err := svc.ListNotifications(userID, 10, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(notifications) < 2 {
			t.Fatal("expected at least 2 notifications")
		}

		if notifications[0].ID != second.ID {
			t.Fatal("expected most recent notification first")
		}

		if notifications[1].ID != first.ID {
			t.Fatal("expected older notification second")
		}
	})

	t.Run("supports pagination", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		for i := 0; i < 10; i++ {
			_, err := svc.CreateNotification(
				userID,
				models.NotificationLowProtein,
				"Test",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		page1, err := svc.ListNotifications(userID, 5, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(page1) != 5 {
			t.Fatalf("expected 5 notifications on page 1, got %d", len(page1))
		}

		page2, err := svc.ListNotifications(userID, 5, 5)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(page2) != 5 {
			t.Fatalf("expected 5 notifications on page 2, got %d", len(page2))
		}
	})

	t.Run("returns empty list when no notifications", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		notifications, err := svc.ListNotifications(userID, 10, 0)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(notifications) != 0 {
			t.Fatalf("expected empty list, got %d notifications", len(notifications))
		}
	})
}

func TestNotificationService_MarkAsRead(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("marks notification as read", func(t *testing.T) {
		notification, err := svc.CreateNotification(
			userID,
			models.NotificationWorkoutReminder,
			"Test",
			"Test message",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		if notification.ReadAt != nil {
			t.Fatal("expected notification to start as unread")
		}

		err = svc.MarkAsRead(userID, notification.ID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		updated, err := svc.GetNotification(userID, notification.ID)
		if err != nil {
			t.Fatalf("failed to get notification: %v", err)
		}

		if updated.ReadAt == nil {
			t.Fatal("expected notification to be marked as read")
		}

		now := time.Now().UTC()
		diff := now.Sub(*updated.ReadAt)
		if diff > 5*time.Second {
			t.Fatalf("expected read_at to be within 5 seconds of now, got %v", diff)
		}
	})

	t.Run("handles already read notification", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		notification, err := svc.CreateNotification(
			userID,
			models.NotificationExportReady,
			"Test",
			"Test message",
			nil,
		)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		err = svc.MarkAsRead(userID, notification.ID)
		if err != nil {
			t.Fatalf("failed to mark as read: %v", err)
		}

		err = svc.MarkAsRead(userID, notification.ID)
		if err != nil {
			t.Fatalf("expected no error on second mark as read, got %v", err)
		}
	})

	t.Run("returns error when notification not found", func(t *testing.T) {
		err := svc.MarkAsRead(userID, uuid.New())
		if err == nil {
			t.Fatal("expected error when notification not found")
		}

		if err.Error() != "notification not found" {
			t.Fatalf("expected 'notification not found' error, got %v", err)
		}
	})
}

func TestNotificationService_MarkAllAsRead(t *testing.T) {
	t.Run("marks all notifications as read", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		for i := 0; i < 3; i++ {
			_, err := svc.CreateNotification(
				userID,
				models.NotificationLowProtein,
				"Test",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}
		}

		count, err := svc.GetUnreadCount(userID)
		if err != nil {
			t.Fatalf("failed to get unread count: %v", err)
		}

		if count != 3 {
			t.Fatalf("expected 3 unread notifications, got %d", count)
		}

		err = svc.MarkAllAsRead(userID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		count, err = svc.GetUnreadCount(userID)
		if err != nil {
			t.Fatalf("failed to get unread count: %v", err)
		}

		if count != 0 {
			t.Fatalf("expected 0 unread notifications, got %d", count)
		}
	})
}

func TestNotificationService_GetUnreadCount(t *testing.T) {
	t.Run("returns correct unread count", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		for i := 0; i < 5; i++ {
			notification, err := svc.CreateNotification(
				userID,
				models.NotificationLowProtein,
				"Test",
				"Test message",
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create notification: %v", err)
			}

			if i < 2 {
				err = svc.MarkAsRead(userID, notification.ID)
				if err != nil {
					t.Fatalf("failed to mark notification as read: %v", err)
				}
			}
		}

		count, err := svc.GetUnreadCount(userID)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if count != 3 {
			t.Fatalf("expected 3 unread notifications, got %d", count)
		}
	})
}

func TestNotificationService_CreateFromIntegrationRule(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("creates notification from integration rule", func(t *testing.T) {
		rule := IntegrationRule{
			ID:          "recovery_warning",
			Name:        "Recovery Warning",
			Description: "Training load is high but protein intake is low",
			Applies:     true,
			Adjustment:  "Increase protein intake for optimal recovery",
		}

		notification, err := svc.CreateFromIntegrationRule(userID, rule)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.Type != models.NotificationType("recovery_warning") {
			t.Fatalf("expected type 'recovery_warning', got %s", notification.Type)
		}

		if notification.Title != "Recovery Warning" {
			t.Fatalf("expected title 'Recovery Warning', got %s", notification.Title)
		}
	})
}

func TestNotificationService_CreateIfNotDuplicate(t *testing.T) {
	db := setupNotificationTestDB(t)
	svc := NewNotificationService(db)
	userID := createNotificationTestUser(t, db)

	t.Run("creates notification if no duplicate exists", func(t *testing.T) {
		notification, err := svc.CreateIfNotDuplicate(
			userID,
			models.NotificationLowProtein,
			"Low Protein",
			"Your protein is low",
			nil,
			24,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification == nil {
			t.Fatal("expected notification to be created")
		}
	})

	t.Run("returns existing notification if duplicate exists within window", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		existing, err := svc.CreateIfNotDuplicate(
			userID,
			models.NotificationLowProtein,
			"Low Protein",
			"Your protein is low",
			nil,
			24,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		notification, err := svc.CreateIfNotDuplicate(
			userID,
			models.NotificationLowProtein,
			"Low Protein",
			"Your protein is low",
			nil,
			24,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.ID != existing.ID {
			t.Fatal("expected to return existing notification within time window")
		}
	})

	t.Run("creates new notification after time window expires", func(t *testing.T) {
		db := setupNotificationTestDB(t)
		svc := NewNotificationService(db)
		userID := createNotificationTestUser(t, db)

		_, err := svc.CreateIfNotDuplicate(
			userID,
			models.NotificationWorkoutReminder,
			"Workout Reminder",
			"Time to workout",
			nil,
			0,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		notification, err := svc.CreateIfNotDuplicate(
			userID,
			models.NotificationWorkoutReminder,
			"Workout Reminder",
			"Time to workout",
			nil,
			0,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification == nil {
			t.Fatal("expected notification to be created after time window")
		}
	})
}
