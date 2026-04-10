package worker

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/metrics"
	"fitness-tracker/services"

	"gorm.io/gorm"
)

type Runner struct {
	db                       *gorm.DB
	m                        *metrics.Metrics
	exportPollInterval       time.Duration
	adminRefreshInterval     time.Duration
	notificationPollInterval time.Duration
	exportSvc                *services.ExportService
	notificationSvc          *services.NotificationAutomationService
}

func New(db *gorm.DB) *Runner {
	m := metrics.New()
	return &Runner{
		db:                       db,
		m:                        m,
		exportPollInterval:       getDurationEnv("WORKER_EXPORT_POLL_INTERVAL", 15*time.Second),
		adminRefreshInterval:     getDurationEnv("WORKER_ADMIN_REFRESH_INTERVAL", time.Hour),
		notificationPollInterval: getDurationEnv("WORKER_NOTIFICATION_POLL_INTERVAL", time.Hour),
		exportSvc:                services.NewExportService(db, m),
		notificationSvc:          services.NewNotificationAutomationServiceWithMetrics(db, m),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	// Start metrics server for worker
	metricsPort := strings.TrimSpace(os.Getenv("WORKER_METRICS_PORT"))
	if metricsPort == "" {
		metricsPort = "9091"
	}
	metrics.StartMetricsServer(":"+metricsPort, r.m)

	// Setup GORM metrics for worker DB queries
	if err := metrics.NewGORMCallbackPlugin(r.m).Initialize(r.db); err != nil {
		log.Printf("warning: failed to register GORM metrics plugin for worker: %v", err)
	}
	if sqlDB, err := r.db.DB(); err == nil {
		connTicker := time.NewTicker(10 * time.Second)
		defer connTicker.Stop()
		go r.m.TrackDBConnStats(sqlDB, connTicker.C)
	}

	if err := r.refreshAdminViews(); err != nil {
		log.Printf("initial admin view refresh failed: %v", err)
	}
	if err := r.processPendingExports(ctx); err != nil {
		log.Printf("initial export processing failed: %v", err)
	}
	if err := r.processDueNotifications(ctx); err != nil {
		log.Printf("initial notification processing failed: %v", err)
	}

	exportTicker := time.NewTicker(r.exportPollInterval)
	defer exportTicker.Stop()

	adminTicker := time.NewTicker(r.adminRefreshInterval)
	defer adminTicker.Stop()

	notificationTicker := time.NewTicker(r.notificationPollInterval)
	defer notificationTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-exportTicker.C:
			r.m.WorkerPollCycles.WithLabelValues("export").Inc()
			if err := r.processPendingExports(ctx); err != nil {
				r.m.WorkerPollErrors.WithLabelValues("export").Inc()
				log.Printf("pending export processing failed: %v", err)
			}
		case <-adminTicker.C:
			r.m.WorkerPollCycles.WithLabelValues("admin_refresh").Inc()
			if err := r.refreshAdminViews(); err != nil {
				r.m.WorkerPollErrors.WithLabelValues("admin_refresh").Inc()
				log.Printf("admin view refresh failed: %v", err)
			}
		case <-notificationTicker.C:
			r.m.WorkerPollCycles.WithLabelValues("notification").Inc()
			if err := r.processDueNotifications(ctx); err != nil {
				r.m.WorkerPollErrors.WithLabelValues("notification").Inc()
				log.Printf("notification processing failed: %v", err)
			}
		}
	}
}

func (r *Runner) processPendingExports(ctx context.Context) error {
	jobs, err := r.exportSvc.ListPendingJobs(10)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := r.exportSvc.ProcessExportJob(job.ID); err != nil {
			log.Printf("export job %s failed: %v", job.ID, err)
		}
	}

	return nil
}

func (r *Runner) refreshAdminViews() error {
	if err := database.RefreshAdminViews(r.db); err != nil {
		return err
	}
	log.Println("admin materialized views refreshed")
	return nil
}

func (r *Runner) processDueNotifications(ctx context.Context) error {
	created, err := r.notificationSvc.ProcessDueNotifications(ctx, time.Now().UTC())
	if err != nil {
		return err
	}
	if created > 0 {
		log.Printf("created %d automated notifications", created)
	}
	return nil
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}