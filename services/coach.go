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
	exerciseLibSvc  *ExerciseLibClient
}

func NewCoachService(db *gorm.DB, analytics *WorkoutAnalyticsService, adherence *AdherenceService, target *NutritionTargetService, integration *IntegrationRulesService, notification *NotificationService, exerciseLib *ExerciseLibClient) *CoachService {
	return &CoachService{
		db:              db,
		analyticsSvc:    analytics,
		adherenceSvc:    adherence,
		targetSvc:       target,
		integrationSvc:  integration,
		notificationSvc: notification,
		exerciseLibSvc:  exerciseLib,
	}
}

type CoachContext struct {
	DailySummary    interface{} `json:"daily_summary"`
	Streaks         interface{} `json:"streaks"`
	Records         interface{} `json:"records"`
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

	case "search_exercises":
		if s.exerciseLibSvc == nil {
			return nil, fmt.Errorf("exercise library service not available")
		}
		query, _ := params["query"].(string)
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		topK := 10
		if tk, ok := params["top_k"].(float64); ok {
			topK = int(tk)
		}
		searchReq := LibSearchRequest{
			Query: query,
			TopK:  topK,
		}
		if lvl, ok := params["level"].(string); ok && lvl != "" {
			searchReq.Level = &lvl
		}
		if eq, ok := params["equipment"].(string); ok && eq != "" {
			searchReq.Equipment = &eq
		}
		if cat, ok := params["category"].(string); ok && cat != "" {
			searchReq.Category = &cat
		}
		if musc, ok := params["muscle"].(string); ok && musc != "" {
			searchReq.Muscle = &musc
		}
		return s.exerciseLibSvc.Search(searchReq)

	case "generate_program":
		if s.exerciseLibSvc == nil {
			return nil, fmt.Errorf("exercise library service not available")
		}
		goal, _ := params["goal"].(string)
		if goal == "" {
			goal = "general_fitness"
		}
		daysPerWeek := 3
		if d, ok := params["days_per_week"].(float64); ok {
			daysPerWeek = int(d)
		}
		sessionMinutes := 60
		if sm, ok := params["session_minutes"].(float64); ok {
			sessionMinutes = int(sm)
		}
		level := "intermediate"
		if l, ok := params["level"].(string); ok && l != "" {
			level = l
		}
		equipmentProfile := "full_gym"
		if ep, ok := params["equipment_profile"].(string); ok && ep != "" {
			equipmentProfile = ep
		}
		var focus []string
		if f, ok := params["focus"].([]interface{}); ok {
			for _, v := range f {
				if s, ok := v.(string); ok {
					focus = append(focus, s)
				}
			}
		}
		notes, _ := params["notes"].(string)
		programReq := LibProgramRequest{
			Goal:             goal,
			DaysPerWeek:      daysPerWeek,
			SessionMinutes:   sessionMinutes,
			Level:            level,
			EquipmentProfile: equipmentProfile,
			Focus:            focus,
			Notes:            notes,
		}
		return s.exerciseLibSvc.GetProgram(programReq)

	case "get_exercise_library_meta":
		if s.exerciseLibSvc == nil {
			return nil, fmt.Errorf("exercise library service not available")
		}
		return s.exerciseLibSvc.GetMeta()

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
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "search_exercises",
				Description: "Search the exercise library using semantic search. Finds exercises by natural language query (e.g. 'chest exercises', 'exercises for biceps', 'compound leg movements'). Supports filtering by level, equipment, category, and muscle group.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Natural language search query (e.g. 'chest exercises', 'back and biceps', 'compound movements for legs')",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results to return (default 10)",
						},
						"level": map[string]interface{}{
							"type":        "string",
							"description": "Difficulty level filter: beginner, intermediate, expert",
						},
						"equipment": map[string]interface{}{
							"type":        "string",
							"description": "Equipment filter: barbell, dumbbell, body only, cable, kettlebells, machine, etc.",
						},
						"category": map[string]interface{}{
							"type":        "string",
							"description": "Category filter: strength, cardio, stretching, plyometrics, etc.",
						},
						"muscle": map[string]interface{}{
							"type":        "string",
							"description": "Target muscle filter: chest, back, shoulders, biceps, triceps, legs, glutes, abs, etc.",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "generate_program",
				Description: "Generate a personalized multi-day workout program. Creates a structured training plan with exercises, sets, reps, and rest periods based on the user's goal, level, and available equipment.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"goal": map[string]interface{}{
							"type":        "string",
							"description": "Training goal: general_fitness, muscle_gain, fat_loss, strength, endurance, flexibility",
						},
						"days_per_week": map[string]interface{}{
							"type":        "integer",
							"description": "Number of training days per week (1-7, default 3)",
						},
						"session_minutes": map[string]interface{}{
							"type":        "integer",
							"description": "Target session duration in minutes (default 60)",
						},
						"level": map[string]interface{}{
							"type":        "string",
							"description": "Experience level: beginner, intermediate, expert (default intermediate)",
						},
						"equipment_profile": map[string]interface{}{
							"type":        "string",
							"description": "Available equipment: full_gym, home_gym, bodyweight_only, dumbbells_only, barbell_only",
						},
						"focus": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"type": "string"},
							"description": "Muscle groups to focus on (e.g. ['chest', 'triceps', 'shoulders'])",
						},
						"notes": map[string]interface{}{
							"type":        "string",
							"description": "Any additional preferences or constraints (injuries, limitations, etc.)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_exercise_library_meta",
				Description: "Get exercise library metadata including available equipment profiles, difficulty levels, categories, equipment types, muscle groups, and sample search queries. Use this to understand what options are available before searching exercises or generating programs.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}
