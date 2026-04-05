package api_test

import (
	"net/http"
	"testing"
	"time"

	"fitness-tracker/models"
)

func TestMealReuseHandlers(t *testing.T) {
	_, server := newTestApp(t)

	auth := registerTestUser(t, server, "mealreuse@example.com", "Meal Reuse", "password123")
	user := auth.User

	// Create some foods manually
	food1 := requestJSONAuth[models.Food](t, server, auth.AccessToken, http.MethodPost, "/v1/foods", map[string]any{
		"name":          "Oats",
		"serving_size":  100,
		"serving_unit":  "g",
		"calories":      389,
		"protein":       16.9,
		"carbohydrates": 66.3,
		"fat":           6.9,
	}, http.StatusCreated)

	food2 := requestJSONAuth[models.Food](t, server, auth.AccessToken, http.MethodPost, "/v1/foods", map[string]any{
		"name":          "Whey Protein",
		"serving_size":  30,
		"serving_unit":  "g",
		"calories":      120,
		"protein":       24,
		"carbohydrates": 3,
		"fat":           1.5,
	}, http.StatusCreated)

	// Create a meal manually yesterday
	mealDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	meal := requestJSONAuth[models.Meal](t, server, auth.AccessToken, http.MethodPost, "/v1/meals", map[string]any{
		"user_id":   user.ID,
		"meal_type": "breakfast",
		"date":      mealDate,
		"notes":     "Pre-workout oats",
	}, http.StatusCreated)

	requestJSONAuth[models.MealFood](t, server, auth.AccessToken, http.MethodPost, "/v1/meals/"+meal.ID.String()+"/foods", map[string]any{
		"food_id":  food1.ID,
		"quantity": 0.8,
	}, http.StatusCreated)

	requestJSONAuth[models.MealFood](t, server, auth.AccessToken, http.MethodPost, "/v1/meals/"+meal.ID.String()+"/foods", map[string]any{
		"food_id":  food2.ID,
		"quantity": 1.5,
	}, http.StatusCreated)

	t.Run("Get Recent Meals", func(t *testing.T) {
		meals := requestJSONAuth[[]models.Meal](t, server, auth.AccessToken, http.MethodGet, "/v1/meals/recent?days=7", nil, http.StatusOK)
		if len(meals) != 1 {
			t.Fatalf("expected 1 recent meal, got %d", len(meals))
		}
		if meals[0].ID != meal.ID {
			t.Fatalf("expected meal id %s, got %s", meal.ID, meals[0].ID)
		}
		if len(meals[0].Items) != 2 {
			t.Fatalf("expected 2 meal foods preloaded")
		}
	})

	t.Run("Clone Meal", func(t *testing.T) {
		cloned := requestJSONAuth[models.Meal](t, server, auth.AccessToken, http.MethodPost, "/v1/meals/"+meal.ID.String()+"/clone", nil, http.StatusCreated)
		
		if cloned.ID == meal.ID {
			t.Fatalf("expected cloned meal to have different ID")
		}
		if len(cloned.Items) != 2 {
			t.Fatalf("expected 2 meal foods mapped in cloned meal, got %d", len(cloned.Items))
		}
		if cloned.Date.Format("2006-01-02") != time.Now().UTC().Format("2006-01-02") {
			t.Fatalf("expected cloned meal date to be today")
		}
	})

	t.Run("Get Recent Foods", func(t *testing.T) {
		recentFoods := requestJSONAuth[[]models.Food](t, server, auth.AccessToken, http.MethodGet, "/v1/foods/recent", nil, http.StatusOK)
		
		if len(recentFoods) != 2 {
			t.Fatalf("expected 2 recent foods, got %d", len(recentFoods))
		}
		
		// Because food2 was added after food1, it might be first in recent sorting
		found1 := false
		found2 := false
		for _, f := range recentFoods {
			if f.ID == food1.ID { found1 = true }
			if f.ID == food2.ID { found2 = true }
		}
		if !found1 || !found2 {
			t.Fatalf("failed to find both recent foods")
		}
	})

	t.Run("Favorite and Get Favorites", func(t *testing.T) {
		fav := requestJSONAuth[models.FavoriteFood](t, server, auth.AccessToken, http.MethodPost, "/v1/foods/"+food1.ID.String()+"/favorite", nil, http.StatusCreated)
		if fav.FoodID != food1.ID {
			t.Fatalf("expected favored food1")
		}

		// Conflict test
		expectStatusAuth(t, server, auth.AccessToken, http.MethodPost, "/v1/foods/"+food1.ID.String()+"/favorite", nil, http.StatusConflict)

		favs := requestJSONAuth[[]models.FavoriteFood](t, server, auth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/favorites", nil, http.StatusOK)
		if len(favs) != 1 {
			t.Fatalf("expected 1 favored food, got %d", len(favs))
		}
		if favs[0].FoodID != food1.ID {
			t.Fatalf("expected favored food in list to be food1")
		}

		expectStatusAuth(t, server, auth.AccessToken, http.MethodDelete, "/v1/foods/"+food1.ID.String()+"/favorite", nil, http.StatusNoContent)

		favsAfterDelete := requestJSONAuth[[]models.FavoriteFood](t, server, auth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/favorites", nil, http.StatusOK)
		if len(favsAfterDelete) != 0 {
			t.Fatalf("expected 0 favored foods after deletion, got %d", len(favsAfterDelete))
		}
	})
}
