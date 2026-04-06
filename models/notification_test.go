package models_test

import (
	"testing"

	"fitness-tracker/models"

	"github.com/google/uuid"
)

func TestNotificationBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("sets UUID when not provided", func(t *testing.T) {
		notification := models.Notification{
			UserID:  uuid.New(),
			Type:    models.NotificationLowProtein,
			Title:   "Low Protein Warning",
			Message: "Your protein intake is below target",
		}

		if notification.ID != uuid.Nil {
			t.Fatalf("expected ID to be nil before test")
		}

		if err := notification.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if notification.ID == uuid.Nil {
			t.Fatalf("expected ID to be set, got nil")
		}
	})

	t.Run("rejects null user_id", func(t *testing.T) {
		notification := models.Notification{
			Type:    models.NotificationLowProtein,
			Title:   "Low Protein Warning",
			Message: "Your protein intake is below target",
		}

		err := notification.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for null user_id, got nil")
		}
	})

	t.Run("rejects empty type", func(t *testing.T) {
		notification := models.Notification{
			UserID:  uuid.New(),
			Title:   "Test",
			Message: "Test message",
		}

		err := notification.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for empty type, got nil")
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		notification := models.Notification{
			UserID:  uuid.New(),
			Type:    models.NotificationLowProtein,
			Message: "Test message",
		}

		err := notification.BeforeCreate(nil)
		if err == nil {
			t.Fatalf("expected error for empty title, got nil")
		}
	})

	t.Run("accepts valid notification", func(t *testing.T) {
		notification := models.Notification{
			UserID:  uuid.New(),
			Type:    models.NotificationWorkoutReminder,
			Title:   "Workout Reminder",
			Message: "Don't forget to log your workout today",
		}

		if err := notification.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error for valid notification, got %v", err)
		}
	})

	t.Run("accepts notification with payload", func(t *testing.T) {
		notification := models.Notification{
			UserID:      uuid.New(),
			Type:        models.NotificationRecoveryWarning,
			Title:       "Recovery Warning",
			Message:     "Your training load is high but protein intake is low",
			PayloadJSON: `{"protein_target": 150, "protein_intake": 80}`,
		}

		if err := notification.BeforeCreate(nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestNotificationTypes(t *testing.T) {
	t.Parallel()

	types := []models.NotificationType{
		models.NotificationLowProtein,
		models.NotificationMissedMeal,
		models.NotificationWorkoutReminder,
		models.NotificationRestDayWarning,
		models.NotificationExportReady,
		models.NotificationRecoveryWarning,
		models.NotificationGoalAlignment,
	}

	for _, notifType := range types {
		notification := models.Notification{
			UserID:  uuid.New(),
			Type:    notifType,
			Title:   "Test",
			Message: "Test message",
		}

		if err := notification.BeforeCreate(nil); err != nil {
			t.Fatalf("expected notification type %s to be valid, got error: %v", notifType, err)
		}
	}
}
