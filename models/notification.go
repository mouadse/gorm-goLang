package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotificationLowProtein      NotificationType = "low_protein_warning"
	NotificationMissedMeal      NotificationType = "missed_meal_logging"
	NotificationWorkoutReminder NotificationType = "workout_reminder"
	NotificationRestDayWarning  NotificationType = "rest_day_warning"
	NotificationExportReady     NotificationType = "export_ready"
	NotificationRecoveryWarning NotificationType = "recovery_warning"
	NotificationGoalAlignment   NotificationType = "goal_alignment_warning"
)

// Notification represents a user notification.
type Notification struct {
	ID          uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	Type        NotificationType `gorm:"type:varchar(50);not null" json:"type"`
	Title       string           `gorm:"type:varchar(255);not null" json:"title"`
	Message     string           `gorm:"type:text;not null" json:"message"`
	PayloadJSON string           `gorm:"type:text" json:"payload_json,omitempty"`
	ReadAt      *time.Time       `json:"read_at,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// BeforeCreate sets a new UUID before inserting and validates required fields.
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}

	if n.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}

	if n.Type == "" {
		return errors.New("type is required")
	}

	if n.Title == "" {
		return errors.New("title is required")
	}

	return nil
}
