package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CoachService struct {
	db              *gorm.DB
	analyticsSvc    *WorkoutAnalyticsService
	adherenceSvc    *AdherenceService
	targetSvc       *NutritionTargetService
	integrationSvc  *IntegrationRulesService
	notificationSvc *NotificationService
}

func NewCoachService(db *gorm.DB, analytics *WorkoutAnalyticsService, adherence *AdherenceService, target *NutritionTargetService, integration *IntegrationRulesService, notification *NotificationService) *CoachService {
	return &CoachService{
		db:              db,
		analyticsSvc:    analytics,
		adherenceSvc:    adherence,
		targetSvc:       target,
		integrationSvc:  integration,
		notificationSvc: notification,
	}
}

type CoachContext struct {
	DailySummary   interface{} `json:"daily_summary"`
	Streaks        interface{} `json:"streaks"`
	Records        interface{} `json:"records"`
	Recommendations interface{} `json:"recommendations"`
}

func (s *CoachService) GetCoachSummary(userID uuid.UUID, date time.Time) (*CoachContext, error) {
	summary, err := s.targetSvc.GetDailyNutritionSummary(userID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summary: %w", err)
	}

	streaks, err := s.adherenceSvc.GetUserStreaks(userID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get user streaks: %w", err)
	}

	records, err := s.analyticsSvc.GetUserPersonalRecords(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user records: %w", err)
	}

	recommendations, _ := s.integrationSvc.GetRecommendations(userID, date)

	return &CoachContext{
		DailySummary:    summary,
		Streaks:         streaks,
		Records:         records,
		Recommendations: recommendations,
	}, nil
}
func (s *CoachService) DispatchFunction(name string, args string, userID uuid.UUID) (interface{}, error) {
	var params map[string]interface{}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return nil, fmt.Errorf("invalid arguments JSON: %w", err)
		}
	}

	switch name {
	case "get_user_streaks":
		dateStr, _ := params["date"].(string)
		refDate := parseDateOrDefault(dateStr)
		return s.adherenceSvc.GetUserStreaks(userID, refDate)

	case "get_user_records":
		return s.analyticsSvc.GetUserPersonalRecords(userID)

	case "get_user_workout_stats":
		return s.analyticsSvc.GetUserWorkoutStats(userID)

	case "get_daily_summary":
		dateStr, _ := params["date"].(string)
		refDate := parseDateOrDefault(dateStr)
		return s.targetSvc.GetDailyNutritionSummary(userID, refDate)

	case "get_recommendations":
		dateStr, _ := params["date"].(string)
		refDate := parseDateOrDefault(dateStr)
		return s.integrationSvc.GetRecommendations(userID, refDate)

	case "get_notifications":
		limit := 20
		if l, ok := params["limit"].(float64); ok {
			limit = int(l)
		}
		offset := 0
		if o, ok := params["offset"].(float64); ok {
			offset = int(o)
		}
		return s.notificationSvc.ListNotifications(userID, limit, offset)

	case "get_unread_notification_count":
		count, err := s.notificationSvc.GetUnreadCount(userID)
		return map[string]interface{}{"unread_count": count}, err

	default:
		return nil, fmt.Errorf("unknown function: %s", name)
	}
}

func parseDateOrDefault(dateStr string) time.Time {
	if dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			return d
		}
	}
	return time.Now()
}

func (s *CoachService) GetTools() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_streaks",
				Description: "Get current streaks for workouts, meals, and weigh-ins, plus adherence percentages.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{
							"type":        "string",
							"description": "Reference date (YYYY-MM-DD)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_records",
				Description: "Get personal records for a user including heaviest sets and best 1RM estimates.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_workout_stats",
				Description: "Get workout statistics for a user.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_daily_summary",
				Description: "Get daily nutrition and workout summary.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{
							"type":        "string",
							"description": "Date for summary (YYYY-MM-DD)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_recommendations",
				Description: "Get personalized nutrition and workout recommendations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{
							"type":        "string",
							"description": "Date for recommendations (YYYY-MM-DD)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_notifications",
				Description: "List notifications for the user.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit":  map[string]interface{}{"type": "integer"},
						"offset": map[string]interface{}{"type": "integer"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_unread_notification_count",
				Description: "Get count of unread notifications.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}
