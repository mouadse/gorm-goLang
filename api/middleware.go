package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type contextKey string

const authenticatedUserIDKey contextKey = "authenticated_user_id"

const (
	defaultCORSAllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	defaultCORSAllowedHeaders = "Authorization, Content-Type, Accept, Origin, X-Requested-With"
	defaultCORSMaxAgeSeconds  = 600
)

var defaultDevelopmentCORSOrigins = []string{
	"http://localhost:3000",
	"http://localhost:5173",
	"http://127.0.0.1:3000",
	"http://127.0.0.1:5173",
}

type corsConfig struct {
	allowedOrigins map[string]struct{}
	allowAll       bool
	allowMethods   string
	allowHeaders   string
	exposeHeaders  string
	credentials    bool
	maxAgeSeconds  int
}

func newCORSConfigFromEnv() corsConfig {
	origins := splitCommaSeparated(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(origins) == 0 && !isProductionEnv() {
		origins = defaultDevelopmentCORSOrigins
	}

	cfg := corsConfig{
		allowedOrigins: make(map[string]struct{}, len(origins)),
		allowMethods:   envString("CORS_ALLOWED_METHODS", defaultCORSAllowedMethods),
		allowHeaders:   strings.TrimSpace(os.Getenv("CORS_ALLOWED_HEADERS")),
		exposeHeaders:  strings.TrimSpace(os.Getenv("CORS_EXPOSED_HEADERS")),
		credentials:    envBool("CORS_ALLOW_CREDENTIALS", true),
		maxAgeSeconds:  envPositiveInt("CORS_MAX_AGE_SECONDS", defaultCORSMaxAgeSeconds),
	}

	for _, origin := range origins {
		if origin == "*" {
			cfg.allowAll = true
			continue
		}
		cfg.allowedOrigins[origin] = struct{}{}
	}
	if cfg.allowAll {
		cfg.credentials = false
	}

	return cfg
}

func (cfg corsConfig) Middleware(next http.Handler) http.Handler {
	if !cfg.enabled() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		appendVary(w.Header(), "Origin")

		if !cfg.originAllowed(origin) {
			if isCORSPreflight(r) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		cfg.writeActualHeaders(w, origin)
		if isCORSPreflight(r) {
			cfg.writePreflightHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (cfg corsConfig) enabled() bool {
	return cfg.allowAll || len(cfg.allowedOrigins) > 0
}

func (cfg corsConfig) originAllowed(origin string) bool {
	if cfg.allowAll {
		return true
	}
	_, ok := cfg.allowedOrigins[origin]
	return ok
}

func (cfg corsConfig) writeActualHeaders(w http.ResponseWriter, origin string) {
	header := w.Header()

	if cfg.allowAll && !cfg.credentials {
		header.Set("Access-Control-Allow-Origin", "*")
	} else {
		header.Set("Access-Control-Allow-Origin", origin)
	}
	if cfg.credentials {
		header.Set("Access-Control-Allow-Credentials", "true")
	}
	if cfg.exposeHeaders != "" {
		header.Set("Access-Control-Expose-Headers", cfg.exposeHeaders)
	}
}

func (cfg corsConfig) writePreflightHeaders(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	appendVary(header, "Access-Control-Request-Method")
	appendVary(header, "Access-Control-Request-Headers")

	header.Set("Access-Control-Allow-Methods", cfg.allowMethods)
	if cfg.allowHeaders != "" {
		header.Set("Access-Control-Allow-Headers", cfg.allowHeaders)
	} else if requestedHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); requestedHeaders != "" {
		header.Set("Access-Control-Allow-Headers", requestedHeaders)
	} else {
		header.Set("Access-Control-Allow-Headers", defaultCORSAllowedHeaders)
	}
	header.Set("Access-Control-Max-Age", strconv.Itoa(cfg.maxAgeSeconds))
}

func isCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")) != ""
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envPositiveInt(key string, fallback int) int {
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

func isProductionEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func appendVary(header http.Header, value string) {
	for _, existingValues := range header.Values("Vary") {
		for _, existing := range strings.Split(existingValues, ",") {
			if strings.EqualFold(strings.TrimSpace(existing), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func Authenticate(db *gorm.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, errors.New("missing authorization header"))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeError(w, http.StatusUnauthorized, errors.New("invalid authorization format"))
			return
		}

		userID, tokenAuthVersion, err := services.ParseAccessToken(parts[1])
		if err != nil {
			if errors.Is(err, services.ErrMissingJWTSecret) {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
			return
		}

		var authState struct {
			AuthVersion sql.NullInt64
			BannedAt    sql.NullTime
		}
		if err := db.Model(&models.User{}).
			Select("auth_version, banned_at").
			Where("id = ? AND deleted_at IS NULL", userID).
			First(&authState).Error; err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
			return
		}
		if authState.AuthVersion.Valid && uint(authState.AuthVersion.Int64) != tokenAuthVersion {
			writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
			return
		}
		if authState.BannedAt.Valid {
			writeError(w, http.StatusForbidden, errors.New("account banned"))
			return
		}

		ctx := context.WithValue(r.Context(), authenticatedUserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin ensures the authenticated user has the "admin" role.
// Must be chained after Authenticate.
func RequireAdmin(db *gorm.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := authenticatedUserID(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}

		var role string
		if err := db.Model(&models.User{}).Select("role").Where("id = ?", userID).Scan(&role).Error; err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("failed to check user privileges"))
			return
		}

		if role != "admin" {
			writeError(w, http.StatusForbidden, errors.New("admin privileges required"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authenticatedUserID(r *http.Request) (uuid.UUID, error) {
	value := r.Context().Value(authenticatedUserIDKey)
	userID, ok := value.(uuid.UUID)
	if !ok || userID == uuid.Nil {
		return uuid.Nil, errors.New("authenticated user missing from context")
	}
	return userID, nil
}

func authorizeUser(r *http.Request, resourceUserID uuid.UUID) error {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		return err
	}
	if currentUserID != resourceUserID {
		return errors.New("forbidden")
	}
	return nil
}

func scopedAuthenticatedUserID(r *http.Request) (uuid.UUID, error) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		return uuid.Nil, err
	}

	if pathUserID := strings.TrimSpace(r.PathValue("user_id")); pathUserID != "" {
		userID, err := parseRequiredUUID("user_id", pathUserID)
		if err != nil {
			return uuid.Nil, err
		}
		if err := authorizeUser(r, userID); err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	}

	if queryUserID := strings.TrimSpace(r.URL.Query().Get("user_id")); queryUserID != "" {
		userID, err := parseRequiredUUID("user_id", queryUserID)
		if err != nil {
			return uuid.Nil, err
		}
		if err := authorizeUser(r, userID); err != nil {
			return uuid.Nil, err
		}
		return userID, nil
	}

	return currentUserID, nil
}

func (s *Server) workoutOwnerID(workoutID uuid.UUID) (uuid.UUID, error) {
	var workout models.Workout
	if err := s.db.Select("user_id").First(&workout, "id = ?", workoutID).Error; err != nil {
		return uuid.Nil, err
	}
	return workout.UserID, nil
}

func (s *Server) mealOwnerID(mealID uuid.UUID) (uuid.UUID, error) {
	var meal models.Meal
	if err := s.db.Select("user_id").First(&meal, "id = ?", mealID).Error; err != nil {
		return uuid.Nil, err
	}
	return meal.UserID, nil
}

func (s *Server) weightEntryOwnerID(weightEntryID uuid.UUID) (uuid.UUID, error) {
	var entry models.WeightEntry
	if err := s.db.Select("user_id").First(&entry, "id = ?", weightEntryID).Error; err != nil {
		return uuid.Nil, err
	}
	return entry.UserID, nil
}

func (s *Server) workoutExerciseOwnerID(workoutExerciseID uuid.UUID) (uuid.UUID, error) {
	var result struct {
		UserID uuid.UUID
	}

	tx := s.db.Table("workout_exercises").
		Select("workouts.user_id AS user_id").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workout_exercises.id = ?", workoutExerciseID).
		Scan(&result)
	if tx.Error != nil {
		return uuid.Nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return uuid.Nil, gorm.ErrRecordNotFound
	}

	return result.UserID, nil
}

func (s *Server) workoutSetOwnerID(workoutSetID uuid.UUID) (uuid.UUID, error) {
	var result struct {
		UserID uuid.UUID
	}

	tx := s.db.Table("workout_sets").
		Select("workouts.user_id AS user_id").
		Joins("JOIN workout_exercises ON workout_exercises.id = workout_sets.workout_exercise_id").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workout_sets.id = ?", workoutSetID).
		Scan(&result)
	if tx.Error != nil {
		return uuid.Nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return uuid.Nil, gorm.ErrRecordNotFound
	}

	return result.UserID, nil
}
