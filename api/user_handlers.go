package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const placeholderPasswordHash = "pending-auth"

type createUserRequest struct {
	Email         string  `json:"email"`
	PasswordHash  string  `json:"password_hash"`
	Name          string  `json:"name"`
	Avatar        string  `json:"avatar"`
	Age           int     `json:"age"`
	DateOfBirth   string  `json:"date_of_birth"`
	Weight        float64 `json:"weight"`
	Height        float64 `json:"height"`
	Goal          string  `json:"goal"`
	ActivityLevel string  `json:"activity_level"`
	TDEE          int     `json:"tdee"`
}

type updateUserRequest struct {
	Email         *string  `json:"email"`
	Name          *string  `json:"name"`
	Avatar        *string  `json:"avatar"`
	Age           *int     `json:"age"`
	DateOfBirth   *string  `json:"date_of_birth"`
	Weight        *float64 `json:"weight"`
	Height        *float64 `json:"height"`
	Goal          *string  `json:"goal"`
	ActivityLevel *string  `json:"activity_level"`
	TDEE          *int     `json:"tdee"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	email, err := requireNonBlank("email", req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	dateOfBirth, err := parseOptionalBirthDate(req.DateOfBirth)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Age < 0 || req.TDEE < 0 || req.Weight < 0 || req.Height < 0 {
		writeError(w, http.StatusBadRequest, errors.New("numeric profile fields cannot be negative"))
		return
	}

	user := models.User{
		Email:         email,
		PasswordHash:  defaultPasswordHash(req.PasswordHash),
		Name:          name,
		Avatar:        strings.TrimSpace(req.Avatar),
		Age:           req.Age,
		DateOfBirth:   dateOfBirth,
		Weight:        req.Weight,
		Height:        req.Height,
		Goal:          strings.TrimSpace(req.Goal),
		ActivityLevel: strings.TrimSpace(req.ActivityLevel),
		TDEE:          req.TDEE,
	}

	if err := s.db.Create(&user).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	query := s.db.Model(&models.User{})

	if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" {
		query = query.Where("email = ?", email)
	}

	if name := strings.TrimSpace(r.URL.Query().Get("name")); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	var users []models.User
	if err := query.Order("created_at desc").Find(&users).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(users))
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateUserRequest
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

	if req.Email != nil {
		email, err := requireNonBlank("email", *req.Email)
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

	if req.Avatar != nil {
		user.Avatar = strings.TrimSpace(*req.Avatar)
	}

	if req.Age != nil {
		if *req.Age < 0 {
			writeError(w, http.StatusBadRequest, errors.New("age cannot be negative"))
			return
		}
		user.Age = *req.Age
	}

	if req.DateOfBirth != nil {
		dateOfBirth, err := parseOptionalBirthDate(*req.DateOfBirth)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		user.DateOfBirth = dateOfBirth
	}

	if req.Weight != nil {
		if *req.Weight < 0 {
			writeError(w, http.StatusBadRequest, errors.New("weight cannot be negative"))
			return
		}
		user.Weight = *req.Weight
	}

	if req.Height != nil {
		if *req.Height < 0 {
			writeError(w, http.StatusBadRequest, errors.New("height cannot be negative"))
			return
		}
		user.Height = *req.Height
	}

	if req.Goal != nil {
		user.Goal = strings.TrimSpace(*req.Goal)
	}

	if req.ActivityLevel != nil {
		user.ActivityLevel = strings.TrimSpace(*req.ActivityLevel)
	}

	if req.TDEE != nil {
		if *req.TDEE < 0 {
			writeError(w, http.StatusBadRequest, errors.New("tdee cannot be negative"))
			return
		}
		user.TDEE = *req.TDEE
	}

	if err := s.db.Save(&user).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Select("id").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		var workoutIDs []uuid.UUID
		if err := tx.Model(&models.Workout{}).
			Where("user_id = ?", userID).
			Pluck("id", &workoutIDs).Error; err != nil {
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

		if err := tx.Where("user_id = ?", userID).Delete(&models.Meal{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.WeightEntry{}).Error; err != nil {
			return err
		}

		return tx.Delete(&models.User{}, "id = ?", userID).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func defaultPasswordHash(passwordHash string) string {
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		return placeholderPasswordHash
	}
	return passwordHash
}

func parseOptionalBirthDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parsed, err := parseDate(raw)
	if err != nil {
		return nil, fmt.Errorf("date_of_birth must be %s", dateLayout)
	}

	return &parsed, nil
}
