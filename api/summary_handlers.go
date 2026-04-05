package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"gorm.io/gorm"
)

type dailySummaryResponse struct {
	Date                time.Time          `json:"date"`
	TotalCalories       float64            `json:"total_calories"`
	TotalProtein        float64            `json:"total_protein"`
	TotalCarbs          float64            `json:"total_carbs"`
	TotalFat            float64            `json:"total_fat"`
	TargetCalories      int                `json:"target_calories"`
	TargetProtein       int                `json:"target_protein"`
	TargetCarbs         int                `json:"target_carbs"`
	TargetFat           int                `json:"target_fat"`
	CalorieDelta        int                `json:"calorie_delta"`
	ProteinDelta        int                `json:"protein_delta"`
	CarbsDelta          int                `json:"carbs_delta"`
	FatDelta            int                `json:"fat_delta"`
	MealCount           int                `json:"meal_count"`
	MicronutrientTotals map[string]float64 `json:"micronutrient_totals"`
	FlaggedDeficiencies []string           `json:"flagged_deficiencies"`
	Meals               []models.Meal      `json:"meals"`
	Workouts            []models.Workout   `json:"workouts"`
}

type weeklySummaryResponse struct {
	StartDate           time.Time          `json:"start_date"`
	EndDate             time.Time          `json:"end_date"`
	TotalCalories       float64            `json:"total_calories"`
	TotalProtein        float64            `json:"total_protein"`
	TotalCarbs          float64            `json:"total_carbs"`
	TotalFat            float64            `json:"total_fat"`
	TargetCalories      int                `json:"target_calories"`
	TargetProtein       int                `json:"target_protein"`
	TargetCarbs         int                `json:"target_carbs"`
	TargetFat           int                `json:"target_fat"`
	CalorieDelta        int                `json:"calorie_delta"`
	ProteinDelta        int                `json:"protein_delta"`
	CarbsDelta          int                `json:"carbs_delta"`
	FatDelta            int                `json:"fat_delta"`
	MealCount           int                `json:"meal_count"`
	WorkoutCount        int                `json:"workout_count"`
	MicronutrientTotals map[string]float64 `json:"micronutrient_totals"`
	FlaggedDeficiencies []string           `json:"flagged_deficiencies"`
}

func startOfWeekUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return dayStart.AddDate(0, 0, -int(dayStart.Weekday()))
}

func calculateDeficiencies(proteinIntake, targetProtein float64, ironIntake float64, days int) []string {
	var flags []string
	if targetProtein > 0 && proteinIntake < targetProtein*0.8 {
		flags = append(flags, "Low Protein")
	}
	// Recommended Daily Allowance of iron is about 8-18mg depending on gender. 
	// We'll use 18mg as a safe generic threshold, but 100% DV is around 18mg.
	// Since iron is tracked from usda in mg. We flag if less than 10mg arbitrarily for this example,
	// or 14mg.
	ironThreshold := 14.0 * float64(days)
	if ironIntake < ironThreshold {
		flags = append(flags, "Low Iron")
	}
	return flags
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

	// Fetch targets
	targetSvc := services.NewNutritionTargetService(s.db)
	targets, err := targetSvc.GetUserNutritionTargets(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var meals []models.Meal
	if err := s.db.Preload("Items.Food.Nutrients.Nutrient").Where("user_id = ? AND date = ?", userID, summaryDate).Find(&meals).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var totalCalories, totalProtein, totalCarbs, totalFat float64
	micronutrients := make(map[string]float64)

	for i := range meals {
		meals[i].CalculateTotals()
		totalCalories += meals[i].TotalCalories
		totalProtein += meals[i].TotalProtein
		totalCarbs += meals[i].TotalCarbs
		totalFat += meals[i].TotalFat

		// Calculate micronutrients from meal items
		for _, item := range meals[i].Items {
			factor := (float64(item.Quantity) * item.Food.ServingSize) / 100.0
			for _, fn := range item.Food.Nutrients {
				micronutrients[fn.Nutrient.Name] += fn.AmountPer100g * factor
			}
		}
	}

	var workouts []models.Workout
	if err := s.db.Where("user_id = ? AND date = ?", userID, summaryDate).Find(&workouts).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	flags := calculateDeficiencies(totalProtein, float64(targets.Protein), micronutrients["Iron, Fe"], 1)

	response := dailySummaryResponse{
		Date:                summaryDate,
		TotalCalories:       totalCalories,
		TotalProtein:        totalProtein,
		TotalCarbs:          totalCarbs,
		TotalFat:            totalFat,
		TargetCalories:      targets.Calories,
		TargetProtein:       targets.Protein,
		TargetCarbs:         targets.Carbs,
		TargetFat:           targets.Fat,
		CalorieDelta:        int(totalCalories) - targets.Calories,
		ProteinDelta:        int(totalProtein) - targets.Protein,
		CarbsDelta:          int(totalCarbs) - targets.Carbs,
		FatDelta:            int(totalFat) - targets.Fat,
		MealCount:           len(meals),
		MicronutrientTotals: micronutrients,
		FlaggedDeficiencies: flags,
		Meals:               ensureSlice(meals),
		Workouts:            ensureSlice(workouts),
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetWeeklySummary(w http.ResponseWriter, r *http.Request) {
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

	startDate := startOfWeekUTC(summaryDate)
	endDate := startDate.AddDate(0, 0, 7)

	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("user not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	targetSvc := services.NewNutritionTargetService(s.db)
	targets, err := targetSvc.GetUserNutritionTargets(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var meals []models.Meal
	if err := s.db.Preload("Items.Food.Nutrients.Nutrient").Where("user_id = ? AND date >= ? AND date < ?", userID, startDate, endDate).Find(&meals).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var totalCalories, totalProtein, totalCarbs, totalFat float64
	micronutrients := make(map[string]float64)

	for i := range meals {
		meals[i].CalculateTotals()
		totalCalories += meals[i].TotalCalories
		totalProtein += meals[i].TotalProtein
		totalCarbs += meals[i].TotalCarbs
		totalFat += meals[i].TotalFat

		for _, item := range meals[i].Items {
			factor := (float64(item.Quantity) * item.Food.ServingSize) / 100.0
			for _, fn := range item.Food.Nutrients {
				micronutrients[fn.Nutrient.Name] += fn.AmountPer100g * factor
			}
		}
	}

	var workouts []models.Workout
	if err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, startDate, endDate).Find(&workouts).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// 7 days of targets
	weeklyTargetCal := targets.Calories * 7
	weeklyTargetPro := targets.Protein * 7
	weeklyTargetCarbs := targets.Carbs * 7
	weeklyTargetFat := targets.Fat * 7

	flags := calculateDeficiencies(totalProtein, float64(weeklyTargetPro), micronutrients["Iron, Fe"], 7)

	response := weeklySummaryResponse{
		StartDate:           startDate,
		EndDate:             endDate.AddDate(0, 0, -1),
		TotalCalories:       totalCalories,
		TotalProtein:        totalProtein,
		TotalCarbs:          totalCarbs,
		TotalFat:            totalFat,
		TargetCalories:      weeklyTargetCal,
		TargetProtein:       weeklyTargetPro,
		TargetCarbs:         weeklyTargetCarbs,
		TargetFat:           weeklyTargetFat,
		CalorieDelta:        int(totalCalories) - weeklyTargetCal,
		ProteinDelta:        int(totalProtein) - weeklyTargetPro,
		CarbsDelta:          int(totalCarbs) - weeklyTargetCarbs,
		FatDelta:            int(totalFat) - weeklyTargetFat,
		MealCount:           len(meals),
		WorkoutCount:        len(workouts),
		MicronutrientTotals: micronutrients,
		FlaggedDeficiencies: flags,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
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
	var recDate time.Time
	if dateParam == "" {
		recDate = time.Now().UTC().Truncate(24 * time.Hour)
	} else {
		recDate, err = parseDate(dateParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	svc := services.NewIntegrationRulesService(s.db)
	recommendation, err := svc.GetRecommendations(userID, recDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, recommendation)
}
