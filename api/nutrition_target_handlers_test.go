package api_test

import (
	"net/http"
	"testing"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"
)

func TestGetUserNutritionTargets(t *testing.T) {
	t.Parallel()

	db, server := newTestApp(t)

	// Create user via register
	auth := registerTestUser(t, server, "targets@example.com", "Target User", "password123")
	user := auth.User

	// Update user with specific metrics needed for calculation
	dob := time.Now().AddDate(-30, 0, 0)
	db.Model(&models.User{}).Where("id = ?", user.ID).Updates(models.User{
		DateOfBirth:   &dob,
		Weight:        80,
		Height:        180,
		Goal:          "build_muscle",
		ActivityLevel: "active",
	})

	t.Run("Valid Request Target User", func(t *testing.T) {
		targets := requestJSONAuth[services.NutritionTargets](t, server, auth.AccessToken, http.MethodGet, "/v1/users/"+user.ID.String()+"/nutrition-targets", nil, http.StatusOK)

		if targets.Calories <= 0 {
			t.Fatalf("expected calculated calories > 0, got %d", targets.Calories)
		}
		if targets.Goal != "build_muscle" {
			t.Fatalf("expected goal build_muscle, got %s", targets.Goal)
		}
		if targets.IsOverride {
			t.Fatalf("expected IsOverride false")
		}
		if targets.ActivityLevel != "active" {
			t.Fatalf("expected activity_level active, got %s", targets.ActivityLevel)
		}
	})

	t.Run("TDEE Override", func(t *testing.T) {
		overrideAuth := registerTestUser(t, server, "override@example.com", "Override User", "password123")

		db.Model(&models.User{}).Where("id = ?", overrideAuth.User.ID).Updates(models.User{
			DateOfBirth:   &dob,
			Weight:        80,
			Height:        180,
			TDEE:          3000,
			Goal:          "lose_fat",
			ActivityLevel: "active",
		})

		targets := requestJSONAuth[services.NutritionTargets](t, server, overrideAuth.AccessToken, http.MethodGet, "/v1/users/"+overrideAuth.User.ID.String()+"/nutrition-targets", nil, http.StatusOK)

		if targets.Calories != 3000 {
			t.Fatalf("expected overridden calories 3000, got %d", targets.Calories)
		}
		if !targets.IsOverride {
			t.Fatalf("expected IsOverride true")
		}
	})

	t.Run("Unauthorized cross-user access", func(t *testing.T) {
		otherAuth := registerTestUser(t, server, "other@example.com", "Other User", "password123")

		expectStatusAuth(t, server, auth.AccessToken, http.MethodGet, "/v1/users/"+otherAuth.User.ID.String()+"/nutrition-targets", nil, http.StatusForbidden)
	})
}
