package services

import (
	"context"
	"encoding/json"
	"time"

	"fitness-tracker/metrics"
	"fitness-tracker/models"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AdminDashboardService struct {
	db          *gorm.DB
	redisClient *redis.Client
	metrics     *metrics.Metrics
}

func NewAdminDashboardService(db *gorm.DB, redis *redis.Client, m *metrics.Metrics) *AdminDashboardService {
	return &AdminDashboardService{
		db:          db,
		redisClient: redis,
		metrics:     m,
	}
}

type ExecutiveSummary struct {
	TotalUsers    int64     `json:"total_users"`
	DAU           int64     `json:"dau"`
	MAU           int64     `json:"mau"`
	DAUMAU_Ratio  float64   `json:"dau_mau_ratio"`
	NewUsers7d    int64     `json:"new_users_7d"`
	WorkoutsToday int64     `json:"workouts_today"`
	MealsToday    int64     `json:"meals_today"`
	ErrorRate24h  float64   `json:"error_rate_24h"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RealtimeMetrics struct {
	ActiveUsers       int       `json:"active_users"`
	WorkoutsToday     int       `json:"workouts_today"`
	MealsToday        int       `json:"meals_today"`
	APIRequestsPerMin int       `json:"api_requests_per_min"`
	ErrorRate         float64   `json:"error_rate"`
	Timestamp         time.Time `json:"timestamp"`
}

func (s *AdminDashboardService) GetExecutiveSummary(ctx context.Context) (*ExecutiveSummary, error) {
	cacheKey := "dashboard:executive_summary"
	if s.redisClient != nil {
		cached, err := s.redisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			var summary ExecutiveSummary
			if err := json.Unmarshal([]byte(cached), &summary); err == nil {
				return &summary, nil
			}
		}
	}

	summary, err := s.computeExecutiveSummary(ctx)
	if err != nil {
		return nil, err
	}

	if s.redisClient != nil {
		data, _ := json.Marshal(summary)
		s.redisClient.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return summary, nil
}

func (s *AdminDashboardService) computeExecutiveSummary(ctx context.Context) (*ExecutiveSummary, error) {
	summary := &ExecutiveSummary{
		UpdatedAt: time.Now(),
	}

	// 1. Total Users
	if err := s.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&summary.TotalUsers).Error; err != nil {
		return nil, err
	}

	// 2. DAU (Unique users with activity today)
	today := time.Now().Truncate(24 * time.Hour)
	err := s.db.Table("daily_user_stats").
		Select("users_with_workouts + users_with_meals + users_with_weights as dau"). // This is simplified, should be distinct users
		Where("stat_date = ?", today).
		Row().Scan(&summary.DAU)
	if err != nil {
		// Fallback if today's entry is not yet in materialized view (not refreshed)
		// Better approach: query raw tables for today's activity
		var rawDAU int64
		s.db.Raw(`
			SELECT COUNT(DISTINCT user_id) FROM (
				SELECT user_id FROM workouts WHERE date = ? AND deleted_at IS NULL
				UNION ALL
				SELECT user_id FROM meals WHERE date = ? AND deleted_at IS NULL
				UNION ALL
				SELECT user_id FROM weight_entries WHERE date = ? AND deleted_at IS NULL
			) activities`, today, today, today).Scan(&rawDAU)
		summary.DAU = rawDAU
	}

	// 3. MAU (Last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Truncate(24 * time.Hour)
	s.db.Raw(`
		SELECT COUNT(DISTINCT user_id) FROM (
			SELECT user_id FROM workouts WHERE date >= ? AND deleted_at IS NULL
			UNION ALL
			SELECT user_id FROM meals WHERE date >= ? AND deleted_at IS NULL
			UNION ALL
			SELECT user_id FROM weight_entries WHERE date >= ? AND deleted_at IS NULL
		) activities`, thirtyDaysAgo, thirtyDaysAgo, thirtyDaysAgo).Scan(&summary.MAU)

	if summary.MAU > 0 {
		summary.DAUMAU_Ratio = float64(summary.DAU) / float64(summary.MAU) * 100
	}

	// 4. New Users 7d
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	s.db.Model(&models.User{}).Where("created_at >= ? AND deleted_at IS NULL", sevenDaysAgo).Count(&summary.NewUsers7d)

	// 5. Workouts Today
	s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", today).Count(&summary.WorkoutsToday)

	// 6. Meals Today
	s.db.Model(&models.Meal{}).Where("date = ? AND deleted_at IS NULL", today).Count(&summary.MealsToday)

	// 7. Error Rate (mocked for now, should ideally come from prometheus metrics or logs)
	summary.ErrorRate24h = 0.05 // 0.05%

	return summary, nil
}

func (s *AdminDashboardService) GetUserAnalytics(ctx context.Context) (map[string]any, error) {
	var stats struct {
		TotalUsers    int64            `json:"total_users"`
		ActiveUsers7d int64            `json:"active_users_7d"`
		MAU           int64            `json:"mau"`
		GoalBreakdown []map[string]any `json:"goal_breakdown"`
		Growth        []map[string]any `json:"growth"`
		Retention     []map[string]any `json:"retention"`
	}

	s.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&stats.TotalUsers)

	sevenDaysAgo := time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour)
	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM (
		SELECT user_id FROM workouts WHERE date >= ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM meals WHERE date >= ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM weight_entries WHERE date >= ? AND deleted_at IS NULL
	) activities`, sevenDaysAgo, sevenDaysAgo, sevenDaysAgo).Scan(&stats.ActiveUsers7d)

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30).Truncate(24 * time.Hour)
	s.db.Raw(`SELECT COUNT(DISTINCT user_id) FROM (
		SELECT user_id FROM workouts WHERE date >= ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM meals WHERE date >= ? AND deleted_at IS NULL
		UNION ALL
		SELECT user_id FROM weight_entries WHERE date >= ? AND deleted_at IS NULL
	) activities`, thirtyDaysAgo, thirtyDaysAgo, thirtyDaysAgo).Scan(&stats.MAU)

	s.db.Model(&models.User{}).
		Select("goal, count(*) as count").
		Where("deleted_at IS NULL").
		Group("goal").
		Scan(&stats.GoalBreakdown)

	s.db.Raw(`
		SELECT DATE_TRUNC('day', created_at) as date, COUNT(*) as new_users
		FROM users
		WHERE deleted_at IS NULL
		GROUP BY date
		ORDER BY date DESC
		LIMIT 30
	`).Scan(&stats.Growth)

	s.db.Table("user_retention_cohorts").
		Order("cohort_month DESC").
		Limit(12).
		Scan(&stats.Retention)

	return map[string]any{
		"total_users":     stats.TotalUsers,
		"active_users_7d": stats.ActiveUsers7d,
		"mau":             stats.MAU,
		"goal_breakdown":  stats.GoalBreakdown,
		"growth":          stats.Growth,
		"retention":       stats.Retention,
	}, nil
}

func (s *AdminDashboardService) GetWorkoutAnalytics(ctx context.Context) (map[string]any, error) {
	var stats struct {
		TotalWorkouts    int64            `json:"total_workouts"`
		WorkoutsToday    int64            `json:"workouts_today"`
		TypeBreakdown    []map[string]any `json:"type_breakdown"`
		PopularExercises []map[string]any `json:"popular_exercises"`
		VolumeTrends     []map[string]any `json:"volume_trends"`
	}

	s.db.Model(&models.Workout{}).Where("deleted_at IS NULL").Count(&stats.TotalWorkouts)
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", today).Count(&stats.WorkoutsToday)

	s.db.Model(&models.Workout{}).
		Select("type, count(*) as count").
		Where("deleted_at IS NULL").
		Group("type").
		Scan(&stats.TypeBreakdown)

	s.db.Table("exercise_popularity").
		Select("exercise_name, usage_count, unique_users").
		Order("usage_count DESC").
		Limit(20).
		Scan(&stats.PopularExercises)

	s.db.Table("daily_user_stats").
		Select("stat_date, total_workouts").
		Order("stat_date DESC").
		Limit(30).
		Scan(&stats.VolumeTrends)

	return map[string]any{
		"total_workouts":    stats.TotalWorkouts,
		"workouts_today":    stats.WorkoutsToday,
		"type_breakdown":    stats.TypeBreakdown,
		"popular_exercises": stats.PopularExercises,
		"volume_trends":     stats.VolumeTrends,
	}, nil
}

func (s *AdminDashboardService) GetNutritionAnalytics(ctx context.Context) (map[string]any, error) {
	var stats struct {
		TotalMealsLogged int64            `json:"total_meals_logged"`
		MealsToday       int64            `json:"meals_today"`
		TypeBreakdown    []map[string]any `json:"type_breakdown"`
		PopularFoods     []map[string]any `json:"popular_foods"`
		Adherence        []map[string]any `json:"adherence"`
	}

	s.db.Model(&models.Meal{}).Where("deleted_at IS NULL").Count(&stats.TotalMealsLogged)
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&models.Meal{}).Where("date = ? AND deleted_at IS NULL", today).Count(&stats.MealsToday)

	s.db.Model(&models.Meal{}).
		Select("meal_type as type, count(*) as count").
		Where("deleted_at IS NULL").
		Group("meal_type").
		Scan(&stats.TypeBreakdown)

	s.db.Table("meal_foods").
		Select("foods.name, count(*) as usage_count").
		Joins("JOIN foods ON foods.id = meal_foods.food_id").
		Group("foods.id, foods.name").
		Order("usage_count DESC").
		Limit(20).
		Scan(&stats.PopularFoods)

	return map[string]any{
		"total_meals":    stats.TotalMealsLogged,
		"meals_today":    stats.MealsToday,
		"type_breakdown": stats.TypeBreakdown,
		"popular_foods":  stats.PopularFoods,
	}, nil
}

func (s *AdminDashboardService) GetModerationAnalytics(ctx context.Context) (map[string]any, error) {
	var stats struct {
		PendingExports   int64 `json:"pending_exports"`
		FailedExports    int64 `json:"failed_exports"`
		CompletedExports int64 `json:"completed_exports"`
		DeletionRequests int64 `json:"deletion_requests"`
	}
	s.db.Model(&ExportJob{}).Where("status = ?", ExportPending).Count(&stats.PendingExports)
	s.db.Model(&ExportJob{}).Where("status = ?", ExportFailed).Count(&stats.FailedExports)
	s.db.Model(&ExportJob{}).Where("status = ?", ExportCompleted).Count(&stats.CompletedExports)
	s.db.Model(&DeletionRequest{}).Count(&stats.DeletionRequests)

	return map[string]any{
		"pending_exports":   stats.PendingExports,
		"failed_exports":    stats.FailedExports,
		"completed_exports": stats.CompletedExports,
		"deletion_requests": stats.DeletionRequests,
	}, nil
}

func (s *AdminDashboardService) GetSystemHealth(ctx context.Context) (map[string]any, error) {
	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
	}

	// Check DB
	sqlDB, err := s.db.DB()
	if err != nil {
		health["database"] = "disconnected"
		health["status"] = "unhealthy"
	} else {
		if err := sqlDB.Ping(); err != nil {
			health["database"] = "unhealthy"
			health["status"] = "unhealthy"
		} else {
			health["database"] = "healthy"
			stats := sqlDB.Stats()
			health["database_stats"] = map[string]any{
				"open_connections": stats.OpenConnections,
				"in_use":           stats.InUse,
				"idle":             stats.Idle,
			}
		}
	}

	// Check Redis
	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err != nil {
			health["redis"] = "unhealthy"
		} else {
			health["redis"] = "healthy"
		}
	} else {
		health["redis"] = "not_configured"
	}

	return health, nil
}
