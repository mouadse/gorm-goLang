package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"fitness-tracker/models"

	"gorm.io/gorm"
)

// handleGetRecentMeals godoc
// @Summary Get recently logged meals
// @Description Fetch recently logged meals for the authenticated user, optionally bounded by days param
// @Tags meal-reuse
// @Produce json
// @Param days query int false "Number of days back (default is 7)"
// @Success 200 {array} models.Meal
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/meals/recent [get]
// @Security BearerAuth
func (s *Server) handleGetRecentMeals(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	days := 7
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	now := time.Now().UTC()
	cutoff := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, time.UTC)

	var meals []models.Meal
	if err := s.db.Where("user_id = ? AND date >= ?", currentUserID, cutoff).
		Order("date desc").
		Preload("Items.Food").
		Find(&meals).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(meals))
}

// handleCloneMeal godoc
// @Summary Clone an existing meal
// @Description Duplicates a meal and all its items into a new entry for the current day
// @Tags meal-reuse
// @Produce json
// @Param id path string true "Meal ID to clone" format(uuid)
// @Success 201 {object} models.Meal
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/meals/{id}/clone [post]
// @Security BearerAuth
func (s *Server) handleCloneMeal(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	originalMealID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var cloned models.Meal
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var original models.Meal
		if err := tx.Preload("Items").First(&original, "id = ?", originalMealID).Error; err != nil {
			return err
		}

		if original.UserID != currentUserID {
			return errors.New("forbidden")
		}

		cloned = models.Meal{
			UserID:   currentUserID,
			MealType: original.MealType,
			Date:     time.Now().UTC(),
			Notes:    original.Notes,
		}

		if err := tx.Create(&cloned).Error; err != nil {
			return err
		}

		for _, item := range original.Items {
			clonedItem := models.MealFood{
				MealID:   cloned.ID,
				FoodID:   item.FoodID,
				Quantity: item.Quantity,
			}
			if err := tx.Create(&clonedItem).Error; err != nil {
				return err
			}
			cloned.Items = append(cloned.Items, clonedItem)
		}

		return nil
	})

	if err != nil {
		if err.Error() == "forbidden" {
			writeError(w, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.db.Preload("Items.Food").First(&cloned, "id = ?", cloned.ID).Error; err == nil {
		cloned.CalculateTotals()
	}

	writeJSON(w, http.StatusCreated, cloned)
}

// handleGetRecentFoods godoc
// @Summary Get recently used foods
// @Description Determines recently used foods derived from the user's meal history
// @Tags meal-reuse
// @Produce json
// @Success 200 {array} models.Food
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/foods/recent [get]
// @Security BearerAuth
func (s *Server) handleGetRecentFoods(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var foods []models.Food
	err = s.db.Raw(`
		SELECT f.* 
		FROM foods f
		JOIN (
			SELECT mf.food_id, MAX(mf.created_at) as last_used
			FROM meal_foods mf
			JOIN meals m ON mf.meal_id = m.id
			WHERE m.user_id = ?
			GROUP BY mf.food_id
			ORDER BY last_used DESC
			LIMIT 20
		) recent ON f.id = recent.food_id
		ORDER BY recent.last_used DESC
	`, currentUserID).Scan(&foods).Error

	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(foods))
}

// handleFavoriteFood godoc
// @Summary Favorite a food
// @Description Adds a food to user's favorites
// @Tags meal-reuse
// @Produce json
// @Param id path string true "Food ID" format(uuid)
// @Success 201 {object} models.FavoriteFood
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/foods/{id}/favorite [post]
// @Security BearerAuth
func (s *Server) handleFavoriteFood(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	foodID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var food models.Food
	if err := s.db.First(&food, "id = ?", foodID).Error; err != nil {
		writeError(w, http.StatusNotFound, errors.New("food not found"))
		return
	}

	favorite := models.FavoriteFood{
		UserID: currentUserID,
		FoodID: foodID,
	}

	if err := s.db.Create(&favorite).Error; err != nil {
		// Note: GORM/SQLite unique constraint error string varies, treating all errors as potentially conflict if row exists
		var existing models.FavoriteFood
		if s.db.Where("user_id = ? AND food_id = ?", currentUserID, foodID).First(&existing).Error == nil {
			writeError(w, http.StatusConflict, errors.New("food already favorited"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, favorite)
}

// handleUnfavoriteFood godoc
// @Summary Unfavorite a food
// @Description Removes a food from user's favorites
// @Tags meal-reuse
// @Param id path string true "Food ID" format(uuid)
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/foods/{id}/favorite [delete]
// @Security BearerAuth
func (s *Server) handleUnfavoriteFood(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	foodID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result := s.db.Where("user_id = ? AND food_id = ?", currentUserID, foodID).Delete(&models.FavoriteFood{})
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}

	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("favorite not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetFavorites godoc
// @Summary Get favorite foods
// @Description Fetch user's favorite foods
// @Tags meal-reuse
// @Produce json
// @Param user_id path string true "User ID" format(uuid)
// @Success 200 {array} models.FavoriteFood
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /v1/users/{user_id}/favorites [get]
// @Security BearerAuth
func (s *Server) handleGetFavorites(w http.ResponseWriter, r *http.Request) {
	targetUserID, err := parsePathUUID(r, "user_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := authorizeUser(r, targetUserID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var favorites []models.FavoriteFood
	if err := s.db.Where("user_id = ?", targetUserID).Preload("Food").Find(&favorites).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(favorites))
}
