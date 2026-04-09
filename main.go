package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fitness-tracker/api"
	"fitness-tracker/database"
	"fitness-tracker/worker"
)

const (
	defaultListenAddr        = ":8080"
	defaultShutdownTimeout   = 15 * time.Second
	defaultReadTimeout       = 15 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultMaxHeaderBytes    = 1 << 20
)

func main() {
	mode := runtimeMode(os.Args[1:], os.Getenv("APP_MODE"))

	var err error
	switch mode {
	case "api":
		err = runAPI()
	case "migrate":
		err = runMigrations()
	case "worker":
		err = runWorker()
	default:
		err = fmt.Errorf("unsupported mode %q; expected api, migrate, or worker", mode)
	}

	if err != nil {
		log.Fatalf("%s mode failed: %v", mode, err)
	}
}

func runAPI() error {
	db, err := database.Connect()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	if err := api.ValidateJWTConfig(); err != nil {
		return fmt.Errorf("auth configuration invalid: %w", err)
	}

	server := api.NewServer(db)
	httpServer := &http.Server{
		Addr:              listenAddr(),
		Handler:           server.Handler(),
		ReadTimeout:       getDurationEnv("HTTP_READ_TIMEOUT", defaultReadTimeout),
		ReadHeaderTimeout: getDurationEnv("HTTP_READ_HEADER_TIMEOUT", defaultReadHeaderTimeout),
		WriteTimeout:      getDurationEnv("HTTP_WRITE_TIMEOUT", defaultWriteTimeout),
		IdleTimeout:       getDurationEnv("HTTP_IDLE_TIMEOUT", defaultIdleTimeout),
		MaxHeaderBytes:    getIntEnv("HTTP_MAX_HEADER_BYTES", defaultMaxHeaderBytes),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout))
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("fitness-tracker API listening on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runMigrations() error {
	db, err := database.Connect()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	log.Println("database migrations completed")
	return nil
}

func runWorker() error {
	db, err := database.Connect()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("fitness-tracker worker started")
	return worker.New(db).Run(ctx)
}

func runtimeMode(args []string, envMode string) string {
	if len(args) > 0 {
		return strings.ToLower(strings.TrimSpace(args[0]))
	}
	if envMode != "" {
		return strings.ToLower(strings.TrimSpace(envMode))
	}
	return "api"
}

func listenAddr() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return defaultListenAddr
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	return ":" + port
}

func getIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
