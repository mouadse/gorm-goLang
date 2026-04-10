package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationAutomationService struct {
	db              *gorm.DB
	notificationSvc *NotificationService
}

// NewNotificationAutomationService creates a notification automation service.
func NewNotificationAutomationService(db *gorm.DB) *NotificationAutomationService {
	return &NotificationAutomationService{
		db:              db,
		notificationSvc: NewNotificationService(db),
	}
}

// ProcessDueNotifications creates scheduled user notifications for the given reference time.
func (s *NotificationAutomationService) ProcessDueNotifications(ctx context.Context, now time.Time) (int, error) {
	now = now.UTC()

	var users []models.User
	if err := s.db.WithContext(ctx).
		Select("id").
		Where("banned_at IS NULL").
		Find(&users).Error; err != nil {
		return 0, err
	}

	created := 0
	for _, user := range users {
		select {
		case <-ctx.Done():
			return created, ctx.Err()
		default:
		}

		count, err := s.processUser(ctx, user.ID, now)
		if err != nil {
			return created, err
		}
		created += count
	}

	return created, nil
}

func (s *NotificationAutomationService) processUser(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	created := 0
	if added, err := s.createMissedMealReminder(ctx, userID, now); err != nil {
		return created, err
	} else {
		created += added
	}

	if added, err := s.createWorkoutReminder(ctx, userID, now); err != nil {
		return created, err
	} else {
		created += added
	}

	if added, err := s.createRestDayWarning(ctx, userID, now); err != nil {
		return created, err
	} else {
		created += added
	}

	if added, err := s.createRecommendationNotifications(ctx, userID, now); err != nil {
		return created, err
	} else {
		created += added
	}

	return created, nil
}

func (s *NotificationAutomationService) createMissedMealReminder(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	if now.Hour() < 18 {
		return 0, nil
	}

	dayStart := startOfDayUTC(now)
	nextDay := dayStart.AddDate(0, 0, 1)

	var mealCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.Meal{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, dayStart, nextDay).
		Count(&mealCount).Error; err != nil {
		return 0, err
	}
	if mealCount > 0 {
		return 0, nil
	}

	return s.createIfNew(ctx, userID, models.NotificationMissedMeal, "Meal logging reminder", "No meals have been logged today.", map[string]interface{}{
		"date": dayStart.Format("2006-01-02"),
	}, now, 20)
}

func (s *NotificationAutomationService) createWorkoutReminder(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	if now.Hour() < 9 {
		return 0, nil
	}

	windowStart := startOfDayUTC(now).AddDate(0, 0, -3)
	nextDay := startOfDayUTC(now).AddDate(0, 0, 1)

	var workoutCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.Workout{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, windowStart, nextDay).
		Count(&workoutCount).Error; err != nil {
		return 0, err
	}
	if workoutCount > 0 {
		return 0, nil
	}

	return s.createIfNew(ctx, userID, models.NotificationWorkoutReminder, "Workout reminder", "No workouts have been logged in the last 3 days.", map[string]interface{}{
		"window_start": windowStart.Format("2006-01-02"),
		"window_end":   startOfDayUTC(now).Format("2006-01-02"),
	}, now, 48)
}

func (s *NotificationAutomationService) createRestDayWarning(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	weekStart := startOfDayUTC(now).AddDate(0, 0, -6)
	nextDay := startOfDayUTC(now).AddDate(0, 0, 1)

	var workoutCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.Workout{}).
		Where("user_id = ? AND date >= ? AND date < ?", userID, weekStart, nextDay).
		Count(&workoutCount).Error; err != nil {
		return 0, err
	}
	if workoutCount < 5 {
		return 0, nil
	}

	return s.createIfNew(ctx, userID, models.NotificationRestDayWarning, "Recovery reminder", "Training frequency is high this week. Consider planning a rest or lighter day.", map[string]interface{}{
		"workouts_last_7_days": workoutCount,
		"window_start":         weekStart.Format("2006-01-02"),
		"window_end":           startOfDayUTC(now).Format("2006-01-02"),
	}, now, 72)
}

func (s *NotificationAutomationService) createRecommendationNotifications(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	recSvc := NewIntegrationRulesService(s.db.WithContext(ctx))
	recommendations, err := recSvc.GetRecommendations(userID, now)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, rule := range recommendations.Rules {
		if !rule.Applies {
			continue
		}

		notifType, ok := notificationTypeForRule(rule.ID)
		if !ok {
			continue
		}

		added, err := s.createIfNew(ctx, userID, notifType, rule.Name, strings.TrimSpace(rule.Description+": "+rule.Adjustment), map[string]interface{}{
			"rule_id":     rule.ID,
			"date":        startOfDayUTC(now).Format("2006-01-02"),
			"description": rule.Description,
			"adjustment":  rule.Adjustment,
		}, now, 20)
		if err != nil {
			return created, err
		}
		created += added
	}

	return created, nil
}

func (s *NotificationAutomationService) createIfNew(ctx context.Context, userID uuid.UUID, notifType models.NotificationType, title, message string, payload map[string]interface{}, now time.Time, windowHours int) (int, error) {
	cutoff := now.UTC().Add(-time.Duration(windowHours) * time.Hour)
	if value, ok := payload["date"].(string); ok && value != "" {
		if parsed, err := time.Parse("2006-01-02", value); err == nil {
			cutoff = parsed
		}
	}

	var existing models.Notification
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND created_at > ?", userID, notifType, cutoff).
		First(&existing).Error
	if err == nil {
		return 0, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	if _, err := s.notificationSvc.CreateNotification(userID, notifType, title, message, payload); err != nil {
		return 0, fmt.Errorf("create %s notification for user %s: %w", notifType, userID, err)
	}
	return 1, nil
}

func notificationTypeForRule(ruleID string) (models.NotificationType, bool) {
	switch ruleID {
	case "recovery_warning":
		return models.NotificationRecoveryWarning, true
	case "muscle_gain_protein_warning", "fat_loss_protein_warning":
		return models.NotificationLowProtein, true
	case "muscle_gain_calorie_warning", "fat_loss_deficit_warning":
		return models.NotificationGoalAlignment, true
	default:
		return "", false
	}
}
