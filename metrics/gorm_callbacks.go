package metrics

import (
	"time"

	"gorm.io/gorm"
)

// GORMCallbackPlugin registers Prometheus metrics as GORM callbacks.
// It tracks query count, duration and errors per operation/table.
type GORMCallbackPlugin struct {
	metrics *Metrics
}

// NewGORMCallbackPlugin creates a new GORM callback plugin for the given metrics.
func NewGORMCallbackPlugin(m *Metrics) *GORMCallbackPlugin {
	return &GORMCallbackPlugin{metrics: m}
}

// Name implements gorm.Plugin interface.
func (p *GORMCallbackPlugin) Name() string {
	return "prometheus_metrics"
}

// Install implements gorm.Plugin interface — registers before/after callbacks
// for all CRUD operations.
func (p *GORMCallbackPlugin) Initialize(db *gorm.DB) error {
	callback := db.Callback()

	// Track create operations
	if err := callback.Create().Before("gorm:create").Register("metrics:before_create", p.before("create")); err != nil {
		return err
	}
	if err := callback.Create().After("gorm:create").Register("metrics:after_create", p.after("create")); err != nil {
		return err
	}

	// Track query operations
	if err := callback.Query().Before("gorm:query").Register("metrics:before_query", p.before("query")); err != nil {
		return err
	}
	if err := callback.Query().After("gorm:query").Register("metrics:after_query", p.after("query")); err != nil {
		return err
	}

	// Track update operations
	if err := callback.Update().Before("gorm:update").Register("metrics:before_update", p.before("update")); err != nil {
		return err
	}
	if err := callback.Update().After("gorm:update").Register("metrics:after_update", p.after("update")); err != nil {
		return err
	}

	// Track delete operations
	if err := callback.Delete().Before("gorm:delete").Register("metrics:before_delete", p.before("delete")); err != nil {
		return err
	}
	if err := callback.Delete().After("gorm:delete").Register("metrics:after_delete", p.after("delete")); err != nil {
		return err
	}

	// Track row operations (used by GORM for raw SQL)
	if err := callback.Row().Before("gorm:row").Register("metrics:before_row", p.before("row")); err != nil {
		return err
	}
	if err := callback.Row().After("gorm:row").Register("metrics:after_row", p.after("row")); err != nil {
		return err
	}

	return nil
}

const startTimeKey = "metrics:start_time"

func (p *GORMCallbackPlugin) before(operation string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		db.InstanceSet(startTimeKey, time.Now())
		_ = operation // captured by closure in after()
	}
}

func (p *GORMCallbackPlugin) after(operation string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		startVal, ok := db.InstanceGet(startTimeKey)
		if !ok {
			return
		}
		start, ok := startVal.(time.Time)
		if !ok {
			return
		}
		duration := time.Since(start).Seconds()
		table := db.Statement.Table
		if table == "" {
			table = "unknown"
		}

		p.metrics.DBQueriesTotal.WithLabelValues(operation, table).Inc()
		p.metrics.DBQueryDuration.WithLabelValues(operation, table).Observe(duration)
	}
}