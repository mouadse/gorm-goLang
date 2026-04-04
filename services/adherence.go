package services

import (
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AdherenceService provides business logic for user streaks and activity calendar.
type AdherenceService struct {
	db *gorm.DB
}

// NewAdherenceService creates a new adherence service.
func NewAdherenceService(db *gorm.DB) *AdherenceService {
	return &AdherenceService{db: db}
}

// Streaks holds the streak data for a user.
type Streaks struct {
	WorkoutStreak int `json:"workout_streak"` // consecutive weeks with a workout
	MealStreak    int `json:"meal_streak"`    // consecutive days with a meal
	WeighInStreak int `json:"weigh_in_streak"`// consecutive days with a weigh-in
}

// AdherenceSummary holds adherence percentages for different periods.
type AdherenceSummary struct {
	Days7  float64 `json:"days_7"`
	Days30 float64 `json:"days_30"`
	Days90 float64 `json:"days_90"`
}

// StreaksResponse combines streaks and summaries.
type StreaksResponse struct {
	Streaks          Streaks          `json:"streaks"`
	AdherenceSummary AdherenceSummary `json:"adherence_summary"`
}

// GetUserStreaks calculates current streaks and adherence percentages based on historical data.
func (s *AdherenceService) GetUserStreaks(userID uuid.UUID, today time.Time) (*StreaksResponse, error) {
	todayStr := startOfDayUTC(today)

	// Fetch all distinct meal dates
	var mealDates []time.Time
	if err := s.db.Model(&models.Meal{}).
		Where("user_id = ?", userID).
		Order("date desc").
		Pluck("DISTINCT date", &mealDates).Error; err != nil {
		return nil, err
	}

	// Fetch all distinct workout dates
	var workoutDates []time.Time
	if err := s.db.Model(&models.Workout{}).
		Where("user_id = ?", userID).
		Order("date desc").
		Pluck("DISTINCT date", &workoutDates).Error; err != nil {
		return nil, err
	}

	// Fetch all distinct weight entry dates
	var weightDates []time.Time
	if err := s.db.Model(&models.WeightEntry{}).
		Where("user_id = ?", userID).
		Order("date desc").
		Pluck("DISTINCT date", &weightDates).Error; err != nil {
		return nil, err
	}

	mealStreak := calculateDailyStreak(mealDates, todayStr)
	weightStreak := calculateDailyStreak(weightDates, todayStr)
	workoutStreak := calculateWeeklyStreak(workoutDates, todayStr)

	// Combine meal, workout, and weight dates for adherence
	activityDatesMap := make(map[time.Time]bool)
	for _, d := range mealDates {
		activityDatesMap[startOfDayUTC(d)] = true
	}
	for _, d := range workoutDates {
		activityDatesMap[startOfDayUTC(d)] = true
	}
	for _, d := range weightDates {
		activityDatesMap[startOfDayUTC(d)] = true
	}

	var allActivityDates []time.Time
	for d := range activityDatesMap {
		allActivityDates = append(allActivityDates, d)
	}

	return &StreaksResponse{
		Streaks: Streaks{
			WorkoutStreak: workoutStreak,
			MealStreak:    mealStreak,
			WeighInStreak: weightStreak,
		},
		AdherenceSummary: AdherenceSummary{
			Days7:  calculateAdherence(allActivityDates, todayStr, 7),
			Days30: calculateAdherence(allActivityDates, todayStr, 30),
			Days90: calculateAdherence(allActivityDates, todayStr, 90),
		},
	}, nil
}

// GetActivityCalendar returns a map of dates to activity types within a date range.
func (s *AdherenceService) GetActivityCalendar(userID uuid.UUID, start, end time.Time) (map[string][]string, error) {
	start = startOfDayUTC(start)
	end = startOfDayUTC(end).Add(24 * time.Hour).Add(-time.Nanosecond)

	var meals []models.Meal
	if err := s.db.Where("user_id = ? AND date BETWEEN ? AND ?", userID, start, end).Select("date").Find(&meals).Error; err != nil {
		return nil, err
	}

	var workouts []models.Workout
	if err := s.db.Where("user_id = ? AND date BETWEEN ? AND ?", userID, start, end).Select("date").Find(&workouts).Error; err != nil {
		return nil, err
	}

	var weights []models.WeightEntry
	if err := s.db.Where("user_id = ? AND date BETWEEN ? AND ?", userID, start, end).Select("date").Find(&weights).Error; err != nil {
		return nil, err
	}

	calendar := make(map[string][]string)
	
	addActivity := func(date time.Time, activity string) {
		dStr := date.Format("2006-01-02")
		for _, a := range calendar[dStr] {
			if a == activity {
				return
			}
		}
		calendar[dStr] = append(calendar[dStr], activity)
	}

	for _, m := range meals {
		addActivity(m.Date, "meal")
	}
	for _, w := range workouts {
		addActivity(w.Date, "workout")
	}
	for _, we := range weights {
		addActivity(we.Date, "weight_entry")
	}

	return calendar, nil
}

func calculateDailyStreak(dates []time.Time, today time.Time) int {
	dateMap := make(map[time.Time]bool)
	for _, d := range dates {
		dateMap[startOfDayUTC(d)] = true
	}

	streak := 0
	current := today
	// Check today first
	if dateMap[current] {
		streak++
		current = current.AddDate(0, 0, -1)
	} else {
		// If not today, maybe yesterday
		current = current.AddDate(0, 0, -1)
		if dateMap[current] {
			streak++
			current = current.AddDate(0, 0, -1)
		} else {
			return 0
		}
	}

	for {
		if dateMap[current] {
			streak++
			current = current.AddDate(0, 0, -1)
		} else {
			break
		}
	}
	return streak
}

func calculateWeeklyStreak(dates []time.Time, today time.Time) int {
	weekMap := make(map[time.Time]bool)
	for _, d := range dates {
		weekMap[startOfWeekUTC(d)] = true
	}

	streak := 0
	currentWeek := startOfWeekUTC(today)

	// Check this week
	if weekMap[currentWeek] {
		streak++
		currentWeek = currentWeek.AddDate(0, 0, -7)
	} else {
		// If not this week, maybe last week
		currentWeek = currentWeek.AddDate(0, 0, -7)
		if weekMap[currentWeek] {
			streak++
			currentWeek = currentWeek.AddDate(0, 0, -7)
		} else {
			return 0
		}
	}

	for {
		if weekMap[currentWeek] {
			streak++
			currentWeek = currentWeek.AddDate(0, 0, -7)
		} else {
			break
		}
	}

	return streak
}

func calculateAdherence(dates []time.Time, today time.Time, days int) float64 {
	if days == 0 {
		return 0
	}
	dateMap := make(map[time.Time]bool)
	for _, d := range dates {
		dateMap[startOfDayUTC(d)] = true
	}

	count := 0
	start := today.AddDate(0, 0, -days+1) // Include today

	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		if dateMap[d] {
			count++
		}
	}

	return float64(count) / float64(days) * 100
}
