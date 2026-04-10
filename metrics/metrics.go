package metrics

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds all Prometheus metrics for the fitness-tracker application.
type Metrics struct {
	registry *prometheus.Registry

	// HTTP metrics
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlight        prometheus.Gauge
	ResponseSize    *prometheus.HistogramVec

	// Database metrics
	DBQueriesTotal       *prometheus.CounterVec
	DBQueryDuration      *prometheus.HistogramVec
	DBConnectionsInUse   prometheus.Gauge
	DBConnectionsIdle    prometheus.Gauge
	DBConnectionsMaxOpen prometheus.Gauge
	DBWaitCount          prometheus.Desc
	DBWaitDuration       prometheus.Desc

	// Business metrics
	WorkoutsCreated     prometheus.Counter
	MealsLogged         prometheus.Counter
	WeightEntriesLogged prometheus.Counter
	UsersRegistered     prometheus.Counter

	// Auth metrics
	AuthAttemptsTotal  *prometheus.CounterVec
	AuthTokenRefreshes prometheus.Counter
	TwoFactorActions   *prometheus.CounterVec
	ActiveSessions     prometheus.Gauge

	// Chat / AI Coach metrics
	ChatMessagesTotal    prometheus.Counter
	CoachRequestsTotal   *prometheus.CounterVec
	CoachRequestDuration *prometheus.HistogramVec
	CoachTokensUsed      *prometheus.CounterVec

	// Export metrics
	ExportJobsCreated   *prometheus.CounterVec
	ExportJobsCompleted *prometheus.CounterVec
	ExportJobsFailed    *prometheus.CounterVec
	ExportDuration      *prometheus.HistogramVec

	// Worker metrics
	WorkerPollCycles *prometheus.CounterVec
	WorkerPollErrors *prometheus.CounterVec

	// Notification metrics
	NotificationsCreated *prometheus.CounterVec
	NotificationsSent    prometheus.Counter

	// External service metrics
	ExtServiceRequests *prometheus.CounterVec
	ExtServiceDuration *prometheus.HistogramVec
	ExtServiceErrors   *prometheus.CounterVec
}

// New creates a new Metrics instance with all metrics registered.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	// Register standard Go and Process collectors
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewBuildInfoCollector(),
	)

	m := &Metrics{
		registry: reg,

		// ── HTTP ──
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		InFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fitness_http_in_flight_requests",
			Help: "Current number of HTTP requests being served",
		}),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path"},
		),

		// ── Database ──
		DBQueriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_db_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"operation", "table"},
		),
		DBQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"operation", "table"},
		),
		DBConnectionsInUse: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fitness_db_connections_in_use",
			Help: "Number of database connections currently in use",
		}),
		DBConnectionsIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fitness_db_connections_idle",
			Help: "Number of idle database connections",
		}),
		DBConnectionsMaxOpen: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fitness_db_connections_max_open",
			Help: "Maximum number of open database connections",
		}),

		// ── Business ──
		WorkoutsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_workouts_created_total",
			Help: "Total number of workouts created",
		}),
		MealsLogged: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_meals_logged_total",
			Help: "Total number of meals logged",
		}),
		WeightEntriesLogged: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_weight_entries_logged_total",
			Help: "Total number of weight entries logged",
		}),
		UsersRegistered: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_users_registered_total",
			Help: "Total number of users registered",
		}),

		// ── Auth ──
		AuthAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_auth_attempts_total",
				Help: "Total authentication attempts",
			},
			[]string{"method", "result"},
		),
		AuthTokenRefreshes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_auth_token_refreshes_total",
			Help: "Total number of token refreshes",
		}),
		TwoFactorActions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_2fa_actions_total",
				Help: "Two-factor authentication actions",
			},
			[]string{"action"},
		),
		ActiveSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "fitness_active_sessions",
			Help: "Number of currently active sessions",
		}),

		// ── Chat / AI Coach ──
		ChatMessagesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_chat_messages_total",
			Help: "Total number of chat messages sent",
		}),
		CoachRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_coach_requests_total",
				Help: "Total AI coach requests",
			},
			[]string{"result"},
		),
		CoachRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_coach_request_duration_seconds",
				Help:    "AI coach request duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.5, 2, 10),
			},
			[]string{"model"},
		),
		CoachTokensUsed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_coach_tokens_used_total",
				Help: "Total tokens consumed by AI coach",
			},
			[]string{"direction"},
		),

		// ── Exports ──
		ExportJobsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_export_jobs_created_total",
				Help: "Total export jobs created",
			},
			[]string{"format"},
		),
		ExportJobsCompleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_export_jobs_completed_total",
				Help: "Total export jobs completed",
			},
			[]string{"format"},
		),
		ExportJobsFailed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_export_jobs_failed_total",
				Help: "Total export jobs failed",
			},
			[]string{"format"},
		),
		ExportDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_export_duration_seconds",
				Help:    "Export job processing duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
			},
			[]string{"format"},
		),

		// ── Worker ──
		WorkerPollCycles: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_worker_poll_cycles_total",
				Help: "Total number of worker poll cycles",
			},
			[]string{"task_type"},
		),
		WorkerPollErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_worker_poll_errors_total",
				Help: "Total number of worker poll errors",
			},
			[]string{"task_type"},
		),

		// ── Notifications ──
		NotificationsCreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_notifications_created_total",
				Help: "Total notifications created",
			},
			[]string{"type"},
		),
		NotificationsSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "fitness_notifications_sent_total",
			Help: "Total notifications sent or delivered",
		}),

		// ── External services ──
		ExtServiceRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_ext_service_requests_total",
				Help: "Total requests to external services",
			},
			[]string{"service", "method", "status"},
		),
		ExtServiceDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "fitness_ext_service_duration_seconds",
				Help:    "External service request duration in seconds",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
			},
			[]string{"service"},
		),
		ExtServiceErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "fitness_ext_service_errors_total",
				Help: "Total errors from external services",
			},
			[]string{"service", "error_type"},
		),
	}

	// Register all metrics
	reg.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.InFlight,
		m.ResponseSize,
		m.DBQueriesTotal,
		m.DBQueryDuration,
		m.DBConnectionsInUse,
		m.DBConnectionsIdle,
		m.DBConnectionsMaxOpen,
		m.WorkoutsCreated,
		m.MealsLogged,
		m.WeightEntriesLogged,
		m.UsersRegistered,
		m.AuthAttemptsTotal,
		m.AuthTokenRefreshes,
		m.TwoFactorActions,
		m.ActiveSessions,
		m.ChatMessagesTotal,
		m.CoachRequestsTotal,
		m.CoachRequestDuration,
		m.CoachTokensUsed,
		m.ExportJobsCreated,
		m.ExportJobsCompleted,
		m.ExportJobsFailed,
		m.ExportDuration,
		m.WorkerPollCycles,
		m.WorkerPollErrors,
		m.NotificationsCreated,
		m.NotificationsSent,
		m.ExtServiceRequests,
		m.ExtServiceDuration,
		m.ExtServiceErrors,
	)

	m.initializeDashboardSeries()

	return m
}

func (m *Metrics) initializeDashboardSeries() {
	for _, operation := range []string{"create", "query", "update", "delete", "row"} {
		m.DBQueriesTotal.WithLabelValues(operation, "none")
		m.DBQueryDuration.WithLabelValues(operation, "none")
	}

	for _, result := range []string{"success", "failure"} {
		m.AuthAttemptsTotal.WithLabelValues("login", result)
		m.AuthAttemptsTotal.WithLabelValues("register", result)
	}

	for _, result := range []string{"success", "error"} {
		m.CoachRequestsTotal.WithLabelValues(result)
	}

	for _, format := range []string{"json", "csv"} {
		m.ExportJobsCreated.WithLabelValues(format)
		m.ExportJobsCompleted.WithLabelValues(format)
		m.ExportJobsFailed.WithLabelValues(format)
	}

	for _, taskType := range []string{"export", "admin_refresh", "notification"} {
		m.WorkerPollCycles.WithLabelValues(taskType)
		m.WorkerPollErrors.WithLabelValues(taskType)
	}

	for _, notificationType := range []string{
		"low_protein_warning",
		"missed_meal_logging",
		"workout_reminder",
		"rest_day_warning",
		"export_ready",
		"recovery_warning",
		"goal_alignment_warning",
	} {
		m.NotificationsCreated.WithLabelValues(notificationType)
	}
}

// Registry returns the Prometheus registry for use with promhttp.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// TrackDBConnStats periodically updates DB connection pool gauges.
// Call this in a goroutine: go m.TrackDBConnStats(sqlDB, 10*time.Second)
func (m *Metrics) TrackDBConnStats(sqlDB *sql.DB, interval <-chan time.Time) {
	for range interval {
		stats := sqlDB.Stats()
		m.DBConnectionsInUse.Set(float64(stats.InUse))
		m.DBConnectionsIdle.Set(float64(stats.Idle))
		m.DBConnectionsMaxOpen.Set(float64(stats.MaxOpenConnections))
	}
}
