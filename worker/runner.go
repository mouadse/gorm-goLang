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
	db                   *gorm.DB
	exportPollInterval   time.Duration
	adminRefreshInterval time.Duration
	exportSvc            *services.ExportService
}

func New(db *gorm.DB) *Runner {
	return &Runner{
		db:                   db,
		exportPollInterval:   getDurationEnv("WORKER_EXPORT_POLL_INTERVAL", 15*time.Second),
		adminRefreshInterval: getDurationEnv("WORKER_ADMIN_REFRESH_INTERVAL", time.Hour),
		exportSvc:            services.NewExportService(db),
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.refreshAdminViews(); err != nil {
		log.Printf("initial admin view refresh failed: %v", err)
	}
	if err := r.processPendingExports(ctx); err != nil {
		log.Printf("initial export processing failed: %v", err)
	}

	exportTicker := time.NewTicker(r.exportPollInterval)
	defer exportTicker.Stop()

	adminTicker := time.NewTicker(r.adminRefreshInterval)
	defer adminTicker.Stop()

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
