package services

import (
	"context"
	"time"

	"fitness-tracker/metrics"
	"fitness-tracker/models"

	"gorm.io/gorm"
)

type AdminDashboardService struct {
	db      *gorm.DB
	metrics *metrics.Metrics
}

func NewAdminDashboardService(db *gorm.DB, m *metrics.Metrics) *AdminDashboardService {
	return &AdminDashboardService{
		db:      db,
		metrics: m,
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

type adminDailyStats struct {
	TotalWorkouts int64
	TotalMeals    int64
}

func (s *AdminDashboardService) countDistinctActivityUsersOn(statDate time.Time) (int64, error) {
	if s.db.Dialector.Name() != "sqlite" {
		var count int64
		err := s.db.Table("user_activity_days").Where("stat_date = ?", statDate).Count(&count).Error
		return count, err
	}

	var count int64
	err := s.db.Raw(`
		SELECT COUNT(DISTINCT user_id)
		FROM (
			SELECT user_id FROM workouts WHERE date = ? AND deleted_at IS NULL
			UNION ALL
			SELECT user_id FROM meals WHERE date = ? AND deleted_at IS NULL
			UNION ALL
			SELECT user_id FROM weight_entries WHERE date = ? AND deleted_at IS NULL
		) activity_users
	`, statDate, statDate, statDate).Scan(&count).Error
	return count, err
}

func (s *AdminDashboardService) countDistinctActivityUsersBetween(startDate, endDate time.Time) (int64, error) {
	if s.db.Dialector.Name() != "sqlite" {
		var count int64
		err := s.db.Table("user_activity_days").
			Distinct("user_id").
			Where("stat_date >= ? AND stat_date <= ?", startDate, endDate).
			Count(&count).Error
		return count, err
	}

	var count int64
	err := s.db.Raw(`
		SELECT COUNT(DISTINCT user_id)
		FROM (
			SELECT user_id, date AS stat_date FROM workouts WHERE deleted_at IS NULL
			UNION ALL
			SELECT user_id, date AS stat_date FROM meals WHERE deleted_at IS NULL
			UNION ALL
			SELECT user_id, date AS stat_date FROM weight_entries WHERE deleted_at IS NULL
		) activity_days
		WHERE stat_date >= ? AND stat_date <= ?
	`, startDate, endDate).Scan(&count).Error
	return count, err
}

func (s *AdminDashboardService) countDistinctActivityUsersSince(startDate time.Time) (int64, error) {
	if s.db.Dialector.Name() != "sqlite" {
		var count int64
		err := s.db.Table("user_activity_days").
			Distinct("user_id").
			Where("stat_date >= ?", startDate).
			Count(&count).Error
		return count, err
	}

	var count int64
	err := s.db.Raw(`
		SELECT COUNT(DISTINCT user_id)
		FROM (
			SELECT user_id, date AS stat_date FROM workouts WHERE deleted_at IS NULL
			UNION ALL
			SELECT user_id, date AS stat_date FROM meals WHERE deleted_at IS NULL
			UNION ALL
			SELECT user_id, date AS stat_date FROM weight_entries WHERE deleted_at IS NULL
		) activity_days
		WHERE stat_date >= ?
	`, startDate).Scan(&count).Error
	return count, err
}

func (s *AdminDashboardService) loadDailyStats(statDate time.Time) (adminDailyStats, error) {
	var stats adminDailyStats

	if s.db.Dialector.Name() != "sqlite" {
		err := s.db.Table("daily_user_stats").
			Select("total_workouts, total_meals").
			Where("stat_date = ?", statDate).
			Scan(&stats).Error
		return stats, err
	}

	if err := s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", statDate).Count(&stats.TotalWorkouts).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Meal{}).Where("date = ? AND deleted_at IS NULL", statDate).Count(&stats.TotalMeals).Error; err != nil {
		return stats, err
	}

	return stats, nil
}

func (s *AdminDashboardService) GetExecutiveSummary(ctx context.Context) (*ExecutiveSummary, error) {
	_ = ctx
	summary, err := s.computeExecutiveSummary(ctx)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// GetRealtimeMetrics returns the current admin realtime metrics snapshot.
func (s *AdminDashboardService) GetRealtimeMetrics(ctx context.Context) (*RealtimeMetrics, error) {
	_ = ctx
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)

	activeUsers, err := s.countDistinctActivityUsersOn(today)
	if err != nil {
		return nil, err
	}

	todayStats, err := s.loadDailyStats(today)
	if err != nil {
		return nil, err
	}

	return &RealtimeMetrics{
		ActiveUsers:   int(activeUsers),
		WorkoutsToday: int(todayStats.TotalWorkouts),
		MealsToday:    int(todayStats.TotalMeals),
		Timestamp:     now,
	}, nil
}

func (s *AdminDashboardService) computeExecutiveSummary(ctx context.Context) (*ExecutiveSummary, error) {
	_ = ctx
	now := time.Now().UTC()
	summary := &ExecutiveSummary{
		UpdatedAt: now,
	}

	// 1. Total Users
	if err := s.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&summary.TotalUsers).Error; err != nil {
		return nil, err
	}

	// 2. DAU (Unique users with activity today)
	today := now.Truncate(24 * time.Hour)
	dau, err := s.countDistinctActivityUsersOn(today)
	if err != nil {
		return nil, err
	}
	summary.DAU = dau

	// 3. MAU (Last 30 days)
	thirtyDaysAgo := now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
	mau, err := s.countDistinctActivityUsersBetween(thirtyDaysAgo, today)
	if err != nil {
		return nil, err
	}
	summary.MAU = mau

	if summary.MAU > 0 {
		summary.DAUMAU_Ratio = float64(summary.DAU) / float64(summary.MAU) * 100
	}

	// 4. New Users 7d
	sevenDaysAgo := now.AddDate(0, 0, -7)
	if err := s.db.Model(&models.User{}).Where("created_at >= ? AND deleted_at IS NULL", sevenDaysAgo).Count(&summary.NewUsers7d).Error; err != nil {
		return nil, err
	}

	todayStats, err := s.loadDailyStats(today)
	if err != nil {
		return nil, err
	}
	summary.WorkoutsToday = todayStats.TotalWorkouts
	summary.MealsToday = todayStats.TotalMeals

	// 7. Error Rate (mocked for now, should ideally come from prometheus metrics or logs)
	summary.ErrorRate24h = 0.05 // 0.05%

	return summary, nil
}

func (s *AdminDashboardService) loadPopularExercises(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}

	var popular []map[string]any
	if s.db.Dialector.Name() == "sqlite" {
		err := s.db.Raw(`
			SELECT 
				e.name as exercise_name,
				COUNT(we.id) as usage_count,
				COUNT(DISTINCT w.user_id) as unique_users
			FROM exercises e
			LEFT JOIN workout_exercises we ON e.id = we.exercise_id
			LEFT JOIN workouts w ON we.workout_id = w.id
			WHERE w.deleted_at IS NULL
			GROUP BY e.id, e.name, e.primary_muscles
			ORDER BY usage_count DESC
			LIMIT ?
		`, limit).Scan(&popular).Error
		return popular, err
	}

	err := s.db.Table("exercise_popularity").
		Select("exercise_name, usage_count, unique_users").
		Order("usage_count DESC").
		Limit(limit).
		Scan(&popular).Error
	return popular, err
}

// GetPopularExercises returns the top exercises by usage for the admin dashboard.
func (s *AdminDashboardService) GetPopularExercises(ctx context.Context, limit int) ([]map[string]any, error) {
	_ = ctx
	return s.loadPopularExercises(limit)
}

func (s *AdminDashboardService) GetUserAnalytics(ctx context.Context) (map[string]any, error) {
	_ = ctx
	now := time.Now().UTC()
	var stats struct {
		TotalUsers    int64            `json:"total_users"`
		ActiveUsers7d int64            `json:"active_users_7d"`
		MAU           int64            `json:"mau"`
		GoalBreakdown []map[string]any `json:"goal_breakdown"`
		Growth        []map[string]any `json:"growth"`
		Retention     []map[string]any `json:"retention"`
	}

	if err := s.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	sevenDaysAgo := now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
	activeUsers7d, err := s.countDistinctActivityUsersSince(sevenDaysAgo)
	if err != nil {
		return nil, err
	}
	stats.ActiveUsers7d = activeUsers7d

	thirtyDaysAgo := now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
	stats.MAU, err = s.countDistinctActivityUsersSince(thirtyDaysAgo)
	if err != nil {
		return nil, err
	}

	if err := s.db.Model(&models.User{}).
		Select("goal, count(*) as count").
		Where("deleted_at IS NULL").
		Group("goal").
		Scan(&stats.GoalBreakdown).Error; err != nil {
		return nil, err
	}

	growthQuery := `
		SELECT DATE_TRUNC('day', created_at) as date, COUNT(*) as new_users
		FROM users
		WHERE deleted_at IS NULL
		GROUP BY date
		ORDER BY date DESC
		LIMIT 30
	`
	if s.db.Dialector.Name() == "sqlite" {
		growthQuery = `
			SELECT DATE(created_at) as date, COUNT(*) as new_users
			FROM users
			WHERE deleted_at IS NULL
			GROUP BY date
			ORDER BY date DESC
			LIMIT 30
		`
	}
	if err := s.db.Raw(growthQuery).Scan(&stats.Growth).Error; err != nil {
		return nil, err
	}

	if s.db.Dialector.Name() == "sqlite" {
		if err := s.db.Raw(`
			WITH cohorts AS (
				SELECT 
					id as user_id,
					strftime('%Y-%m-01', created_at) as cohort_month
				FROM users
				WHERE deleted_at IS NULL
			),
			active_months AS (
				SELECT DISTINCT
					user_id,
					strftime('%Y-%m-01', activity_date) as month
				FROM (
					SELECT user_id, date as activity_date FROM workouts WHERE deleted_at IS NULL
					UNION ALL
					SELECT user_id, date FROM meals WHERE deleted_at IS NULL
					UNION ALL
					SELECT user_id, date FROM weight_entries WHERE deleted_at IS NULL
				) activities
			)
			SELECT 
				c.cohort_month,
				COUNT(DISTINCT c.user_id) as cohort_size,
				SUM(CASE WHEN am.month = c.cohort_month THEN 1 ELSE 0 END) as month_0,
				SUM(CASE WHEN am.month = date(c.cohort_month, '+1 month') THEN 1 ELSE 0 END) as month_1,
				SUM(CASE WHEN am.month = date(c.cohort_month, '+2 months') THEN 1 ELSE 0 END) as month_2,
				SUM(CASE WHEN am.month = date(c.cohort_month, '+3 months') THEN 1 ELSE 0 END) as month_3,
				SUM(CASE WHEN am.month = date(c.cohort_month, '+6 months') THEN 1 ELSE 0 END) as month_6,
				SUM(CASE WHEN am.month = date(c.cohort_month, '+12 months') THEN 1 ELSE 0 END) as month_12
			FROM cohorts c
			LEFT JOIN active_months am ON c.user_id = am.user_id
			GROUP BY c.cohort_month
			ORDER BY c.cohort_month DESC
			LIMIT 12
		`).Scan(&stats.Retention).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Table("user_retention_cohorts").
			Order("cohort_month DESC").
			Limit(12).
			Scan(&stats.Retention).Error; err != nil {
			return nil, err
		}
	}

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
	_ = ctx
	var stats struct {
		TotalWorkouts    int64            `json:"total_workouts"`
		WorkoutsToday    int64            `json:"workouts_today"`
		TypeBreakdown    []map[string]any `json:"type_breakdown"`
		PopularExercises []map[string]any `json:"popular_exercises"`
		VolumeTrends     []map[string]any `json:"volume_trends"`
	}

	s.db.Model(&models.Workout{}).Where("deleted_at IS NULL").Count(&stats.TotalWorkouts)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", today).Count(&stats.WorkoutsToday)

	s.db.Model(&models.Workout{}).
		Select("type, count(*) as count").
		Where("deleted_at IS NULL").
		Group("type").
		Scan(&stats.TypeBreakdown)

	popularExercises, err := s.loadPopularExercises(20)
	if err != nil {
		return nil, err
	}
	stats.PopularExercises = popularExercises

	if s.db.Dialector.Name() == "sqlite" {
		s.db.Raw(`
			SELECT date as stat_date, COUNT(*) as total_workouts 
			FROM workouts 
			WHERE deleted_at IS NULL 
			GROUP BY date 
			ORDER BY stat_date DESC 
			LIMIT 30
		`).Scan(&stats.VolumeTrends)
	} else {
		s.db.Table("daily_user_stats").
			Select("stat_date, total_workouts").
			Order("stat_date DESC").
			Limit(30).
			Scan(&stats.VolumeTrends)
	}

	return map[string]any{
		"total_workouts":    stats.TotalWorkouts,
		"workouts_today":    stats.WorkoutsToday,
		"type_breakdown":    stats.TypeBreakdown,
		"popular_exercises": stats.PopularExercises,
		"volume_trends":     stats.VolumeTrends,
	}, nil
}

func (s *AdminDashboardService) GetNutritionAnalytics(ctx context.Context) (map[string]any, error) {
	_ = ctx
	var stats struct {
		TotalMealsLogged int64            `json:"total_meals_logged"`
		MealsToday       int64            `json:"meals_today"`
		TypeBreakdown    []map[string]any `json:"type_breakdown"`
		PopularFoods     []map[string]any `json:"popular_foods"`
		Adherence        []map[string]any `json:"adherence"`
	}

	s.db.Model(&models.Meal{}).Where("deleted_at IS NULL").Count(&stats.TotalMealsLogged)
	today := time.Now().UTC().Truncate(24 * time.Hour)
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
	_ = ctx
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
		"timestamp": time.Now().UTC(),
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

	return health, nil
}
