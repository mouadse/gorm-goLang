package api

import (
	"encoding/json"
	"fitness-tracker/database"
	"fitness-tracker/models"
	"fitness-tracker/services"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLeaderboardHandlersTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}

	return db
}

func TestLeaderboardHandlers(t *testing.T) {
	db := setupLeaderboardHandlersTestDB(t)

	server := NewServer(db)

	user := models.User{
		ID:           uuid.New(),
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "x",
	}
	db.Create(&user)

	// Create session token so we can authenticate
	authSvc := services.NewAuthService(db)
	token, _ := authSvc.CreateSession(user.ID, "user agent", "127.0.0.1")

	server.leaderboardSvc.AwardPoints(user.ID, 150, "Test", "T1", models.PillarTraining, nil, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboard?period=weekly", nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("response: %s", rr.Body.String())
	}

	var res services.LeaderboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Period != "weekly" {
		t.Errorf("expected period weekly, got %s", res.Period)
	}

	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(res.Entries))
	}

	if res.Entries[0].UserID != user.ID {
		t.Errorf("expected user ID to match, got %v", res.Entries[0].UserID)
	}
}
