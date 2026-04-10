package api_test

import (
	"fitness-tracker/api"
	"net/http"
	"testing"
	"time"

	"fitness-tracker/models"
)

func TestRecipeHandlers(t *testing.T) {
	_, server := newTestApp(t)

	auth := registerTestUser(t, server, "recipes@example.com", "Chef", "password123")

	food1 := requestJSONAuth[models.Food](t, server, auth.AccessToken, http.MethodPost, "/v1/foods", map[string]any{
		"name":          "Flour",
		"serving_size":  100,
		"serving_unit":  "g",
		"calories":      364,
		"protein":       10,
		"carbohydrates": 76,
		"fat":           1,
	}, http.StatusCreated)

	food2 := requestJSONAuth[models.Food](t, server, auth.AccessToken, http.MethodPost, "/v1/foods", map[string]any{
		"name":          "Egg",
		"serving_size":  50,
		"serving_unit":  "g",
		"calories":      70,
		"protein":       6,
		"carbohydrates": 1,
		"fat":           5,
	}, http.StatusCreated)

	var recipe models.Recipe
	t.Run("Create Recipe", func(t *testing.T) {
		recipe = requestJSONAuth[models.Recipe](t, server, auth.AccessToken, http.MethodPost, "/v1/recipes", map[string]any{
			"name":     "Pancakes",
			"servings": 4,
			"notes":    "Fluffy pancakes",
			"items": []map[string]any{
				{"food_id": food1.ID, "quantity": 2.0}, // 200g flour
				{"food_id": food2.ID, "quantity": 2.0}, // 2 eggs
			},
		}, http.StatusCreated)

		if recipe.Name != "Pancakes" {
			t.Fatalf("expected Pancakes, got %s", recipe.Name)
		}
		if len(recipe.Items) != 2 {
			t.Fatalf("expected 2 items")
		}
	})

	t.Run("List Recipes", func(t *testing.T) {
		recipes := requestJSONAuth[api.PaginatedResponse[models.Recipe]](t, server, auth.AccessToken, http.MethodGet, "/v1/recipes", nil, http.StatusOK).Data
		if len(recipes) != 1 {
			t.Fatalf("expected 1 recipe")
		}
	})

	t.Run("Update Recipe", func(t *testing.T) {
		updated := requestJSONAuth[models.Recipe](t, server, auth.AccessToken, http.MethodPatch, "/v1/recipes/"+recipe.ID.String(), map[string]any{
			"servings": 2,
		}, http.StatusOK)
		if updated.Servings != 2 {
			t.Fatalf("expected servings 2")
		}
		// update current reference for further tests
		recipe = updated
	})

	t.Run("Recipe Nutrition", func(t *testing.T) {
		nutrition := requestJSONAuth[map[string]float64](t, server, auth.AccessToken, http.MethodGet, "/v1/recipes/"+recipe.ID.String()+"/nutrition", nil, http.StatusOK)

		// 200g flour = 728 cal, 2 eggs = 140 cal. Total = 868 cal.
		// Servings = 2. So 434 cal per serving
		if nutrition["calories"] != 434 {
			t.Fatalf("expected 434 calories per serving, got %f", nutrition["calories"])
		}

		// Protein = 20g (flour) + 12g (egg) = 32g. / 2 servings = 16g
		if nutrition["protein"] != 16 {
			t.Fatalf("expected 16g protein, got %f", nutrition["protein"])
		}
	})

	t.Run("Log Recipe to Meal", func(t *testing.T) {
		todayStr := time.Now().UTC().Format("2006-01-02")
		meal := requestJSONAuth[models.Meal](t, server, auth.AccessToken, http.MethodPost, "/v1/recipes/"+recipe.ID.String()+"/log-to-meal", map[string]any{
			"date":      todayStr,
			"meal_type": "breakfast",
			"servings":  1, // log 1 serving out of the 2 recipe servings
		}, http.StatusCreated)

		if meal.MealType != "breakfast" {
			t.Fatalf("expected breakfast")
		}
		if len(meal.Items) != 2 {
			t.Fatalf("expected 2 items logged")
		}

		// The logged quantity should be (Requested / RecipeServings) = 1 / 2 = 0.5 ratio of the original item amounts.
		// Flour was 2.0 -> should be 1.0
		// Egg was 2.0 -> should be 1.0
		for _, item := range meal.Items {
			if item.Quantity != 1.0 {
				t.Fatalf("expected item quantity 1.0 after scaling, got %f", item.Quantity)
			}
		}
	})

	t.Run("Delete Recipe", func(t *testing.T) {
		expectStatusAuth(t, server, auth.AccessToken, http.MethodDelete, "/v1/recipes/"+recipe.ID.String(), nil, http.StatusNoContent)
		expectStatusAuth(t, server, auth.AccessToken, http.MethodGet, "/v1/recipes/"+recipe.ID.String(), nil, http.StatusNotFound)
	})
}
