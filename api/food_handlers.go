package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"gorm.io/gorm"
)

type createFoodRequest struct {
	Name          string  `json:"name"`
	Brand         string  `json:"brand"`
	ServingSize   float64 `json:"serving_size"`
	ServingUnit   string  `json:"serving_unit"`
	Calories      float64 `json:"calories"`
	Protein       float64 `json:"protein"`
	Carbohydrates float64 `json:"carbohydrates"`
	Fat           float64 `json:"fat"`
	Fiber         float64 `json:"fiber"`
	Sugar         float64 `json:"sugar"`
	Sodium        float64 `json:"sodium"`
}

type updateFoodRequest struct {
	Name          *string  `json:"name"`
	Brand         *string  `json:"brand"`
	ServingSize   *float64 `json:"serving_size"`
	ServingUnit   *string  `json:"serving_unit"`
	Calories      *float64 `json:"calories"`
	Protein       *float64 `json:"protein"`
	Carbohydrates *float64 `json:"carbohydrates"`
	Fat           *float64 `json:"fat"`
	Fiber         *float64 `json:"fiber"`
	Sugar         *float64 `json:"sugar"`
	Sodium        *float64 `json:"sodium"`
}

func (s *Server) handleCreateFood(w http.ResponseWriter, r *http.Request) {
	var req createFoodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	servingUnit, err := requireNonBlank("serving_unit", req.ServingUnit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	food := models.Food{
		Name:          name,
		Brand:         strings.TrimSpace(req.Brand),
		ServingSize:   req.ServingSize,
		ServingUnit:   servingUnit,
		Calories:      req.Calories,
		Protein:       req.Protein,
		Carbohydrates: req.Carbohydrates,
		Fat:           req.Fat,
		Fiber:         req.Fiber,
		Sugar:         req.Sugar,
		Sodium:        req.Sodium,
	}

	if err := s.db.Create(&food).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusCreated, food)
}

func (s *Server) handleListFoods(w http.ResponseWriter, r *http.Request) {
	query := s.db.Model(&models.Food{})

	if name := strings.TrimSpace(r.URL.Query().Get("name")); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if brand := strings.TrimSpace(r.URL.Query().Get("brand")); brand != "" {
		query = query.Where("brand ILIKE ?", "%"+brand+"%")
	}

	if category := strings.TrimSpace(r.URL.Query().Get("category")); category != "" {
		query = query.Where("category ILIKE ?", "%"+category+"%")
	}

	if source := strings.TrimSpace(r.URL.Query().Get("source")); source != "" {
		query = query.Where("source = ?", source)
	}

	var foods []models.Food
	if err := query.Order("name asc").Find(&foods).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(foods))
}


func (s *Server) handleGetFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var food models.Food
	if err := s.db.First(&food, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("food not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, food)
}

func (s *Server) handleUpdateFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateFoodRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var food models.Food
	if err := s.db.First(&food, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("food not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if req.Name != nil {
		food.Name = strings.TrimSpace(*req.Name)
	}
	if req.Brand != nil {
		food.Brand = strings.TrimSpace(*req.Brand)
	}
	if req.ServingSize != nil {
		food.ServingSize = *req.ServingSize
	}
	if req.ServingUnit != nil {
		food.ServingUnit = strings.TrimSpace(*req.ServingUnit)
	}
	if req.Calories != nil {
		food.Calories = *req.Calories
	}
	if req.Protein != nil {
		food.Protein = *req.Protein
	}
	if req.Carbohydrates != nil {
		food.Carbohydrates = *req.Carbohydrates
	}
	if req.Fat != nil {
		food.Fat = *req.Fat
	}
	if req.Fiber != nil {
		food.Fiber = *req.Fiber
	}
	if req.Sugar != nil {
		food.Sugar = *req.Sugar
	}
	if req.Sodium != nil {
		food.Sodium = *req.Sodium
	}

	if err := s.db.Save(&food).Error; err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, food)
}

func (s *Server) handleDeleteFood(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var count int64
	if err := s.db.Model(&models.MealFood{}).Where("food_id = ?", id).Count(&count).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, errors.New("cannot delete food because it is referenced by one or more meals"))
		return
	}

	result := s.db.Delete(&models.Food{}, "id = ?", id)
	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		writeError(w, http.StatusNotFound, errors.New("food not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
