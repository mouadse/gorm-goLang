package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const coachToolDateLayout = "2006-01-02"

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

type CoachWeeklySummary struct {
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	TotalCalories  float64   `json:"total_calories"`
	TotalProtein   float64   `json:"total_protein"`
	TotalCarbs     float64   `json:"total_carbs"`
	TotalFat       float64   `json:"total_fat"`
	TargetCalories int       `json:"target_calories"`
	TargetProtein  int       `json:"target_protein"`
	TargetCarbs    int       `json:"target_carbs"`
	TargetFat      int       `json:"target_fat"`
	CalorieDelta   int       `json:"calorie_delta"`
	ProteinDelta   int       `json:"protein_delta"`
	CarbsDelta     int       `json:"carbs_delta"`
	FatDelta       int       `json:"fat_delta"`
	MealCount      int       `json:"meal_count"`
	WorkoutCount   int       `json:"workout_count"`
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
	case "get_user":
		return s.getUserProfile(userID)

	case "get_user_workouts":
		dateStr, _ := params["date"].(string)
		workoutType, _ := params["workout_type"].(string)
		limit := intFromParams(params, "limit", 10)
		return s.getUserWorkouts(userID, dateStr, workoutType, limit)

	case "get_workout":
		workoutIDStr, _ := params["workout_id"].(string)
		workoutID, err := uuid.Parse(workoutIDStr)
		if err != nil {
			return nil, fmt.Errorf("workout_id must be a valid UUID")
		}
		return s.getWorkout(userID, workoutID)

	case "get_user_meals":
		dateStr, _ := params["date"].(string)
		mealType, _ := params["meal_type"].(string)
		limit := intFromParams(params, "limit", 10)
		return s.getUserMeals(userID, dateStr, mealType, limit)

	case "get_user_weight_entries":
		dateStr, _ := params["date"].(string)
		startDateStr, _ := params["start_date"].(string)
		endDateStr, _ := params["end_date"].(string)
		limit := intFromParams(params, "limit", 10)
		return s.getUserWeightEntries(userID, dateStr, startDateStr, endDateStr, limit)

	case "get_user_streaks":
		dateStr, _ := params["date"].(string)
		refDate, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		return s.adherenceSvc.GetUserStreaks(userID, refDate)

	case "get_user_records":
		return s.analyticsSvc.GetUserPersonalRecords(userID)

	case "get_user_workout_stats":
		return s.analyticsSvc.GetUserWorkoutStats(userID)

	case "get_daily_summary":
		dateStr, _ := params["date"].(string)
		refDate, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		return s.targetSvc.GetDailyNutritionSummary(userID, refDate)

	case "get_weekly_summary":
		dateStr, _ := params["date"].(string)
		refDate, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		return s.getWeeklySummary(userID, refDate)

	case "get_recommendations":
		dateStr, _ := params["date"].(string)
		refDate, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
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

func intFromParams(params map[string]interface{}, key string, fallback int) int {
	if raw, ok := params[key].(float64); ok {
		return int(raw)
	}
	return fallback
}

func parseDateOrDefault(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Now().UTC().Truncate(24 * time.Hour), nil
	}

	parsed, err := time.Parse(coachToolDateLayout, dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must be %s", coachToolDateLayout)
	}

	return parsed.UTC(), nil
}

func (s *CoachService) getUserProfile(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *CoachService) getUserWorkouts(userID uuid.UUID, dateStr string, workoutType string, limit int) ([]models.Workout, error) {
	query := s.db.Model(&models.Workout{}).Where("user_id = ?", userID)

	if dateStr != "" {
		date, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		query = query.Where("date = ?", date)
	}
	if workoutType != "" {
		query = query.Where("type = ?", workoutType)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var workouts []models.Workout
	if err := query.Order("date desc, created_at desc, id asc").Find(&workouts).Error; err != nil {
		return nil, err
	}
	return workouts, nil
}

func (s *CoachService) getWorkout(userID uuid.UUID, workoutID uuid.UUID) (*models.Workout, error) {
	var workout models.Workout
	if err := s.db.
		Preload("WorkoutExercises", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc, created_at asc, id asc")
		}).
		Preload("WorkoutExercises.Exercise").
		Preload("WorkoutExercises.WorkoutSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc, created_at asc, id asc")
		}).
		Where("id = ? AND user_id = ?", workoutID, userID).
		First(&workout).Error; err != nil {
		return nil, err
	}
	return &workout, nil
}

func (s *CoachService) getUserMeals(userID uuid.UUID, dateStr string, mealType string, limit int) ([]models.Meal, error) {
	query := s.db.Model(&models.Meal{}).
		Preload("Items.Food").
		Where("user_id = ?", userID)

	if dateStr != "" {
		date, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		query = query.Where("date = ?", date)
	}
	if mealType != "" {
		query = query.Where("meal_type = ?", mealType)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var meals []models.Meal
	if err := query.Order("date desc, created_at desc, id asc").Find(&meals).Error; err != nil {
		return nil, err
	}
	for i := range meals {
		meals[i].CalculateTotals()
	}
	return meals, nil
}

func (s *CoachService) getUserWeightEntries(userID uuid.UUID, dateStr string, startDateStr string, endDateStr string, limit int) ([]models.WeightEntry, error) {
	query := s.db.Model(&models.WeightEntry{}).Where("user_id = ?", userID)

	if dateStr != "" {
		date, err := parseDateOrDefault(dateStr)
		if err != nil {
			return nil, err
		}
		query = query.Where("date = ?", date)
	}
	if startDateStr != "" {
		startDate, err := parseDateOrDefault(startDateStr)
		if err != nil {
			return nil, err
		}
		query = query.Where("date >= ?", startDate)
	}
	if endDateStr != "" {
		endDate, err := parseDateOrDefault(endDateStr)
		if err != nil {
			return nil, err
		}
		query = query.Where("date <= ?", endDate)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var entries []models.WeightEntry
	if err := query.Order("date desc, created_at desc, id asc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *CoachService) getWeeklySummary(userID uuid.UUID, date time.Time) (*CoachWeeklySummary, error) {
	startDate := startOfWeekUTC(date)
	endDate := startDate.AddDate(0, 0, 7)

	targets, err := s.targetSvc.GetUserNutritionTargets(userID)
	if err != nil {
		return nil, err
	}

	var meals []models.Meal
	if err := s.db.
		Preload("Items.Food").
		Where("user_id = ? AND date >= ? AND date < ?", userID, startDate, endDate).
		Find(&meals).Error; err != nil {
		return nil, err
	}

	var workouts []models.Workout
	if err := s.db.
		Where("user_id = ? AND date >= ? AND date < ?", userID, startDate, endDate).
		Find(&workouts).Error; err != nil {
		return nil, err
	}

	var totalCalories float64
	var totalProtein float64
	var totalCarbs float64
	var totalFat float64
	for i := range meals {
		meals[i].CalculateTotals()
		totalCalories += meals[i].TotalCalories
		totalProtein += meals[i].TotalProtein
		totalCarbs += meals[i].TotalCarbs
		totalFat += meals[i].TotalFat
	}

	weeklyTargetCalories := targets.Calories * 7
	weeklyTargetProtein := targets.Protein * 7
	weeklyTargetCarbs := targets.Carbs * 7
	weeklyTargetFat := targets.Fat * 7

	return &CoachWeeklySummary{
		StartDate:      startDate,
		EndDate:        endDate.AddDate(0, 0, -1),
		TotalCalories:  totalCalories,
		TotalProtein:   totalProtein,
		TotalCarbs:     totalCarbs,
		TotalFat:       totalFat,
		TargetCalories: weeklyTargetCalories,
		TargetProtein:  weeklyTargetProtein,
		TargetCarbs:    weeklyTargetCarbs,
		TargetFat:      weeklyTargetFat,
		CalorieDelta:   int(totalCalories) - weeklyTargetCalories,
		ProteinDelta:   int(totalProtein) - weeklyTargetProtein,
		CarbsDelta:     int(totalCarbs) - weeklyTargetCarbs,
		FatDelta:       int(totalFat) - weeklyTargetFat,
		MealCount:      len(meals),
		WorkoutCount:   len(workouts),
	}, nil
}

func (s *CoachService) GetTools() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user",
				Description: "Get the authenticated user's profile information, including name, email, goal, activity level, weight, height, and TDEE. Use this for questions like 'what is my name?' or 'what is my TDEE?'",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_workouts",
				Description: "List the authenticated user's logged workouts in reverse chronological order. Use this before answering questions about the last workout they logged.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":         map[string]interface{}{"type": "string", "description": "Filter by workout date (YYYY-MM-DD)"},
						"workout_type": map[string]interface{}{"type": "string", "description": "Filter by workout type such as push, pull, legs, cardio"},
						"limit":        map[string]interface{}{"type": "integer", "description": "Maximum number of workouts to return (default 10)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_workout",
				Description: "Get one logged workout with its exercises and sets. Use this after get_user_workouts when the user asks for details about a specific workout.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"workout_id": map[string]interface{}{"type": "string", "description": "Workout UUID"},
					},
					"required": []string{"workout_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_meals",
				Description: "List the authenticated user's meals with nutrition totals. Use this for meal history and recent nutrition logging questions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":      map[string]interface{}{"type": "string", "description": "Filter by meal date (YYYY-MM-DD)"},
						"meal_type": map[string]interface{}{"type": "string", "description": "Filter by meal type such as breakfast, lunch, dinner, snack"},
						"limit":     map[string]interface{}{"type": "integer", "description": "Maximum number of meals to return (default 10)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDef{
				Name:        "get_user_weight_entries",
				Description: "List the authenticated user's weight entries in reverse chronological order. Use this for weight progress questions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date":       map[string]interface{}{"type": "string", "description": "Filter by exact weigh-in date (YYYY-MM-DD)"},
						"start_date": map[string]interface{}{"type": "string", "description": "Filter to entries on or after this date (YYYY-MM-DD)"},
						"end_date":   map[string]interface{}{"type": "string", "description": "Filter to entries on or before this date (YYYY-MM-DD)"},
						"limit":      map[string]interface{}{"type": "integer", "description": "Maximum number of entries to return (default 10)"},
					},
				},
			},
		},
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
				Description: "Get the authenticated user's daily nutrition and workout summary.",
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
				Name:        "get_weekly_summary",
				Description: "Get the authenticated user's weekly nutrition and workout summary.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"date": map[string]interface{}{
							"type":        "string",
							"description": "Any date inside the week to summarize (YYYY-MM-DD)",
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
