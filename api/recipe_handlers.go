package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createRecipeRequest struct {
	Name     string                    `json:"name"`
	Servings int                       `json:"servings"`
	Notes    string                    `json:"notes"`
	Items    []createRecipeItemRequest `json:"items"`
}

type createRecipeItemRequest struct {
	FoodID   uuid.UUID `json:"food_id"`
	Quantity float64   `json:"quantity"`
}

type updateRecipeRequest struct {
	Name     *string `json:"name"`
	Servings *int    `json:"servings"`
	Notes    *string `json:"notes"`
}

// handleCreateRecipe godoc
func (s *Server) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req createRecipeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Servings <= 0 {
		req.Servings = 1
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, errors.New("recipe name is required"))
		return
	}

	recipe := models.Recipe{
		UserID:   currentUserID,
		Name:     name,
		Servings: req.Servings,
		Notes:    req.Notes,
	}

	for _, itemReq := range req.Items {
		if itemReq.Quantity <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("quantity must be greater than zero"))
			return
		}

		var food models.Food
		if err := s.db.First(&food, "id = ?", itemReq.FoodID).Error; err != nil {
			writeError(w, http.StatusBadRequest, errors.New("invalid food referenced"))
			return
		}

		recipe.Items = append(recipe.Items, models.RecipeItem{
			FoodID:   itemReq.FoodID,
			Quantity: itemReq.Quantity,
		})
	}

	if err := s.db.Create(&recipe).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// reload with preloaded foods
	s.db.Preload("Items.Food").First(&recipe, "id = ?", recipe.ID)

	writeJSON(w, http.StatusCreated, recipe)
}

// handleListRecipes godoc
func (s *Server) handleListRecipes(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var recipes []models.Recipe
	if err := s.db.Where("user_id = ?", currentUserID).
		Order("created_at desc").
		Preload("Items.Food").
		Find(&recipes).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, ensureSlice(recipes))
}

// handleGetRecipe godoc
func (s *Server) handleGetRecipe(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var recipe models.Recipe
	if err := s.db.Preload("Items.Food").First(&recipe, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("recipe not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if recipe.UserID != currentUserID {
		writeError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}

	writeJSON(w, http.StatusOK, recipe)
}

// handleUpdateRecipe godoc
func (s *Server) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateRecipeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var recipe models.Recipe
	if err := s.db.First(&recipe, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("recipe not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if recipe.UserID != currentUserID {
		writeError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, errors.New("recipe name cannot be blank"))
			return
		}
		recipe.Name = name
	}
	if req.Servings != nil {
		if *req.Servings <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("servings must be greater than zero"))
			return
		}
		recipe.Servings = *req.Servings
	}
	if req.Notes != nil {
		recipe.Notes = *req.Notes
	}

	if err := s.db.Save(&recipe).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.db.Preload("Items.Food").First(&recipe, "id = ?", recipe.ID)
	writeJSON(w, http.StatusOK, recipe)
}

// handleDeleteRecipe godoc
func (s *Server) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var recipe models.Recipe
	if err := s.db.First(&recipe, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("recipe not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if recipe.UserID != currentUserID {
		writeError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("recipe_id = ?", recipe.ID).Delete(&models.RecipeItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&recipe).Error
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetRecipeNutrition godoc
func (s *Server) handleGetRecipeNutrition(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var recipe models.Recipe
	if err := s.db.Preload("Items.Food").First(&recipe, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("recipe not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if recipe.UserID != currentUserID {
		writeError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}

	// Calculate nutrition per serving
	var cals, prot, carbs, fat, fib float64

	for _, item := range recipe.Items {
		cals += item.Food.Calories * item.Quantity
		prot += item.Food.Protein * item.Quantity
		carbs += item.Food.Carbohydrates * item.Quantity
		fat += item.Food.Fat * item.Quantity
		fib += item.Food.Fiber * item.Quantity
	}

	servings := float64(recipe.Servings)
	if servings <= 0 {
		servings = 1
	}

	nutrition := map[string]float64{
		"calories":      cals / servings,
		"protein":       prot / servings,
		"carbohydrates": carbs / servings,
		"fat":           fat / servings,
		"fiber":         fib / servings,
	}

	writeJSON(w, http.StatusOK, nutrition)
}

type logRecipeToMealRequest struct {
	Date     string  `json:"date"`
	MealType string  `json:"meal_type"`
	Servings float64 `json:"servings"`
}

// handleLogRecipeToMeal godoc
func (s *Server) handleLogRecipeToMeal(w http.ResponseWriter, r *http.Request) {
	currentUserID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	id, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req logRecipeToMealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Servings <= 0 {
		req.Servings = 1
	}

	date, err := parseDate(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid date"))
		return
	}

	var recipe models.Recipe
	if err := s.db.Preload("Items").First(&recipe, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, errors.New("recipe not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if recipe.UserID != currentUserID {
		writeError(w, http.StatusForbidden, errors.New("forbidden"))
		return
	}

	mealType, err := requireNonBlank("meal_type", req.MealType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var meal models.Meal
	err = s.db.Transaction(func(tx *gorm.DB) error {
		meal = models.Meal{
			UserID:   currentUserID,
			MealType: mealType,
			Date:     date,
			Notes:    "From recipe: " + recipe.Name,
		}
		if err := tx.Create(&meal).Error; err != nil {
			return err
		}

		// Calculate ratio depending on how many servings they want
		// The recipe items represent the TOTAL recipe.
		// If recipe makes 4 servings, and they want 2 servings logged, ratio = 2/4 = 0.5.
		ratio := req.Servings / float64(recipe.Servings)

		for _, item := range recipe.Items {
			mealFood := models.MealFood{
				MealID:   meal.ID,
				FoodID:   item.FoodID,
				Quantity: item.Quantity * ratio,
			}
			if err := tx.Create(&mealFood).Error; err != nil {
				return err
			}
			meal.Items = append(meal.Items, mealFood)
		}
		return nil
	})

	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.db.Preload("Items.Food").First(&meal, "id = ?", meal.ID).Error; err == nil {
		meal.CalculateTotals()
	}

	writeJSON(w, http.StatusCreated, meal)
}
