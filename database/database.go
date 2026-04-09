package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var loadEnvOnce sync.Once

// Connect opens a connection to the PostgreSQL database using the DATABASE_URL
// environment variable, or falls back to individual PG* variables.
// Returns a configured *gorm.DB instance or an error.
func Connect() (*gorm.DB, error) {
	loadEnvOnce.Do(func() {
		_ = godotenv.Load()
	})

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Build DSN from individual environment variables
		host := getEnvOrDefault("PGHOST", "localhost")
		port := getEnvOrDefault("PGPORT", "5433")
		user := getEnvOrDefault("PGUSER", "postgres")
		password := getEnvOrDefault("PGPASSWORD", "postgres")
		dbname := getEnvOrDefault("PGDATABASE", "fitness_tracker")
		sslmode := getEnvOrDefault("PGSSLMODE", "disable")

		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode,
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(configuredLogLevel()),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access sql db: %w", err)
	}
	sqlDB.SetMaxOpenConns(getIntEnv("DB_MAX_OPEN_CONNS", 25))
	sqlDB.SetMaxIdleConns(getIntEnv("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(getDurationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute))
	sqlDB.SetConnMaxIdleTime(getDurationEnv("DB_CONN_MAX_IDLE_TIME", 5*time.Minute))

	log.Println("✅ Database connection established")
	return db, nil
}

func getEnvOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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

func configuredLogLevel() logger.LogLevel {
	level := strings.ToLower(strings.TrimSpace(os.Getenv("GORM_LOG_LEVEL")))
	if level == "" {
		env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		switch env {
		case "development", "dev", "local", "test":
			return logger.Info
		default:
			return logger.Warn
		}
	}

	switch level {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn", "warning":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
