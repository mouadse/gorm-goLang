package worker

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/services"

	"gorm.io/gorm"
)

type Runner struct {
	db                       *gorm.DB
	exportPollInterval       time.Duration
	adminRefreshInterval     time.Duration
	notificationPollInterval time.Duration
	exportSvc                *services.ExportService
	notificationSvc          *services.NotificationAutomationService
}

func New(db *gorm.DB) *Runner {
	return &Runner{
		db:                       db,
		exportPollInterval:       getDurationEnv("WORKER_EXPORT_POLL_INTERVAL", 15*time.Second),
		adminRefreshInterval:     getDurationEnv("WORKER_ADMIN_REFRESH_INTERVAL", time.Hour),
		notificationPollInterval: getDurationEnv("WORKER_NOTIFICATION_POLL_INTERVAL", time.Hour),
		exportSvc:                services.NewExportService(db),
		notificationSvc:          services.NewNotificationAutomationService(db),
	}
}

func (r *Runner) Run(ctx context.Context) error {
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
			if err := r.processPendingExports(ctx); err != nil {
				log.Printf("pending export processing failed: %v", err)
			}
		case <-adminTicker.C:
			if err := r.refreshAdminViews(); err != nil {
				log.Printf("admin view refresh failed: %v", err)
			}
		case <-notificationTicker.C:
			if err := r.processDueNotifications(ctx); err != nil {
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
