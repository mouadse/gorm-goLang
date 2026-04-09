package services

import (
	"fitness-tracker/models"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLeaderboardTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.UserPointsLog{})
	if err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}
	return db
}

func TestLeaderboardService_AwardPoints(t *testing.T) {
	db := setupLeaderboardTestDB(t)
	s := NewLeaderboardService(db)

	user := models.User{Name: "Test User", Email: "test@example.com", PasswordHash: "x"}
	db.Create(&user)

	err := s.AwardPoints(user.ID, 10, "Workout Logged", "T1", models.PillarTraining, nil, time.Now())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var count int64
	db.Model(&models.UserPointsLog{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 points log, got %d", count)
	}
}

func TestLeaderboardService_GetLeaderboard(t *testing.T) {
	db := setupLeaderboardTestDB(t)
	s := NewLeaderboardService(db)

	now := time.Now()

	u1 := models.User{Name: "User 1", Email: "u1@example.com", PasswordHash: "x"}
	u2 := models.User{Name: "User 2", Email: "u2@example.com", PasswordHash: "x"}
	db.Create(&u1)
	db.Create(&u2)

	// User 1 gets points today
	s.AwardPoints(u1.ID, 100, "Test", "T1", models.PillarTraining, nil, now)
	s.AwardPoints(u1.ID, 50, "Test", "N1", models.PillarNutrition, nil, now)

	// User 2 gets points a few days ago (decayed in weekly)
	s.AwardPoints(u2.ID, 200, "Test", "T1", models.PillarTraining, nil, now.Add(-4*24*time.Hour))

	res, err := s.GetLeaderboard("weekly", "all", 0, 10, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(res.Entries))
	}

	// User 1 has 150 points * 1.0 = 150
	// User 2 has 200 points * e^(-0.23 * 4) = 200 * e^-0.92 = 200 * 0.398 = ~79.7
	if res.Entries[0].UserID != u1.ID {
		t.Errorf("expected user 1 to be first, got %v", res.Entries[0].UserID)
	}
	if res.Entries[1].UserID != u2.ID {
		t.Errorf("expected user 2 to be second, got %v", res.Entries[1].UserID)
	}

	if res.Entries[0].Score <= res.Entries[1].Score {
		t.Errorf("expected user 1 score (%v) to be greater than user 2 score (%v)", res.Entries[0].Score, res.Entries[1].Score)
	}
}
