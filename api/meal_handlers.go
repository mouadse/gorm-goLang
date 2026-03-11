package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

type createMealRequest struct {
	UserID   string `json:"user_id"`
	MealType string `json:"meal_type"`
	Date     string `json:"date"`
	Notes    string `json:"notes"`
}

type updateMealRequest struct {
	UserID   *string `json:"user_id"`
	MealType *string `json:"meal_type"`
	Date     *string `json:"date"`
	Notes    *string `json:"notes"`
}

func (s *Server) handleCreateMeal(w http.ResponseWriter, r *http.Request) {
	var req createMealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userID, err := resolveScopedUUID(r, "user_id", "user_id", req.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := authorizeUser(r, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	exists, err := recordExists(s.db, &models.User{}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}

	mealType, err := requireNonBlank("meal_type", req.MealType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	mealDate, err := parseDateOrDefault(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	meal := models.Meal{
		UserID:   userID,
		MealType: mealType,
		Date:     mealDate,
		Notes:    strings.TrimSpace(req.Notes),
	}

	if err := s.db.Create(&meal).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, meal)
}

func (s *Server) handleListMeals(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}
	query := s.db.Model(&models.Meal{}).Where("user_id = ?", userID)

	if dateParam := strings.TrimSpace(r.URL.Query().Get("date")); dateParam != "" {
		parsedDate, err := parseDate(dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		query = query.Where("date = ?", parsedDate)
	}

	if mealType := strings.TrimSpace(r.URL.Query().Get("meal_type")); mealType != "" {
		query = query.Where("meal_type = ?", mealType)
	}

	var meals []models.Meal
	if err := query.Order("date desc, created_at desc").Find(&meals).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(meals))
}

func (s *Server) handleGetMeal(w http.ResponseWriter, r *http.Request) {
	mealID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var meal models.Meal
	if err := s.db.First(&meal, "id = ?", mealID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, meal)
}

func (s *Server) handleUpdateMeal(w http.ResponseWriter, r *http.Request) {
	mealID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateMealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var meal models.Meal
	if err := s.db.First(&meal, "id = ?", mealID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.UserID != nil {
		userID, err := parseRequiredUUID("user_id", *req.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		exists, err := recordExists(s.db, &models.User{}, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		if err := authorizeUser(r, userID); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}

		meal.UserID = userID
	}

	if req.MealType != nil {
		mealType, err := requireNonBlank("meal_type", *req.MealType)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		meal.MealType = mealType
	}

	if req.Date != nil {
		parsedDate, err := parseDate(*req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		meal.Date = parsedDate
	}

	if req.Notes != nil {
		meal.Notes = strings.TrimSpace(*req.Notes)
	}

	if err := s.db.Save(&meal).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, meal)
}

func (s *Server) handleDeleteMeal(w http.ResponseWriter, r *http.Request) {
	mealID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	result := s.db.Delete(&models.Meal{}, "id = ?", mealID)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("meal not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
