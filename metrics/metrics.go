package metrics

import (
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
	DBQueriesTotal     *prometheus.CounterVec
	DBQueryDuration    *prometheus.HistogramVec
	DBConnectionsInUse prometheus.Gauge

	// Business metrics
	WorkoutsCreated     prometheus.Counter
	MealsLogged         prometheus.Counter
	WeightEntriesLogged prometheus.Counter
	UsersRegistered     prometheus.Counter
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
		m.WorkoutsCreated,
		m.MealsLogged,
		m.WeightEntriesLogged,
		m.UsersRegistered,
	)

	return m
}

// Registry returns the Prometheus registry for use with promhttp.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}
