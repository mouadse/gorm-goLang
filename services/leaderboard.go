package services

import (
	"fitness-tracker/models"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LeaderboardService struct {
	db *gorm.DB
}

func NewLeaderboardService(db *gorm.DB) *LeaderboardService {
	return &LeaderboardService{db: db}
}

func (s *LeaderboardService) AwardPoints(userID uuid.UUID, points int, reason, reasonCode, pillar string, sourceID *uuid.UUID, earnedAt time.Time) error {
	log := models.UserPointsLog{
		UserID:         userID,
		Points:         points,
		Reason:         reason,
		ReasonCode:     reasonCode,
		Pillar:         pillar,
		SourceEntityID: sourceID,
		EarnedAt:       earnedAt,
	}
	return s.db.Create(&log).Error
}

func (s *LeaderboardService) RevokePoints(sourceID uuid.UUID, reasonCode string) error {
	return s.db.Where("source_entity_id = ? AND reason_code = ?", sourceID, reasonCode).Delete(&models.UserPointsLog{}).Error
}

type LeaderboardResponse struct {
	Period  string             `json:"period"`
	Pillar  string             `json:"pillar"`
	Entries []LeaderboardEntry `json:"entries"`
	Total   int                `json:"total"`
	Offset  int                `json:"offset"`
	Limit   int                `json:"limit"`
}

type LeaderboardEntry struct {
	Rank             int       `json:"rank"`
	UserID           uuid.UUID `json:"user_id"`
	UserName         string    `json:"user_name"`
	Avatar           string    `json:"avatar"`
	Score            float64   `json:"score"`
	TrainingScore    float64   `json:"training_score"`
	NutritionScore   float64   `json:"nutrition_score"`
	ConsistencyScore float64   `json:"consistency_score"`
	Breakdown        Breakdown `json:"breakdown"`
}

type Breakdown struct {
	Workouts             int `json:"workouts"`
	MealsLogged          int `json:"meals_logged"`
	PerfectNutritionDays int `json:"perfect_nutrition_days"`
	All3Days             int `json:"all_3_days"`
	StreakDays           int `json:"streak_days"`
}

func (s *LeaderboardService) GetLeaderboard(period, pillar string, offset, limit int, now time.Time) (*LeaderboardResponse, error) {
	// 1. Fetch relevant points
	var logs []models.UserPointsLog
	query := s.db.Model(&models.UserPointsLog{}).
		Joins("JOIN users ON users.id = user_points_logs.user_id AND users.deleted_at IS NULL").
		Preload("User")

	var lambda float64
	var minDate time.Time

	switch period {
	case "weekly":
		lambda = 0.23
		minDate = now.AddDate(0, 0, -14) // Don't fetch older than 14 days for weekly to save memory
	case "monthly":
		lambda = 0.05
		minDate = now.AddDate(0, -2, 0)
	case "yearly":
		lambda = 0.005
		minDate = now.AddDate(-2, 0, 0)
	case "alltime":
		lambda = 0
		// Fetch all
	default:
		lambda = 0.23 // default to weekly
		period = "weekly"
		minDate = now.AddDate(0, 0, -14)
	}

	if !minDate.IsZero() {
		query = query.Where("earned_at >= ?", minDate)
	}

	if pillar != "all" && pillar != "" {
		query = query.Where("pillar = ?", pillar)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	// 2. Aggregate and calculate scores
	type UserData struct {
		User             models.User
		TotalScore       float64
		TrainingScore    float64
		NutritionScore   float64
		ConsistencyScore float64
		RawPoints        int
		FirstActivity    time.Time
		LastActivity     time.Time
		ActiveDaysMap    map[string]bool
	}

	userMap := make(map[uuid.UUID]*UserData)

	for _, log := range logs {
		ud, ok := userMap[log.UserID]
		if !ok {
			ud = &UserData{
				User:          log.User,
				ActiveDaysMap: make(map[string]bool),
				FirstActivity: log.EarnedAt,
			}
			userMap[log.UserID] = ud
		}

		if log.EarnedAt.Before(ud.FirstActivity) {
			ud.FirstActivity = log.EarnedAt
		}
		if log.EarnedAt.After(ud.LastActivity) {
			ud.LastActivity = log.EarnedAt
		}

		dateStr := log.EarnedAt.Format("2006-01-02")
		ud.ActiveDaysMap[dateStr] = true
		ud.RawPoints += log.Points

		var score float64
		if period == "alltime" {
			// handled later
			score = float64(log.Points)
		} else {
			ageDays := now.Sub(log.EarnedAt).Hours() / 24.0
			if ageDays < 0 {
				ageDays = 0
			}
			weight := math.Exp(-lambda * ageDays)
			score = float64(log.Points) * weight
		}

		ud.TotalScore += score
		switch log.Pillar {
		case models.PillarTraining:
			ud.TrainingScore += score
		case models.PillarNutrition:
			ud.NutritionScore += score
		case models.PillarConsistency:
			ud.ConsistencyScore += score
		}
	}

	// Calculate all-time specific logic
	if period == "alltime" {
		for _, ud := range userMap {
			activeDays := float64(len(ud.ActiveDaysMap))
			daysSinceFirst := now.Sub(ud.FirstActivity).Hours() / 24.0
			if daysSinceFirst < 1 {
				daysSinceFirst = 1
			}
			consistencySchedule := activeDays / daysSinceFirst
			if consistencySchedule > 1 {
				consistencySchedule = 1
			}

			multiplier := 0.5 + 0.5*consistencySchedule
			ud.TotalScore = math.Sqrt(float64(ud.RawPoints)) * multiplier
			
			// Re-distribute scores relative to raw points (simple proportion)
			if ud.RawPoints > 0 {
			    ud.TrainingScore = ud.TotalScore * (ud.TrainingScore / float64(ud.RawPoints))
			    ud.NutritionScore = ud.TotalScore * (ud.NutritionScore / float64(ud.RawPoints))
			    ud.ConsistencyScore = ud.TotalScore * (ud.ConsistencyScore / float64(ud.RawPoints))
			}
		}
	}

	var entries []LeaderboardEntry
	for id, ud := range userMap {
		entries = append(entries, LeaderboardEntry{
			UserID:           id,
			UserName:         ud.User.Name,
			Avatar:           ud.User.Avatar,
			Score:            ud.TotalScore,
			TrainingScore:    ud.TrainingScore,
			NutritionScore:   ud.NutritionScore,
			ConsistencyScore: ud.ConsistencyScore,
			Breakdown:        Breakdown{}, // To be populated if needed
		})
	}

	// 3. Sort
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	// 4. Assign Ranks and Paginate
	total := len(entries)
	
	// Apply pagination
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	
	pagedEntries := entries[start:end]
	for i := range pagedEntries {
		pagedEntries[i].Rank = start + i + 1
	}

	return &LeaderboardResponse{
		Period:  period,
		Pillar:  pillar,
		Entries: pagedEntries,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
	}, nil
}
