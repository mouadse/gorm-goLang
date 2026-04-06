// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"encoding/json"
	"errors"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationService provides business logic for user notifications.
type NotificationService struct {
	db *gorm.DB
}

// NewNotificationService creates a new notification service.
func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// CreateNotification creates a new notification for a user.
func (s *NotificationService) CreateNotification(
	userID uuid.UUID,
	notifType models.NotificationType,
	title, message string,
	payload map[string]interface{},
) (*models.Notification, error) {
	var payloadJSON string
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadJSON = string(data)
	}

	notification := models.Notification{
		UserID:      userID,
		Type:        notifType,
		Title:       title,
		Message:     message,
		PayloadJSON: payloadJSON,
	}

	if err := s.db.Create(&notification).Error; err != nil {
		return nil, err
	}

	return &notification, nil
}

// GetNotification retrieves a notification by ID for a specific user.
func (s *NotificationService) GetNotification(userID, notificationID uuid.UUID) (*models.Notification, error) {
	var notification models.Notification
	err := s.db.First(&notification, "id = ? AND user_id = ?", notificationID, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}
	return &notification, nil
}

// ListNotifications retrieves all notifications for a user with pagination.
func (s *NotificationService) ListNotifications(userID uuid.UUID, limit, offset int) ([]models.Notification, error) {
	var notifications []models.Notification
	err := s.db.
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	return notifications, err
}

// MarkAsRead marks a notification as read.
func (s *NotificationService) MarkAsRead(userID, notificationID uuid.UUID) error {
	var notification models.Notification
	err := s.db.First(&notification, "id = ? AND user_id = ?", notificationID, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("notification not found")
		}
		return err
	}

	if notification.ReadAt != nil {
		return nil
	}

	now := time.Now().UTC()
	return s.db.Model(&notification).Update("read_at", now).Error
}

// MarkAllAsRead marks all notifications as read for a user.
func (s *NotificationService) MarkAllAsRead(userID uuid.UUID) error {
	now := time.Now().UTC()
	return s.db.
		Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Update("read_at", now).Error
}

// GetUnreadCount returns the count of unread notifications for a user.
func (s *NotificationService) GetUnreadCount(userID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.
		Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

// CreateFromIntegrationRule creates a notification from an integration rule warning.
func (s *NotificationService) CreateFromIntegrationRule(
	userID uuid.UUID,
	rule IntegrationRule,
) (*models.Notification, error) {
	return s.CreateNotification(
		userID,
		models.NotificationType(rule.ID),
		rule.Name,
		rule.Description+": "+rule.Adjustment,
		map[string]interface{}{
			"rule_id":     rule.ID,
			"name":        rule.Name,
			"description": rule.Description,
			"adjustment":  rule.Adjustment,
			"applies":     rule.Applies,
		},
	)
}

// CreateIfNotDuplicate creates a notification only if no duplicate exists within the time window.
// This prevents spamming the same notification type repeatedly.
func (s *NotificationService) CreateIfNotDuplicate(
	userID uuid.UUID,
	notifType models.NotificationType,
	title, message string,
	payload map[string]interface{},
	windowHours int,
) (*models.Notification, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(windowHours) * time.Hour)

	var existing models.Notification
	err := s.db.
		Where("user_id = ? AND type = ? AND created_at > ?", userID, notifType, cutoff).
		First(&existing).Error

	if err == nil {
		return &existing, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return s.CreateNotification(userID, notifType, title, message, payload)
}
