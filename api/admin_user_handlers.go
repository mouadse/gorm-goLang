package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type adminUpdateUserRequest struct {
	Name          *string  `json:"name"`
	Email         *string  `json:"email"`
	Role          *string  `json:"role"`
	Goal          *string  `json:"goal"`
	ActivityLevel *string  `json:"activity_level"`
	Avatar        *string  `json:"avatar"`
}

type adminBanUserRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	query := s.db.Model(&models.User{})

	if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" {
		query = query.Where("email LIKE ?", "%"+strings.ToLower(email)+"%")
	}
	if name := strings.TrimSpace(r.URL.Query().Get("name")); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if role := strings.TrimSpace(r.URL.Query().Get("role")); role != "" {
		query = query.Where("role = ?", role)
	}

	// Active vs Banned vs All (default active only if not specified?)
	// Let's just return all users including banned ones, unless specified
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "banned" {
		query = query.Where("banned_at IS NOT NULL")
	} else if status == "active" {
		query = query.Where("banned_at IS NULL")
	}

	// Pagination
	page := 1
	limit := 50
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		if l > 100 {
			l = 100 // cap max limit
		}
		limit = l
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var users []models.User
	if err := query.Order("created_at desc").Limit(limit).Offset((page - 1) * limit).Find(&users).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	response := map[string]any{
		"users": ensureSlice(users),
		"total": total,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Optionally fetch extra stats for the user
	var stats struct {
		WorkoutsCount int64 `json:"workouts_count"`
		MealsCount    int64 `json:"meals_count"`
	}
	s.db.Model(&models.Workout{}).Where("user_id = ?", userID).Count(&stats.WorkoutsCount)
	s.db.Model(&models.Meal{}).Where("user_id = ?", userID).Count(&stats.MealsCount)

	response := map[string]any{
		"user":  user,
		"stats": stats,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req adminUpdateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	oldUser := user // for audit

	if req.Email != nil {
		email, err := normalizeEmail(*req.Email)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user.Email = email
	}

	if req.Name != nil {
		name, err := requireNonBlank("name", *req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user.Name = name
	}

	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if role != "admin" && role != "user" && role != "moderator" {
			writeError(w, http.StatusBadRequest, errors.New("invalid role"))
			return
		}
		user.Role = role
	}

	if req.Goal != nil {
		user.Goal = strings.TrimSpace(*req.Goal)
	}

	if req.ActivityLevel != nil {
		user.ActivityLevel = strings.TrimSpace(*req.ActivityLevel)
	}

	if req.Avatar != nil {
		user.Avatar = strings.TrimSpace(*req.Avatar)
	}

	if err := s.db.Save(&user).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.logAdminAction(r, "update_user", "user", user.ID, oldUser, user)

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	confirm := r.URL.Query().Get("confirm")
	if confirm != "true" {
		writeError(w, http.StatusBadRequest, errors.New("must provide confirm=true query parameter to delete user"))
		return
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Just reuse the logic in handleAdminDeleteUser but wrapped in a transaction, or use the existing helper.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var workoutIDs []uuid.UUID
		if err := tx.Model(&models.Workout{}).Where("user_id = ?", userID).Pluck("id", &workoutIDs).Error; err != nil {
			return err
		}

		if len(workoutIDs) > 0 {
			if err := deleteWorkoutDependencies(tx, workoutIDs); err != nil {
				return err
			}
			if err := tx.Where("id IN ?", workoutIDs).Delete(&models.Workout{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("meal_id IN (SELECT id FROM meals WHERE user_id = ?)", userID).Delete(&models.MealFood{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.Meal{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.WeightEntry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.TwoFactorSecret{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, "id = ?", userID).Error
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.logAdminAction(r, "delete_user", "user", user.ID, user, nil)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminBanUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req adminBanUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if user.BannedAt != nil {
		writeError(w, http.StatusBadRequest, errors.New("user is already banned"))
		return
	}

	oldUser := user

	now := time.Now()
	user.BannedAt = &now
	user.BanReason = req.Reason

	// Force logout by incrementing auth version
	user.AuthVersion++

	if err := s.db.Save(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.logAdminAction(r, "ban_user", "user", user.ID, oldUser, user)

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleAdminUnbanUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if user.BannedAt == nil {
		writeError(w, http.StatusBadRequest, errors.New("user is not banned"))
		return
	}

	oldUser := user

	user.BannedAt = nil
	user.BanReason = ""

	if err := s.db.Save(&user).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.logAdminAction(r, "unban_user", "user", user.ID, oldUser, user)

	writeJSON(w, http.StatusOK, user)
}
