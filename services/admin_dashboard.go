package services

import (
	"context"
	"sort"
	"time"

	"fitness-tracker/metrics"
	"fitness-tracker/models"

	"github.com/google/uuid"
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

type activityTrendRow struct {
	StatDate      time.Time `json:"stat_date"`
	TotalWorkouts int64     `json:"total_workouts"`
	TotalMeals    int64     `json:"total_meals"`
	TotalWeights  int64     `json:"total_weights"`
}

type namedCountRow struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type goalCountRow struct {
	Goal  string `json:"goal"`
	Count int64  `json:"count"`
}

type datedCountRow struct {
	StatDate time.Time `json:"stat_date"`
	Count    int64     `json:"count"`
}

type exercisePopularityRow struct {
	ExerciseName string `json:"exercise_name"`
	UsageCount   int64  `json:"usage_count"`
	UniqueUsers  int64  `json:"unique_users"`
}

type retentionRow struct {
	CohortMonth time.Time `json:"cohort_month"`
	CohortSize  int64     `json:"cohort_size"`
	Month0      int64     `json:"month_0"`
	Month1      int64     `json:"month_1"`
	Month2      int64     `json:"month_2"`
	Month3      int64     `json:"month_3"`
	Month6      int64     `json:"month_6"`
	Month12     int64     `json:"month_12"`
}

type userDateRow struct {
	UserID uuid.UUID
	Date   time.Time
}

type userCreatedRow struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

func startOfMonthUTC(value time.Time) time.Time {
	return time.Date(value.UTC().Year(), value.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
}

func rowsToMaps[T any](rows []T) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		switch v := any(row).(type) {
		case activityTrendRow:
			result = append(result, map[string]any{
				"stat_date":      v.StatDate,
				"total_workouts": v.TotalWorkouts,
				"total_meals":    v.TotalMeals,
				"total_weights":  v.TotalWeights,
			})
		case datedCountRow:
			result = append(result, map[string]any{
				"date":      v.StatDate,
				"new_users": v.Count,
			})
		case namedCountRow:
			result = append(result, map[string]any{
				"type":  v.Name,
				"count": v.Count,
			})
		case goalCountRow:
			result = append(result, map[string]any{
				"goal":  v.Goal,
				"count": v.Count,
			})
		case exercisePopularityRow:
			result = append(result, map[string]any{
				"exercise_name": v.ExerciseName,
				"usage_count":   v.UsageCount,
				"unique_users":  v.UniqueUsers,
			})
		case retentionRow:
			result = append(result, map[string]any{
				"cohort_month": v.CohortMonth,
				"cohort_size":  v.CohortSize,
				"month_0":      v.Month0,
				"month_1":      v.Month1,
				"month_2":      v.Month2,
				"month_3":      v.Month3,
				"month_6":      v.Month6,
				"month_12":     v.Month12,
			})
		}
	}
	return result
}

func mergeDistinctUserIDs(groups ...[]uuid.UUID) int64 {
	seen := make(map[uuid.UUID]struct{})
	for _, group := range groups {
		for _, id := range group {
			seen[id] = struct{}{}
		}
	}
	return int64(len(seen))
}

func (s *AdminDashboardService) distinctUserIDsForDateRange(startDate *time.Time, endDate *time.Time) (int64, error) {
	load := func(model any) ([]uuid.UUID, error) {
		var ids []uuid.UUID
		query := s.db.Model(model).Where("deleted_at IS NULL")
		if startDate != nil {
			query = query.Where("date >= ?", *startDate)
		}
		if endDate != nil {
			query = query.Where("date <= ?", *endDate)
		}
		if err := query.Distinct("user_id").Pluck("user_id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	}

	workoutUsers, err := load(&models.Workout{})
	if err != nil {
		return 0, err
	}
	mealUsers, err := load(&models.Meal{})
	if err != nil {
		return 0, err
	}
	weightUsers, err := load(&models.WeightEntry{})
	if err != nil {
		return 0, err
	}

	return mergeDistinctUserIDs(workoutUsers, mealUsers, weightUsers), nil
}

func (s *AdminDashboardService) countDistinctActivityUsersOn(statDate time.Time) (int64, error) {
	return s.distinctUserIDsForDateRange(&statDate, &statDate)
}

func (s *AdminDashboardService) countDistinctActivityUsersBetween(startDate, endDate time.Time) (int64, error) {
	return s.distinctUserIDsForDateRange(&startDate, &endDate)
}

func (s *AdminDashboardService) countDistinctActivityUsersSince(startDate time.Time) (int64, error) {
	return s.distinctUserIDsForDateRange(&startDate, nil)
}

func (s *AdminDashboardService) loadDailyStats(statDate time.Time) (adminDailyStats, error) {
	var stats adminDailyStats

	if err := s.db.Model(&models.Workout{}).
		Where("date = ? AND deleted_at IS NULL", statDate).
		Count(&stats.TotalWorkouts).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&models.Meal{}).
		Where("date = ? AND deleted_at IS NULL", statDate).
		Count(&stats.TotalMeals).Error; err != nil {
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

	if err := s.db.Model(&models.User{}).Where("deleted_at IS NULL").Count(&summary.TotalUsers).Error; err != nil {
		return nil, err
	}

	today := now.Truncate(24 * time.Hour)
	dau, err := s.countDistinctActivityUsersOn(today)
	if err != nil {
		return nil, err
	}
	summary.DAU = dau

	thirtyDaysAgo := now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
	mau, err := s.countDistinctActivityUsersBetween(thirtyDaysAgo, today)
	if err != nil {
		return nil, err
	}
	summary.MAU = mau

	if summary.MAU > 0 {
		summary.DAUMAU_Ratio = float64(summary.DAU) / float64(summary.MAU) * 100
	}

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
	summary.ErrorRate24h = 0.05

	return summary, nil
}

func (s *AdminDashboardService) loadPopularExercises(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows []exercisePopularityRow
	if err := s.db.Model(&models.WorkoutExercise{}).
		Select("exercises.name as exercise_name, COUNT(workout_exercises.id) as usage_count, COUNT(DISTINCT workouts.user_id) as unique_users").
		Joins("JOIN exercises ON exercises.id = workout_exercises.exercise_id AND exercises.deleted_at IS NULL").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id AND workouts.deleted_at IS NULL").
		Where("workout_exercises.deleted_at IS NULL").
		Group("exercises.id, exercises.name").
		Order("usage_count DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	return rowsToMaps(rows), nil
}

func (s *AdminDashboardService) loadCountsByDate(model any) ([]datedCountRow, error) {
	var rows []datedCountRow
	if err := s.db.Model(model).
		Select("date as stat_date, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("date").
		Order("date DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *AdminDashboardService) loadActivityTrends(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 30
	}

	workouts, err := s.loadCountsByDate(&models.Workout{})
	if err != nil {
		return nil, err
	}
	meals, err := s.loadCountsByDate(&models.Meal{})
	if err != nil {
		return nil, err
	}
	weights, err := s.loadCountsByDate(&models.WeightEntry{})
	if err != nil {
		return nil, err
	}

	merged := make(map[time.Time]*activityTrendRow)
	accumulate := func(rows []datedCountRow, apply func(*activityTrendRow, int64)) {
		for _, row := range rows {
			day := row.StatDate.UTC().Truncate(24 * time.Hour)
			entry := merged[day]
			if entry == nil {
				entry = &activityTrendRow{StatDate: day}
				merged[day] = entry
			}
			apply(entry, row.Count)
		}
	}

	accumulate(workouts, func(row *activityTrendRow, count int64) {
		row.TotalWorkouts = count
	})
	accumulate(meals, func(row *activityTrendRow, count int64) {
		row.TotalMeals = count
	})
	accumulate(weights, func(row *activityTrendRow, count int64) {
		row.TotalWeights = count
	})

	ordered := make([]activityTrendRow, 0, len(merged))
	for _, row := range merged {
		ordered = append(ordered, *row)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].StatDate.After(ordered[j].StatDate)
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}

	return rowsToMaps(ordered), nil
}

func (s *AdminDashboardService) loadUserGrowth(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 30
	}

	var users []userCreatedRow
	if err := s.db.Model(&models.User{}).
		Select("id, created_at").
		Where("deleted_at IS NULL").
		Scan(&users).Error; err != nil {
		return nil, err
	}

	countsByDay := make(map[time.Time]int64)
	for _, user := range users {
		day := user.CreatedAt.UTC().Truncate(24 * time.Hour)
		countsByDay[day]++
	}

	rows := make([]datedCountRow, 0, len(countsByDay))
	for day, count := range countsByDay {
		rows = append(rows, datedCountRow{
			StatDate: day,
			Count:    count,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].StatDate.After(rows[j].StatDate)
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}

	return rowsToMaps(rows), nil
}

func (s *AdminDashboardService) loadRetention(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 12
	}

	var users []userCreatedRow
	if err := s.db.Model(&models.User{}).
		Select("id, created_at").
		Where("deleted_at IS NULL").
		Scan(&users).Error; err != nil {
		return nil, err
	}

	loadActivity := func(model any) ([]userDateRow, error) {
		var rows []userDateRow
		if err := s.db.Model(model).
			Select("user_id, date").
			Where("deleted_at IS NULL").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		return rows, nil
	}

	workouts, err := loadActivity(&models.Workout{})
	if err != nil {
		return nil, err
	}
	meals, err := loadActivity(&models.Meal{})
	if err != nil {
		return nil, err
	}
	weights, err := loadActivity(&models.WeightEntry{})
	if err != nil {
		return nil, err
	}

	activeMonths := make(map[uuid.UUID]map[time.Time]struct{})
	addActivityMonths := func(rows []userDateRow) {
		for _, row := range rows {
			month := startOfMonthUTC(row.Date)
			if activeMonths[row.UserID] == nil {
				activeMonths[row.UserID] = make(map[time.Time]struct{})
			}
			activeMonths[row.UserID][month] = struct{}{}
		}
	}

	addActivityMonths(workouts)
	addActivityMonths(meals)
	addActivityMonths(weights)

	cohorts := make(map[time.Time]*retentionRow)
	monthOffsets := []int{0, 1, 2, 3, 6, 12}
	for _, user := range users {
		cohortMonth := startOfMonthUTC(user.CreatedAt)
		row := cohorts[cohortMonth]
		if row == nil {
			row = &retentionRow{CohortMonth: cohortMonth}
			cohorts[cohortMonth] = row
		}
		row.CohortSize++

		userMonths := activeMonths[user.ID]
		for _, offset := range monthOffsets {
			month := cohortMonth.AddDate(0, offset, 0)
			if _, ok := userMonths[month]; !ok {
				continue
			}
			switch offset {
			case 0:
				row.Month0++
			case 1:
				row.Month1++
			case 2:
				row.Month2++
			case 3:
				row.Month3++
			case 6:
				row.Month6++
			case 12:
				row.Month12++
			}
		}
	}

	ordered := make([]retentionRow, 0, len(cohorts))
	for _, row := range cohorts {
		ordered = append(ordered, *row)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].CohortMonth.After(ordered[j].CohortMonth)
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}

	return rowsToMaps(ordered), nil
}

func (s *AdminDashboardService) loadWorkoutVolumeTrends(limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 30
	}

	var rows []datedCountRow
	if err := s.db.Model(&models.Workout{}).
		Select("date as stat_date, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("date").
		Order("date DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"stat_date":      row.StatDate,
			"total_workouts": row.Count,
		})
	}
	return result, nil
}

// GetActivityTrends returns recent aggregate workout, meal, and weight activity by day.
func (s *AdminDashboardService) GetActivityTrends(ctx context.Context, limit int) ([]map[string]any, error) {
	_ = ctx
	return s.loadActivityTrends(limit)
}

// GetPopularExercises returns the top exercises by usage for the admin dashboard.
func (s *AdminDashboardService) GetPopularExercises(ctx context.Context, limit int) ([]map[string]any, error) {
	_ = ctx
	return s.loadPopularExercises(limit)
}

// GetUserGrowth returns recent user signups grouped by day.
func (s *AdminDashboardService) GetUserGrowth(ctx context.Context, limit int) ([]map[string]any, error) {
	_ = ctx
	return s.loadUserGrowth(limit)
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

	var goalBreakdown []goalCountRow
	if err := s.db.Model(&models.User{}).
		Select("goal, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("goal").
		Scan(&goalBreakdown).Error; err != nil {
		return nil, err
	}
	stats.GoalBreakdown = rowsToMaps(goalBreakdown)

	stats.Growth, err = s.loadUserGrowth(30)
	if err != nil {
		return nil, err
	}

	stats.Retention, err = s.loadRetention(12)
	if err != nil {
		return nil, err
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

	if err := s.db.Model(&models.Workout{}).Where("deleted_at IS NULL").Count(&stats.TotalWorkouts).Error; err != nil {
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.Workout{}).Where("date = ? AND deleted_at IS NULL", today).Count(&stats.WorkoutsToday).Error; err != nil {
		return nil, err
	}

	var breakdown []namedCountRow
	if err := s.db.Model(&models.Workout{}).
		Select("type as name, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("type").
		Scan(&breakdown).Error; err != nil {
		return nil, err
	}
	stats.TypeBreakdown = rowsToMaps(breakdown)

	popularExercises, err := s.loadPopularExercises(20)
	if err != nil {
		return nil, err
	}
	stats.PopularExercises = popularExercises

	stats.VolumeTrends, err = s.loadWorkoutVolumeTrends(30)
	if err != nil {
		return nil, err
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

	if err := s.db.Model(&models.Meal{}).Where("deleted_at IS NULL").Count(&stats.TotalMealsLogged).Error; err != nil {
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := s.db.Model(&models.Meal{}).Where("date = ? AND deleted_at IS NULL", today).Count(&stats.MealsToday).Error; err != nil {
		return nil, err
	}

	var typeBreakdown []namedCountRow
	if err := s.db.Model(&models.Meal{}).
		Select("meal_type as name, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("meal_type").
		Scan(&typeBreakdown).Error; err != nil {
		return nil, err
	}
	stats.TypeBreakdown = rowsToMaps(typeBreakdown)

	if err := s.db.Table("meal_foods").
		Select("foods.name as name, COUNT(*) as count").
		Joins("JOIN foods ON foods.id = meal_foods.food_id AND foods.deleted_at IS NULL").
		Group("foods.id, foods.name").
		Order("count DESC").
		Limit(20).
		Scan(&typeBreakdown).Error; err != nil {
		return nil, err
	}
	stats.PopularFoods = make([]map[string]any, 0, len(typeBreakdown))
	for _, row := range typeBreakdown {
		stats.PopularFoods = append(stats.PopularFoods, map[string]any{
			"name":        row.Name,
			"usage_count": row.Count,
		})
	}

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

	sqlDB, err := s.db.DB()
	if err != nil {
		health["database"] = "disconnected"
		health["status"] = "unhealthy"
	} else {
		if err := sqlDB.PingContext(ctx); err != nil {
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
