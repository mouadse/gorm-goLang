package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type contextKey string

const authenticatedUserIDKey contextKey = "authenticated_user_id"

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
