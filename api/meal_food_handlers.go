package api

import (
	"errors"
	"net/http"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

type createMealFoodRequest struct {
	MealID   string  `json:"meal_id"`
	FoodID   string  `json:"food_id"`
	Quantity float64 `json:"quantity"`
}

type updateMealFoodRequest struct {
	Quantity *float64 `json:"quantity"`
}

func (s *Server) handleCreateMealFood(w http.ResponseWriter, r *http.Request) {
	var req createMealFoodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	mealID, err := resolveScopedUUID(r, "id", "meal_id", req.MealID)
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

	foodID, err := parseRequiredUUID("food_id", req.FoodID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	exists, err := recordExists(s.db, &models.Food{}, foodID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, errors.New("food not found"))
		return
	}

	if req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, errors.New("quantity must be greater than 0"))
		return
	}

	mealFood := models.MealFood{
		MealID:   mealID,
		FoodID:   foodID,
		Quantity: req.Quantity,
	}

	if err := s.db.Create(&mealFood).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// Preload Food for the response
	s.db.Preload("Food").First(&mealFood, "id = ?", mealFood.ID)

	writeJSON(w, http.StatusCreated, mealFood)
}

func (s *Server) handleListMealFoods(w http.ResponseWriter, r *http.Request) {
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

	var items []models.MealFood
	if err := s.db.Preload("Food").Where("meal_id = ?", mealID).Find(&items).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(items))
}

func (s *Server) handleGetMealFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var mealFood models.MealFood
	if err := s.db.First(&mealFood, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal food item not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealFood.MealID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	s.db.Preload("Food").First(&mealFood, "id = ?", id)
	writeJSON(w, http.StatusOK, mealFood)
}

func (s *Server) handleUpdateMealFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var mealFood models.MealFood
	if err := s.db.First(&mealFood, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal food item not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealFood.MealID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	var req updateMealFoodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Quantity != nil {
		if *req.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("quantity must be greater than 0"))
			return
		}
		mealFood.Quantity = *req.Quantity
	}

	if err := s.db.Save(&mealFood).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.db.Preload("Food").First(&mealFood, "id = ?", id)
	writeJSON(w, http.StatusOK, mealFood)
}

func (s *Server) handleDeleteMealFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var mealFood models.MealFood
	if err := s.db.First(&mealFood, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("meal food item not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ownerID, err := s.mealOwnerID(mealFood.MealID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := authorizeUser(r, ownerID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	if err := s.db.Delete(&mealFood).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
