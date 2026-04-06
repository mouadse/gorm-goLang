# Comprehensive Admin Dashboard Plan for UM6P_FIT

## Executive Summary

This plan merges two dashboard proposals and fills critical gaps to provide **complete visibility** into all aspects of the UM6P_FIT fitness tracking platform. The dashboard will serve administrators with real-time metrics, business intelligence, content management, and system monitoring capabilities.

### Key Innovations Over Individual Plans

1. **2FA & Session Analytics** - Leverages existing `two_factor.go` and `auth_sessions.go`
2. **Export Job Monitoring** - Tracks pending/failed export jobs (existing export service)
3. **USDA Import Metrics** - Tracks food import success/failure rates
4. **Recipe & Favorite Food Analytics** - Tracks recipe creation and popularity
5. **Cross-Feature Correlations** - Analyzes engagement patterns across features
6. **Seasonal Pattern Recognition** - Accounts for fitness seasonality
7. **Comprehensive Audit Trail** - Tracks all admin actions for accountability
8. **Real-time WebSocket Architecture** - Sub-second updates for critical metrics
9. **Materialized Views & Caching** - Performance optimization for heavy queries

---

## Dashboard Architecture

### Technology Stack

```
Frontend:     React 18+ TypeScript + Tailwind CSS + shadcn/ui
Charts:       Recharts (composition-based, typed)
Real-time:    WebSocket with auto-reconnect
Backend:      Go API extensions with admin endpoints
Auth:         Existing admin role (RequireAdmin middleware)
Caching:      Redis for dashboard metrics
Database:     PostgreSQL with materialized views
Monitoring:    Prometheus + Grafana (existing)
```

### Database Optimizations

```sql
-- Materialized view for daily user stats
CREATE MATERIALIZED VIEW daily_user_stats AS
SELECT 
    DATE(COALESCE(w.date, m.date, we.date)) as stat_date,
    COUNT(DISTINCT w.user_id) as users_with_workouts,
    COUNT(DISTINCT m.user_id) as users_with_meals,
    COUNT(DISTINCT we.user_id) as users_with_weights,
    COUNT(*) FILTER (WHERE w.id IS NOT NULL) as total_workouts,
    COUNT(*) FILTER (WHERE m.id IS NOT NULL) as total_meals,
    COUNT(*) FILTER (WHERE we.id IS NOT NULL) as total_weights
FROM workouts w
FULL OUTER JOIN meals m ON w.user_id = m.user_id AND w.date = m.date
FULL OUTER JOIN weight_entries we ON COALESCE(w.user_id, m.user_id) = we.user_id 
    AND COALESCE(w.date, m.date) = we.date
GROUP BY DATE(COALESCE(w.date, m.date, we.date));

CREATE UNIQUE INDEX idx_daily_user_stats_date ON daily_user_stats(stat_date);

-- Refresh hourly via cron job
```

```sql
-- Materialized view for exercise popularity
CREATE MATERIALIZED VIEW exercise_popularity AS
SELECT 
    e.id as exercise_id,
    e.name as exercise_name,
    e.muscle_group,
    COUNT(we.id) as usage_count,
    COUNT(DISTINCT w.user_id) as unique_users,
    MAX(w.date) as last_used
FROM exercises e
LEFT JOIN workout_exercises we ON e.id = we.exercise_id
LEFT JOIN workouts w ON we.workout_id = w.id
WHERE w.deleted_at IS NULL
GROUP BY e.id, e.name, e.muscle_group;

CREATE INDEX idx_exercise_popularity_usage ON exercise_popularity(usage_count DESC);
```

```sql
-- User retention cohort analysis (refresh nightly)
CREATE MATERIALIZED VIEW user_retention_cohorts AS
WITH cohorts AS (
    SELECT 
        user_id,
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
```

### Backend Service Layer

```go
// services/admin_dashboard_service.go
package services

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

// Cached dashboard metrics with Redis
func (s *AdminDashboardService) GetExecutiveSummary(ctx context.Context) (*ExecutiveSummary, error) {
    cacheKey := "dashboard:executive_summary"
    cached, err := s.redisClient.Get(ctx, cacheKey).Result()
    if err == nil {
        var summary ExecutiveSummary
        json.Unmarshal([]byte(cached), &summary)
        return &summary, nil
    }
    
    // Compute from database
    summary := s.computeExecutiveSummary(ctx)
    
    // Cache for 5 minutes
    data, _ := json.Marshal(summary)
    s.redisClient.Set(ctx, cacheKey, data, 5*time.Minute)
    
    return summary, nil
}
```

---

## Dashboard Sections (10 Major Sections)

### 1. Executive Summary Dashboard

**Purpose**: High-level KPIs for quick health check

**Layout**: 
- Top: 8 KPI cards with real-time indicators
- Middle: Trend sparklines (30-day)
- Bottom: System health status

**KPIs**:

| Metric | Calculation | Real-time | Cache TTL |
|--------|-------------|-----------|-----------|
| **Total Users** | `COUNT(*) FROM users WHERE deleted_at IS NULL` | No | 5min |
| **DAU** | Unique users with activity today | Yes (WS) | - |
| **MAU** | Unique users active in 30 days | No | 1hr |
| **DAU/MAU Ratio** | DAU / MAU * 100 | No | 1hr |
| **New Users (7d)** | `COUNT(*) WHERE created_at >= NOW() - 7d` | No | 1hr |
| **Workouts Today** | `COUNT(*) FROM workouts WHERE date = TODAY` | Yes (WS) | - |
| **Meals Today** | `COUNT(*) FROM meals WHERE date = TODAY` | Yes (WS) | - |
| **Error Rate (24h)** | Prometheus: `rate(http_requests{status=~"5.."}[24h])` | Yes (WS) | - |

**Trend Sparklines** (30-day historical data):
- User growth
- Daily workouts
- Daily meals logged
- Error rate

**WebSocket Updates**:
```go
type RealtimeMetrics struct {
    ActiveUsers       int       `json:"active_users"`
    WorkoutsToday     int       `json:"workouts_today"`
    MealsToday        int       `json:"meals_today"`
    APIRequestsPerMin int       `json:"api_requests_per_min"`
    ErrorRate         float64   `json:"error_rate"`
    Timestamp         time.Time `json:"timestamp"`
}
// Broadcast every 5 seconds
```

---

### 2. User Analytics Dashboard

**Purpose**: Understand user growth, engagement, and behavior patterns

**KPIs**:

#### 2.1 Growth Metrics

| KPI | Calculation | Refresh |
|-----|-------------|---------|
| **New Users Today** | `COUNT(*) WHERE created_at::date = CURRENT_DATE` | 1min |
| **New Users This Week** | `COUNT(*) WHERE created_at >= week_start` | 5min |
| **New Users This Month** | `COUNT(*) WHERE created_at >= month_start` | 15min |
| **Growth Rate (MoM)** | `(this_month - last_month) / last_month * 100` | Daily |
| **Churn Rate** | Users inactive 30+ days / total users | Daily |

#### 2.2 Engagement Metrics

| KPI | Calculation | Refresh |
|-----|-------------|---------|
| **DAU** | Unique users with workouts/meals/weights today | Real-time |
| **WAU** | Unique users active in 7 days | Hourly |
| **MAU** | Unique users active in 30 days | Hourly |
| **DAU/MAU** | Engagement stickiness (target >20%) | Hourly |
| **Avg Sessions/User** | From `auth_sessions` table | Daily |
| **Avg Session Duration** | `last_activity_at - created_at` | Daily |

#### 2.3 Engagement Funnel (NEW - Not in either plan!)

```
Registration (100%) →
Profile Complete (>80%) →
First Workout (>60%) →
First Meal (>50%) →
Week 1 Active (>40%) →
Week 2 Retention (>25%) →
Week 4 Retention (>15%) →
Month 2 Retention (>10%)
```

**Implementation**:
```go
type EngagementFunnel struct {
    Registered      int       `json:"registered"`
    ProfileComplete int       `json:"profile_complete"`
    FirstWorkout    int       `json:"first_workout"`
    FirstMeal       int       `json:"first_meal"`
    Week1Active     int       `json:"week_1_active"`
    Week2Retention  int       `json:"week_2_retention"`
    Week4Retention  int       `json:"week_4_retention"`
    Month2Retention int       `json:"month_2_retention"`
    ConversionRates []float64 `json:"conversion_rates"`
}
```

#### 2.4 Demographics Breakdown

| Dimension | Values |
|-----------|--------|
| **Goal Distribution** | build_muscle, lose_fat, maintain (percentages) |
| **Activity Level** | sedentary, lightly_active, moderately_active, active, very_active |
| **Age Brackets** | 18-25, 26-35, 36-45, 46-55, 55+ |
| **2FA Adoption** | % users with `two_factor_enabled = true` (NEW!) |
| **Session Count Distribution** | Users with 1-2, 3-5, 6-10, 10+ sessions |

#### 2.5 Cohort Analysis

Track retention by registration cohort:
- Month 0 (signup month)
- Month 1 retention
- Month 2 retention
- Month 3 retention
- Month 6 retention
- Month 12 retention

**Visual**: Heatmap showing retention percentages by cohort

#### 2.6 Session Analytics (NEW!)

Leverage `auth_sessions` table:
```sql
-- Concurrent sessions
SELECT COUNT(*) as concurrent_sessions
FROM auth_sessions
WHERE expires_at > NOW() AND last_activity_at > NOW() - INTERVAL '15 minutes';

-- Session distribution
SELECT 
    CASE 
        WHEN count_sessions BETWEEN 1 AND 2 THEN '1-2'
        WHEN count_sessions BETWEEN 3 AND 5 THEN '3-5'
        WHEN count_sessions BETWEEN 6 AND 10 THEN '6-10'
        ELSE '10+'
    END as bucket,
    COUNT(*) as users
FROM (
    SELECT user_id, COUNT(*) as count_sessions
    FROM auth_sessions
    WHERE expires_at > NOW()
    GROUP BY user_id
) sessions
GROUP BY bucket;
```

---

### 3. Workout Analytics Dashboard

**Purpose**: Track workout patterns, progress, and trends

**KPIs**:

#### 3.1 Volume Metrics

| KPI | Calculation | Granularity |
|-----|-------------|-------------|
| **Total Workouts** | `COUNT(*) FROM workouts` | All time |
| **Workouts Today** | `COUNT(*) WHERE date = TODAY` | Daily |
| **Workouts This Week** | `COUNT(*) WHERE date >= week_start` | Weekly |
| **Workouts This Month** | `COUNT(*) WHERE date >= month_start` | Monthly |
| **Total Duration** | `SUM(duration)` | Weekly/Monthly |
| **Total Volume (kg)** | `SUM(weight * reps)` from completed sets | Weekly |
| **Avg Workout Duration** | `AVG(duration)` | Weekly |

#### 3.2 Workout Type Distribution

```sql
SELECT type, COUNT(*) as count, 
       ROUND(COUNT(*)::NUMERIC / SUM(COUNT(*)) OVER() * 100, 2) as percentage
FROM workouts
WHERE date >= CURRENT_DATE - INTERVAL '30 days'
  AND deleted_at IS NULL
GROUP BY type
ORDER BY count DESC;
```

#### 3.3 Exercise Popularity (Top 20)

```sql
SELECT 
    e.name,
    e.muscle_group,
    COUNT(we.id) as usage_count,
    COUNT(DISTINCT w.user_id) as unique_users,
    ROUND(AVG(ws.weight), 2) as avg_weight,
    ROUND(AVG(ws.reps), 1) as avg_reps
FROM exercises e
JOIN workout_exercises we ON e.id = we.exercise_id
JOIN workouts w ON we.workout_id = w.id
JOIN workout_sets ws ON we.id = ws.workout_exercise_id
WHERE w.date >= CURRENT_DATE - INTERVAL '30 days'
  AND ws.completed = true
GROUP BY e.id, e.name, e.muscle_group
ORDER BY usage_count DESC
LIMIT 20;
```

#### 3.4 Personal Records

| KPI | Description |
|-----|-------------|
| **PRs This Week** | Count of new personal records |
| **PRs by Exercise Type** | Distribution across exercises |
| **Avg Time Between PRs** | Days between PR attempts per user |
| **PR Leaderboard** | Top 10 users by PR count |

#### 3.5 Program & Template Usage

| KPI | Source |
|-----|--------|
| **Active Programs** | `workout_programs` with enrolled users |
| **Program Completion Rate** | Users completing all weeks |
| **Template Usage Count** | Times templates applied |
| **Most Popular Templates** | Top 10 by usage |
| **Most Popular Programs** | Top 10 by enrollment |

#### 3.6 Cardio Metrics

| KPI | Calculation |
|-----|-------------|
| **Total Cardio Sessions** | `COUNT(*) FROM workout_cardio_entries` |
| **Total Duration** | `SUM(duration_minutes)` |
| **Total Distance** | `SUM(distance_km)` |
| **Avg Duration** | `AVG(duration_minutes)` |
| **Cardio Types Distribution** | Running, cycling, swimming, etc. |

**Implementation**:
```sql
-- Cardio type distribution
SELECT 
    type,
    COUNT(*) as session_count,
    SUM(duration_minutes) as total_minutes,
    SUM(distance_km) as total_km
FROM workout_cardio_entries
WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY type
ORDER BY session_count DESC;
```

---

### 4. Nutrition Analytics Dashboard

**Purpose**: Monitor nutritional logging behavior and macro adherence

**KPIs**:

#### 4.1 Meal Logging Activity

| KPI | Calculation | Granularity |
|-----|-------------|-------------|
| **Total Meals Logged** | `COUNT(*) FROM meals` | All time |
| **Meals Today** | `COUNT(*) WHERE date = TODAY` | Daily |
| **Meals This Week** | `COUNT(*) WHERE date >= week_start` | Weekly |
| **Avg Meals/User/Day** | `meals / active_users` | Daily |
| **Meal Type Distribution** | Breakfast, Lunch, Dinner, Snack | Daily |

#### 4.2 Macro Adherence

```sql
-- Users hitting protein target (within 10%)
WITH user_macros AS (
    SELECT 
        m.user_id,
        SUM(mf.quantity * f.protein) as total_protein,
        SUM(mf.quantity * f.calories) as total_calories,
        u.goal,
        CASE 
            WHEN u.goal = 'build_muscle' THEN u.weight * 2.2
            WHEN u.goal = 'lose_fat' THEN u.weight * 2.4
            ELSE u.weight * 1.8
        END as target_protein,
        u.tdee as target_calories
    FROM meals m
    JOIN meal_foods mf ON m.id = mf.meal_id
    JOIN foods f ON mf.food_id = f.id
    JOIN users u ON m.user_id = u.id
    WHERE m.date = CURRENT_DATE
    GROUP BY m.user_id, u.goal, u.weight, u.tdee
)
SELECT 
    COUNT(*) FILTER (WHERE total_protein >= target_protein * 0.9 
                     AND total_protein <= target_protein * 1.1)::FLOAT / COUNT(*) * 100 
    as protein_adherence_rate,
    COUNT(*) FILTER (WHERE total_calories >= target_calories * 0.9 
                     AND total_calories <= target_calories * 1.1)::FLOAT / COUNT(*) * 100 
    as calorie_adherence_rate
FROM user_macros;
```

#### 4.3 Micronutrient Tracking

| KPI | Description |
|-----|-------------|
| **Users Tracking Micronutrients** | Count and % using detailed tracking |
| **Common Deficiencies** | Top 5 from `nutrient` table analysis |
| **Iron Deficiency Rate** | % users below iron target |
| **Vitamin D Deficiency Rate** | % users below vitamin D target |
| **Low Protein Warnings Sent** | From `notifications` table |

#### 4.4 Food Database Metrics

| KPI | Source |
|-----|--------|
| **Total Foods** | `COUNT(*) FROM foods WHERE deleted_at IS NULL` |
| **USDA Foods** | `COUNT(*) WHERE source = 'usda'` |
| **User-Created Foods** | `COUNT(*) WHERE source = 'user'` |
| **Most Logged Foods** | Top 20 from `meal_foods` |
| **Recipe Count** | `COUNT(*) FROM recipes` |
| **Recipe Usage** | Times recipes logged |

#### 4.5 Recipe Analytics (NEW!)

```sql
-- Most popular recipes
SELECT 
    r.name,
    COUNT(*) as times_logged,
    COUNT(DISTINCT m.user_id) as unique_users
FROM recipes r
JOIN meals m ON m.recipe_id = r.id
GROUP BY r.id, r.name
ORDER BY times_logged DESC
LIMIT 10;

-- Recipe complexity
SELECT 
    COUNT(recipe_id) as recipe_count,
    ingredient_count,
    COUNT(*) FILTER (WHERE ingredient_count BETWEEN 1 AND 5) as simple_recipes,
    COUNT(*) FILTER (WHERE ingredient_count BETWEEN 6 AND 10) as medium_recipes,
    COUNT(*) FILTER (WHERE ingredient_count > 10) as complex_recipes
FROM (SELECT recipe_id, COUNT(*) as ingredient_count FROM recipe_foods GROUP BY recipe_id) rf
GROUP BY ingredient_count;
```

#### 4.6 Favorite Foods Analysis (NEW!)

```sql
-- Most favorited foods
SELECT 
    f.name,
    COUNT(*) as favorite_count,
    COUNT(DISTINCT ff.user_id) as unique_users
FROM favorite_foods ff
JOIN foods f ON ff.food_id = f.id
GROUP BY f.id, f.name
ORDER BY favorite_count DESC
LIMIT 20;

-- Correlation: Users with favorites have better logging habits
WITH users_with_favorites AS (
    SELECT DISTINCT user_id FROM favorite_foods
),
meal_counts AS (
    SELECT user_id, COUNT(*) as meal_count
    FROM meals
    WHERE date >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY user_id
)
SELECT 
    CASE WHEN uwf.user_id IS NOT NULL THEN 'has_favorites' ELSE 'no_favorites' END as bucket,
    AVG(mc.meal_count) as avg_meals_per_month
FROM meal_counts mc
LEFT JOIN users_with_favorites uwf ON mc.user_id = uwf.user_id
GROUP BY bucket;
```

#### 4.7 Weight Tracking

| KPI | Calculation |
|-----|-------------|
| **Total Weight Entries** | `COUNT(*) FROM weight_entries` |
| **Users Weighing In (7d)** | Unique users in last 7 days |
| **Avg Weight Change** | Weekly/monthly average delta |
| **Weigh-in Streak Distribution** | Bucket users by streak length |

---

### 5. Goal Achievement & Adherence Dashboard

**Purpose**: Track user progress and behavior consistency

**KPIs**:

#### 5.1 Goal Distribution

```sql
SELECT 
    goal,
    COUNT(*) as user_count,
    ROUND(COUNT(*)::NUMERIC / SUM(COUNT(*)) OVER() * 100, 2) as percentage
FROM users
WHERE deleted_at IS NULL
GROUP BY goal
ORDER BY user_count DESC;
```

#### 5.2 Adherence by Goal Type

```sql
-- Average adherence by goal
WITH user_adherence AS (
    SELECT 
        u.id as user_id,
        u.goal,
        COUNT(DISTINCT activity_date)::FLOAT / 30 as adherence_30d
    FROM users u
    LEFT JOIN (
        SELECT user_id, date as activity_date FROM workouts
        UNION ALL
        SELECT user_id, date FROM meals
        UNION ALL
        SELECT user_id, date FROM weight_entries
    ) activities ON u.id = activities.user_id
    WHERE activities.activity_date >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY u.id, u.goal
)
SELECT 
    goal,
    ROUND(AVG(adherence_30d) * 100, 2) as avg_adherence_pct
FROM user_adherence
GROUP BY goal;
```

#### 5.3 Streak Metrics

| KPI | Description |
|-----|-------------|
| **Workout Streak Leaderboard** | Top 10 by consecutive weeks with workout |
| **Meal Streak Leaderboard** | Top 10 by consecutive days with meals |
| **Weigh-in Streak Leaderboard** | Top 10 by consecutive weigh-ins |
| **Longest Current Streaks** | Per category |
| **Avg Streak Length** | Per streak type |

**Implementation** (reuse existing `AdherenceService`):
```go
// Extend AdherenceService for platform-wide stats
func (s *AdherenceService) GetPlatformStreaks(ctx context.Context) (*PlatformStreaks, error) {
    // Query top streaks across all users
}
```

#### 5.4 Adherence Distribution

Bucket users by adherence percentage:
- 0-20% (Needs attention)
- 20-40% (Low engagement)
- 40-60% (Moderate)
- 60-80% (Good)
- 80-100% (Excellent)

#### 5.5 Activity Calendar Heatmap

Platform-wide activity visualization:
- Color intensity by activity level
- Filter by: workouts, meals, weight entries, all
- Exportable as CSV

---

### 6. Integration Intelligence Dashboard (NEW!)

**Purpose**: Track effectiveness of workout-nutrition recommendations

**Observation**: Your codebase has `services/integration_rules.go` which provides smart recommendations. This dashboard tracks their impact.

**KPIs**:

#### 6.1 Rule-Based Adjustments

| KPI | Source |
|-----|--------|
| **Calorie Adjustments Applied** | Count of workout-based calorie mods |
| **Leg Day Bonuses Given** | +200 calorie adjustments |
| **Rest Day Reductions** | -200 calorie adjustments |
| **Cardio Bonuses** | Additional calorie additions |
| **Recovery Warnings Sent** | High volume + low protein notifications |
| **Goal Alignment Warnings** | Users off-track from goal |

#### 6.2 Recommendation Effectiveness

```sql
-- Recommendations shown and followed
SELECT 
    n.type,
    COUNT(*) as recommendations_sent,
    COUNT(*) FILTER (WHERE read_at IS NOT NULL) as read_count,
    AVG(EXTRACT(EPOCH FROM (read_at - created_at))/3600) FILTER (WHERE read_at IS NOT NULL) as avg_hours_to_read
FROM notifications n
WHERE n.type IN ('low_protein_warning', 'recovery_warning', 'goal_alignment_warning')
  AND n.created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY n.type;
```

#### 6.3 Cross-Feature Correlation (NEW!)

**Question**: Do users who log both meals AND workouts have better retention?

```sql
-- Retention by feature combination
WITH user_feature_usage AS (
    SELECT 
        u.id,
        CASE WHEN workout_count > 0 THEN 1 ELSE 0 END as has_workouts,
        CASE WHEN meal_count > 0 THEN 1 ELSE 0 END as has_meals,
        CASE WHEN weight_count > 0 THEN 1 ELSE 0 END as has_weights
    FROM users u
    LEFT JOIN (SELECT user_id, COUNT(*) as workout_count FROM workouts GROUP BY user_id) w ON u.id = w.user_id
    LEFT JOIN (SELECT user_id, COUNT(*) as meal_count FROM meals GROUP BY user_id) m ON u.id = m.user_id
    LEFT JOIN (SELECT user_id, COUNT(*) as weight_count FROM weight_entries GROUP BY user_id) we ON u.id = we.user_id
    WHERE u.deleted_at IS NULL
),
retention AS (
    SELECT 
        user_id,
        CASE WHEN EXISTS(
            SELECT 1 FROM workouts WHERE user_id = u.id AND date >= u.created_at + INTERVAL '30 days'
        ) THEN 1 ELSE 0 END as retained_30d
    FROM users u
)
SELECT 
    CASE 
        WHEN has_workouts = 1 AND has_meals = 1 THEN 'both_workout_meal'
        WHEN has_workouts = 1 AND has_meals = 0 THEN 'workout_only'
        WHEN has_workouts = 0 AND has_meals = 1 THEN 'meal_only'
        ELSE 'neither'
    END as feature_combo,
    AVG(retained_30d) * 100 as retention_rate_30d
FROM user_feature_usage ufu
JOIN retention r ON ufu.id = r.user_id
GROUP BY feature_combo;
```

---

### 7. Content & Exercise Library Dashboard

**Purpose**: Manage content quality and usage

**KPIs**:

#### 7.1 Exercise Library

| KPI | Calculation |
|-----|-------------|
| **Total Exercises** | `COUNT(*) FROM exercises WHERE deleted_at IS NULL` |
| **Exercises by Muscle Group** | Distribution count |
| **Exercises by Equipment** | Distribution count |
| **Exercises with Videos** | `% WHERE video_url IS NOT NULL` |
| **Recently Added** | Count in last 30 days |
| **Most Used Exercises** | From `exercise_popularity` materialized view |

#### 7.2 Food Database

| KPI | Calculation |
|-----|-------------|
| **Total Foods** | `COUNT(*) FROM foods WHERE deleted_at IS NULL` |
| **USDA Imported** | Count where `source = 'usda'` |
| **User-Created** | Count where `source = 'user'` |
| **Complete Nutrition** | Foods with 19+ nutrients populated |
| **Missing Micronutrients** | Foods needing review |

#### 7.3 Programs & Templates

| KPI | Description |
|-----|-------------|
| **Active Templates** | Available for use |
| **Active Programs** | Multi-week programs |
| **Program Creators** | Admins creating programs |
| **Template vs Program Usage** | Adoption comparison |

---

### 8. System Health & Performance Dashboard

**Purpose**: Monitor infrastructure and API health

**KPIs**:

#### 8.1 API Performance (from Prometheus)

| Metric | Query | Target |
|--------|-------|--------|
| **Request Rate (RPS)** | `rate(fitness_http_requests_total[1m])` | Monitor trend |
| **Avg Response Time** | `rate(fitness_http_request_duration_seconds_sum[5m]) / rate(fitness_http_request_duration_seconds_count[5m])` | <200ms |
| **P95 Response Time** | `histogram_quantile(0.95, fitness_http_request_duration_seconds)` | <500ms |
| **P99 Response Time** | `histogram_quantile(0.99, fitness_http_request_duration_seconds)` | <1000ms |
| **Error Rate (%)** | `rate(fitness_http_requests_total{status=~"5.."}[1h]) / rate(fitness_http_requests_total[1h])` | <1% |
| **4xx Errors** | `rate(fitness_http_requests_total{status=~"4.."}[1h])` | Monitor |
| **5xx Errors** | `rate(fitness_http_requests_total{status=~"5.."}[1h])` | Alert if >0.5% |

#### 8.2 Database Metrics

| Metric | Query | Target |
|--------|-------|--------|
| **Active Connections** | `fitness_db_connections_in_use` | <80% pool |
| **Query Latency P95** | `histogram_quantile(0.95, fitness_db_query_duration_seconds)` | <100ms |
| **Slow Queries (>1s)** | Custom metric | Investigate |
| **Pool Saturation** | `db_connections_in_use / db_connections_max` | <80% |

#### 8.3 Business Metrics (Prometheus)

| Metric | Description |
|--------|-------------|
| `fitness_workouts_created_total` | Cumulative workouts |
| `fitness_meals_logged_total` | Cumulative meals |
| `fitness_weight_entries_logged_total` | Cumulative weight entries |
| `fitness_users_registered_total` | Cumulative registrations |

#### 8.4 Infrastructure

| Metric | Source |
|--------|--------|
| **CPU Usage (%)** | Prometheus node_exporter |
| **Memory Usage (%)** | Prometheus |
| **Disk I/O** | Read/write ops |
| **Network Traffic** | Inbound/outbound bytes |

#### 8.5 2FA & Security (NEW!)

| KPI | Calculation |
|-----|-------------|
| **2FA Adoption Rate** | `% users WHERE two_factor_enabled = true` |
| **2FA Setup Success Rate** | Completions / initiation attempts |
| **2FA Recovery Attempts** | From `two_factor_attempts` tracking |
| **Failed Login Attempts** | Suspicious login detection |
| **Active Sessions per User** | Average concurrent sessions |

```sql
-- 2FA Adoption
SELECT 
    COUNT(*) FILTER (WHERE two_factor_enabled = true) as users_with_2fa,
    COUNT(*) FILTER (WHERE two_factor_enabled = false) as users_without_2fa,
    ROUND(COUNT(*) FILTER (WHERE two_factor_enabled = true)::FLOAT / COUNT(*) * 100, 2) as adoption_rate
FROM users
WHERE deleted_at IS NULL;

-- Failed login attempts (last 24h)
SELECT COUNT(*) as failed_attempts
FROM auth_sessions
WHERE created_at >= NOW() - INTERVAL '24 hours'
  AND last_activity_at IS NULL;
```

---

### 9. Moderation, Safety & Compliance Dashboard

**Purpose**: Monitor platform safety and GDPR compliance

**KPIs**:

#### 9.1 User Safety

| KPI | Description |
|-----|-------------|
| **2FA Adoption Rate** | Security metric |
| **Active Sessions per User** | Average concurrent sessions |
| **Failed Login Attempts (24h)** | Potential brute force attacks |
| **Account Deletion Requests** | GDPR compliance tracking |

#### 9.2 Data Export Requests

| KPI | Description |
|-----|-------------|
| **Pending Exports** | Jobs in `processing` state |
| **Completed Exports (24h)** | Successful jobs |
| **Failed Exports** | Jobs with errors |
| **Avg Export Size** | MB per export |

```sql
-- Export job status
SELECT 
    status,
    COUNT(*) as job_count,
    AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) as avg_duration_seconds
FROM export_jobs
WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY status;
```

#### 9.3 Notification Delivery

| KPI | Description |
|-----|-------------|
| **Notifications Sent (24h)** | Total created |
| **Notifications by Type** | Distribution |
| **Unread Rate** | % not yet read |
| **Avg Time to Read** | Engagement metric |

```sql
-- Notification metrics
SELECT 
    type,
    COUNT(*) as sent,
    COUNT(*) FILTER (WHERE read_at IS NOT NULL) as read,
    AVG(EXTRACT(EPOCH FROM (read_at - created_at))/3600) FILTER (WHERE read_at IS NOT NULL) as avg_hours_to_read
FROM notifications
WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY type;
```

#### 9.4 Audit Trail (NEW!)

**Requirement**: All admin actions must be logged for accountability

```go
// New model: models/audit_log.go
type AuditLog struct {
    ID         uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
    AdminID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"admin_id"`
    Action     string          `gorm:"type:varchar(100);not null" json:"action"` // create_user, delete_user, etc.
    EntityType string          `gorm:"type:varchar(100);not null" json:"entity_type"` // user, exercise, etc.
    EntityID   uuid.UUID       `gorm:"type:uuid" json:"entity_id"`
    OldValue   json.RawMessage `gorm:"type:jsonb" json:"old_value"`
    NewValue   json.RawMessage `gorm:"type:jsonb" json:"new_value"`
    IPAddress  string          `gorm:"type:varchar(45)" json:"ip_address"`
    UserAgent  string          `gorm:"type:varchar(255)" json:"user_agent"`
    CreatedAt  time.Time       `json:"created_at"`
}
```

---

### 10. USDA Import & Food Management Dashboard (NEW!)

**Purpose**: Track food import operations and quality

**KPIs**:

#### 10.1 Import Metrics

| KPI | Description |
|-----|-------------|
| **Imports Attempted** | Total USDA import operations |
| **Imports Successful** | Successfully imported foods |
| **Imports Failed** | Failed operations |
| **Last Import Timestamp** | Most recent import |
| **Foods with Complete Nutrition** | 19+ nutrients populated |
| **Foods Needing Review** | Missing critical fields |

#### 10.2 Import Log

```sql
-- Food import log (add table)
CREATE TABLE food_import_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id UUID NOT NULL REFERENCES users(id),
    source VARCHAR(50) NOT NULL DEFAULT 'usda',
    fdc_id INT,
    status VARCHAR(20) NOT NULL, -- success, failed, duplicate
    error_message TEXT,
    foods_imported INT DEFAULT 0,
    duration_ms INT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

---

## Additional Enhancements (Not in Either Plan)

### Seasonal Pattern Recognition

Fitness apps see significant seasonality:
- **New Year's Resolutions**: Jan spike
- **Summer Prep**: May-June spike
- **Holiday Dip**: Nov-Dec decline

```sql
-- Year-over-year comparison
WITH yearly_stats AS (
    SELECT 
        EXTRACT(YEAR FROM date) as year,
        EXTRACT(WEEK FROM date) as week,
        COUNT(*) as workout_count
    FROM workouts
    WHERE deleted_at IS NULL
    GROUP BY year, week
)
SELECT 
    week,
    year,
    workout_count,
    LAG(workout_count) OVER (PARTITION BY week ORDER BY year) as prev_year_week
FROM yearly_stats
ORDER BY week, year;
```

---

## API Endpoints

### Admin Dashboard Endpoints

```
# Executive Summary
GET /v1/admin/dashboard/summary
GET /v1/admin/dashboard/trends

# User Analytics
GET /v1/admin/users/stats
GET /v1/admin/users/growth
GET /v1/admin/users/retention
GET /v1/admin/users/funnel
GET /v1/admin/users/cohort-analysis
GET /v1/admin/users/sessions
GET /v1/admin/users/2fa-metrics

# Workout Analytics
GET /v1/admin/workouts/stats
GET /v1/admin/workouts/volume
GET /v1/admin/workouts/exercises/popular
GET /v1/admin/workouts/personal-records
GET /v1/admin/workouts/templates/usage
GET /v1/admin/workouts/programs/usage
GET /v1/admin/workouts/cardio/metrics

# Nutrition Analytics
GET /v1/admin/nutrition/stats
GET /v1/admin/nutrition/macros/adherence
GET /v1/admin/nutrition/micronutrients/deficiencies
GET /v1/admin/nutrition/foods/popular
GET /v1/admin/nutrition/recipes/stats
GET /v1/admin/nutrition/favorites/analysis

# Goal & Adherence
GET /v1/admin/adherence/streaks
GET /v1/admin/adherence/distribution
GET /v1/admin/adherence/calendar
GET /v1/admin/goals/distribution
GET /v1/admin/goals/progress

# Integration Intelligence
GET /v1/admin/integration/rules-effectiveness
GET /v1/admin/integration/cross-feature-correlation

# Content Management
GET /v1/admin/content/exercises
GET /v1/admin/content/foods
GET /v1/admin/content/programs
POST /v1/admin/content/exercises
POST /v1/admin/content/foods/import-usda

# System Health
GET /v1/admin/system/health
GET /v1/admin/system/metrics
GET /v1/admin/system/errors

# Moderation & Compliance
GET /v1/admin/moderation/exports
GET /v1/admin/moderation/notifications
GET /v1/admin/audit-logs
GET /v1/admin/users/{id}

# WebSocket
WS /v1/admin/dashboard/realtime
```

---

## Frontend Component Structure

```
frontend/src/
├── app/
│   ├── admin/
│   │   ├── layout.tsx                 # Admin layout with sidebar
│   │   ├── page.tsx                   # Executive summary
│   │   ├── users/
│   │   │   ├── page.tsx               # User overview
│   │   │   ├── growth.tsx              # Growth charts
│   │   │   ├── retention.tsx           # Retention funnel
│   │   │   ├── cohorts.tsx             # Cohort heatmap
│   │   │   └── sessions.tsx            # Session analytics
│   │   ├── workouts/
│   │   │   ├── page.tsx                # Workout overview
│   │   │   ├── volume.tsx              # Volume charts
│   │   │   ├── exercises.tsx           # Exercise rankings
│   │   │   ├── cardio.tsx              # Cardio metrics
│   │   │   ├── programs.tsx            # Program usage
│   │   │   └── personal-records.tsx    # PR leaderboard
│   │   ├── nutrition/
│   │   │   ├── page.tsx                # Nutrition overview
│   │   │   ├── macros.tsx               # Macro adherence
│   │   │   ├── foods.tsx                # Food database
│   │   │   ├── recipes.tsx              # Recipe analytics
│   │   │   └── micronutrients.tsx      # Micronutrient tracking
│   │   ├── adherence/
│   │   │   ├── page.tsx                 # Streak overview
│   │   │   ├── calendar.tsx             # Activity heatmap
│   │   │   └── goals.tsx                # Goal progress
│   │   ├── content/
│   │   │   ├── exercises.tsx            # Exercise management
│   │   │   ├── foods.tsx                # Food management
│   │   │   └── programs.tsx             # Program management
│   │   ├── system/
│   │   │   ├── page.tsx                 # System health
│   │   │   ├── metrics.tsx              # Prometheus metrics
│   │   │   ├── errors.tsx               # Error logs
│   │   │   └── security.tsx              # 2FA & security
│   │   ├── moderation/
│   │   │   ├── page.tsx                 # Moderation overview
│   │   │   ├── exports.tsx               # Export jobs
│   │   │   └── audit-logs.tsx           # Audit trail
│   │   └── settings/
│   │       └── page.tsx                  # Admin settings
│   └── layout.tsx
├── components/
│   ├── dashboard/
│   │   ├── DashboardLayout.tsx
│   │   ├── KPICard.tsx
│   │   ├── ChartWidget.tsx
│   │   ├── RealtimeCard.tsx
│   │   └── Sparkline.tsx
│   ├── charts/
│   │   ├── LineChart.tsx
│   │   ├── BarChart.tsx
│   │   ├── PieChart.tsx
│   │   ├── HeatmapChart.tsx
│   │   ├── FunnelChart.tsx
│   │   └── CohortHeatmap.tsx
│   └── tables/
│       ├── DataTable.tsx
│       ├── UsersTable.tsx
│       └── AuditLogTable.tsx
├── hooks/
│   ├── useWebSocket.ts
│   ├── useAdminData.ts
│   ├── usePolling.ts
│   └── useChartData.ts
├── api/
│   └── admin.ts
└── types/
    └── admin.ts
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)
- [ ] Create admin dashboard layout and routing
- [ ] Implement executive summary endpoints
- [ ] Add WebSocket support for real-time metrics
- [ ] Create materialized views for daily stats
- [ ] Set up Redis caching layer
- [ ] Build KPI cards with real-time indicators

### Phase 2: User Analytics (Week 3-4)
- [ ] User growth and retention endpoints
- [ ] Engagement funnel visualization
- [ ] Cohort analysis heatmap
- [ ] Session analytics metrics
- [ ] 2FA adoption tracking
- [ ] Demographics breakdown

### Phase 3: Workout & Nutrition (Week 5-6)
- [ ] Workout volume and stats endpoints
- [ ] Exercise popularity rankings
- [ ] Personal records leaderboard
- [ ] Nutrition adherence metrics
- [ ] Food database management
- [ ] Recipe analytics

### Phase 4: Advanced Features (Week 7-8)
- [ ] Streak and adherence dashboards
- [ ] Activity calendar heatmap
- [ ] Integration intelligence metrics
- [ ] Cross-feature correlation analysis
- [ ] Content management interfaces

### Phase 5: System & Security (Week 9-10)
- [ ] System health dashboard
- [ ] Prometheus metrics integration
- [ ] Security & 2FA analytics
- [ ] Export job monitoring
- [ ] USDA import tracking
- [ ] Audit trail logging

### Phase 6: Polish & Optimization (Week 11-12)
- [ ] Performance optimization (query tuning, caching)
- [ ] Export functionality (PDF/CSV reports)
- [ ] Mobile-responsive design
- [ ] Loading states and error handling
- [ ] End-to-end testing
- [ ] Documentation

---

## Security Considerations

1. **Admin-Only Access**: All endpoints protected with `RequireAdmin` middleware
2. **Rate Limiting**: Stricter limits for admin dashboard (100 req/min)
3. **Audit Logging**: Every admin action logged to `audit_logs` table
4. **Session Management**: Short-lived sessions with re-authentication required
5. **IP Whitelisting**: Optional IP restriction for admin endpoints
6. **GDPR Compliance**: Ensure user data exported follows GDPR guidelines

---

## Performance Targets

| Metric | Target |
|--------|--------|
| Dashboard page load time | < 500ms (cached) |
| API response time (P95) | < 200ms |
| Real-time update latency | < 1s |
| WebSocket connection capacity | 10k concurrent admins |
| Query response time (complex aggregations) | < 100ms (materialized views) |
| Redis cache hit rate | > 95% |

---

## Testing Strategy

1. **Unit Tests**: Service layer KPI calculations (90% coverage)
2. **Integration Tests**: API endpoint responses with mocked DB
3. **Load Tests**: Dashboard query performance with 10M+ records
4. **E2E Tests**: Frontend dashboard flows with Playwright
5. **Performance Tests**: Query optimization with EXPLAIN ANALYZE

---

## Key Performance Indicators Summary

### Primary KPIs (Executive Level)

| KPI | Target | Frequency | Owner |
|-----|--------|-----------|-------|
| **MAU** | Growing | Monthly | Product |
| **DAU/MAU Ratio** | >20% | Daily | Product |
| **30-Day Retention** | >15% | Monthly | Product |
| **7-Day Retention** | >40% | Weekly | Product |
| **Error Rate** | <1% | Real-time | DevOps |
| **API P95 Latency** | <500ms | Real-time | DevOps |

### Secondary KPIs (Operational Level)

| KPI | Target | Frequency | Owner |
|-----|--------|-----------|-------|
| **Workouts/User/Week** | >2 | Weekly | Fitness |
| **Meals/User/Day** | >2 | Daily | Nutrition |
| **Macro Adherence Rate** | >60% | Weekly | Nutrition |
| **2FA Adoption** | >30% | Monthly | Security |
| **Session Duration** | >5min | Daily | Product |

---

## Conclusion

This comprehensive admin dashboard plan provides complete visibility into the UM6P_FIT platform by merging the strengths of both previous plans and addressing critical gaps:

### From Your Plan:
- Real-time WebSocket architecture with detailed implementation
- Database optimization strategies (materialized views)
- Redis caching layer
- Security considerations and audit trail

### From Friend's Plan:
- User funnel analysis
- Cohort analysis
- Moderation & safety dashboard
- Integration intelligence (workout-nutrition rules)

### New Additions:
- **2FA & Session Analytics** (leveraging `two_factor.go` and `auth_sessions.go`)
- **Recipe Analytics** (leveraging `recipes` table)
- **Favorite Foods Analysis** (correlation with engagement)
- **Cross-Feature Correlation** (meal+workout users vs retention)
- **Export Job Monitoring** (leveraging `export_jobs` table)
- **USDA Import Metrics** (new `food_import_logs` table)
- **Comprehensive Audit Trail** (new `audit_logs` table)
- **Seasonal Pattern Recognition** (year-over-year comparisons)

The dashboard will serve administrators with actionable insights, real-time metrics, and comprehensive monitoring across all platform aspects, enabling data-driven decisions and proactive platform management.