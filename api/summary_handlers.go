package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

type dailySummaryResponse struct {
	Date           time.Time        `json:"date"`
	TotalCalories  float64          `json:"total_calories"`
	TotalProtein   float64          `json:"total_protein"`
	TotalCarbs     float64          `json:"total_carbs"`
	TotalFat       float64          `json:"total_fat"`
	TargetCalories int              `json:"target_calories"`
	Meals          []models.Meal    `json:"meals"`
	Workouts       []models.Workout `json:"workouts"`
}

func (s *Server) handleGetDailySummary(w http.ResponseWriter, r *http.Request) {
	userID, err := scopedAuthenticatedUserID(r)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "forbidden" {
			status = http.StatusForbidden
		}
		writeError(w, status, err)
		return
	}

	dateParam := strings.TrimSpace(r.URL.Query().Get("date"))
	var summaryDate time.Time
	if dateParam == "" {
		summaryDate = time.Now().UTC().Truncate(24 * time.Hour)
	} else {
		summaryDate, err = parseDate(dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
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

	var meals []models.Meal
	if err := s.db.Preload("Items.Food").Where("user_id = ? AND date = ?", userID, summaryDate).Find(&meals).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var totalCalories, totalProtein, totalCarbs, totalFat float64
	for i := range meals {
		meals[i].CalculateTotals()
		totalCalories += meals[i].TotalCalories
		totalProtein += meals[i].TotalProtein
		totalCarbs += meals[i].TotalCarbs
		totalFat += meals[i].TotalFat
	}

	var workouts []models.Workout
	if err := s.db.Where("user_id = ? AND date = ?", userID, summaryDate).Find(&workouts).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	response := dailySummaryResponse{
		Date:           summaryDate,
		TotalCalories:  totalCalories,
		TotalProtein:   totalProtein,
		TotalCarbs:     totalCarbs,
		TotalFat:       totalFat,
		TargetCalories: user.TDEE,
		Meals:          ensureSlice(meals),
		Workouts:       ensureSlice(workouts),
	}

	writeJSON(w, http.StatusOK, response)
}
