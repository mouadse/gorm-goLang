package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

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

	userID, err := parseRequiredUUID("user_id", req.UserID)
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
		MealType: strings.TrimSpace(req.MealType),
		Date:     mealDate,
		Notes:    req.Notes,
	}

	if meal.MealType == "" {
		writeError(w, http.StatusBadRequest, errors.New("meal_type is required"))
		return
	}

	if err := s.db.Create(&meal).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, meal)
}

func (s *Server) handleListMeals(w http.ResponseWriter, r *http.Request) {
	userID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	query := s.db.Where("user_id = ?", userID)

	if dateParam := strings.TrimSpace(r.URL.Query().Get("date")); dateParam != "" {
		parsedDate, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("date must be YYYY-MM-DD"))
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

	writeJSON(w, http.StatusOK, meals)
}

func (s *Server) handleUpdateMeal(w http.ResponseWriter, r *http.Request) {
	mealID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
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

	if req.MealType != nil {
		mealType := strings.TrimSpace(*req.MealType)
		if mealType == "" {
			writeError(w, http.StatusBadRequest, errors.New("meal_type cannot be empty"))
			return
		}
		meal.MealType = mealType
	}

	if req.Date != nil {
		parsedDate, err := time.Parse("2006-01-02", strings.TrimSpace(*req.Date))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("date must be YYYY-MM-DD"))
			return
		}
		meal.Date = parsedDate.UTC()
	}

	if req.Notes != nil {
		meal.Notes = *req.Notes
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
