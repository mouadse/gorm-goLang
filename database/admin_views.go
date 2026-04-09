package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// EnsureAdminViews creates or updates materialized views for the admin dashboard.
func EnsureAdminViews(db *gorm.DB) error {
	log.Println("checking admin dashboard materialized views...")

	if db.Dialector.Name() == "sqlite" {
		log.Println("skipping materialized views for sqlite (not supported)")
		return nil
	}

	views := map[string]string{
		"user_activity_days": `
			CREATE MATERIALIZED VIEW IF NOT EXISTS user_activity_days AS
			SELECT DISTINCT
				user_id,
				date AS stat_date
			FROM (
				SELECT user_id, date FROM workouts WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date FROM meals WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date FROM weight_entries WHERE deleted_at IS NULL
			) activity_days;
		`,
		"daily_user_stats": `
			CREATE MATERIALIZED VIEW IF NOT EXISTS daily_user_stats AS
			WITH all_activities AS (
				SELECT user_id, date, 'workout' as type, id FROM workouts WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date, 'meal' as type, id FROM meals WHERE deleted_at IS NULL
				UNION ALL
				SELECT user_id, date, 'weight' as type, id FROM weight_entries WHERE deleted_at IS NULL
			)
			SELECT 
				date as stat_date,
				COUNT(DISTINCT user_id) FILTER (WHERE type = 'workout') as users_with_workouts,
				COUNT(DISTINCT user_id) FILTER (WHERE type = 'meal') as users_with_meals,
				COUNT(DISTINCT user_id) FILTER (WHERE type = 'weight') as users_with_weights,
				COUNT(*) FILTER (WHERE type = 'workout') as total_workouts,
				COUNT(*) FILTER (WHERE type = 'meal') as total_meals,
				COUNT(*) FILTER (WHERE type = 'weight') as total_weights
			FROM all_activities
			GROUP BY date;
		`,
		"exercise_popularity": `
			CREATE MATERIALIZED VIEW IF NOT EXISTS exercise_popularity AS
			SELECT 
				e.id as exercise_id,
				e.name as exercise_name,
				e.primary_muscles AS muscle_group,
				COUNT(we.id) as usage_count,
				COUNT(DISTINCT w.user_id) as unique_users,
				MAX(w.date) as last_used
			FROM exercises e
			LEFT JOIN workout_exercises we ON e.id = we.exercise_id
			LEFT JOIN workouts w ON we.workout_id = w.id
			WHERE w.deleted_at IS NULL
			GROUP BY e.id, e.name, e.primary_muscles;
		`,
		"user_retention_cohorts": `
			CREATE MATERIALIZED VIEW IF NOT EXISTS user_retention_cohorts AS
			WITH cohorts AS (
				SELECT 
					id as user_id,
					DATE_TRUNC('month', created_at) as cohort_month
				FROM users
				WHERE deleted_at IS NULL
			),
			active_months AS (
				SELECT DISTINCT
					user_id,
					DATE_TRUNC('month', activity_date) as month
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
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month) as month_0,
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month + INTERVAL '1 month') as month_1,
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month + INTERVAL '2 months') as month_2,
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month + INTERVAL '3 months') as month_3,
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month + INTERVAL '6 months') as month_6,
				COUNT(DISTINCT am.user_id) FILTER (WHERE am.month = c.cohort_month + INTERVAL '12 months') as month_12
			FROM cohorts c
			LEFT JOIN active_months am ON c.user_id = am.user_id
			GROUP BY c.cohort_month
			ORDER BY c.cohort_month DESC;
		`,
	}

	indices := map[string]string{
		"idx_user_activity_days_unique": "CREATE UNIQUE INDEX IF NOT EXISTS idx_user_activity_days_unique ON user_activity_days(user_id, stat_date)",
		"idx_user_activity_days_date":   "CREATE INDEX IF NOT EXISTS idx_user_activity_days_date ON user_activity_days(stat_date)",
		"idx_daily_user_stats_date":     "CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_user_stats_date ON daily_user_stats(stat_date)",
		"idx_exercise_popularity_id":    "CREATE UNIQUE INDEX IF NOT EXISTS idx_exercise_popularity_id ON exercise_popularity(exercise_id)",
		"idx_exercise_popularity_usage": "CREATE INDEX IF NOT EXISTS idx_exercise_popularity_usage ON exercise_popularity(usage_count DESC)",
		"idx_cohort_month":              "CREATE UNIQUE INDEX IF NOT EXISTS idx_cohort_month ON user_retention_cohorts(cohort_month)",
	}

	for name, sql := range views {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("create materialized view %s: %w", name, err)
		}
	}

	for name, sql := range indices {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("create index %s: %w", name, err)
		}
	}

	return nil
}

// RefreshAdminViews refreshes all materialized views.
func RefreshAdminViews(db *gorm.DB) error {
	if db.Dialector.Name() == "sqlite" {
		return nil
	}

	views := []string{"user_activity_days", "daily_user_stats", "exercise_popularity", "user_retention_cohorts"}
	for _, view := range views {
		if err := db.Exec(fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s", view)).Error; err != nil {
			return fmt.Errorf("refresh materialized view %s: %w", view, err)
		}
	}
	return nil
}
