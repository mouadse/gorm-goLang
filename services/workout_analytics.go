// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"math"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkoutSetData represents a completed set with its performance metrics.
type WorkoutSetData struct {
	Weight    float64 `json:"weight"`
	Reps      int     `json:"reps"`
	RPE       float64 `json:"rpe"`
	Completed bool    `json:"completed"`
}

// WorkoutExerciseSession represents a single exercise within a workout session.
type WorkoutExerciseSession struct {
	ExerciseID   uuid.UUID
	ExerciseName string
	Sets         []WorkoutSetData
}

// WorkoutSession represents a complete workout with all exercises.
type WorkoutSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Date      time.Time
	Duration  int
	Type      string
	Exercises []WorkoutExerciseSession
}

// PersonalRecord represents a user's personal best for a specific metric.
type PersonalRecord struct {
	Type         string    `json:"type"` // heaviest_set, best_1rm, best_volume, best_reps
	ExerciseID   uuid.UUID `json:"exercise_id"`
	ExerciseName string    `json:"exercise_name"`
	Value        float64   `json:"value"` // weight, 1RM, volume, or repsdepending on type
	Date         time.Time `json:"date"`
	WorkoutID    uuid.UUID `json:"workout_id"`
	SetID        uuid.UUID `json:"set_id,omitempty"`
}

// WorkoutStats contains aggregated workout statistics.
type WorkoutStats struct {
	TotalWorkouts      int            `json:"total_workouts"`
	TotalDuration      int            `json:"total_duration"`
	TotalVolume        float64        `json:"total_volume"`
	TotalSets          int            `json:"total_sets"`
	AvgWorkoutDuration float64        `json:"avg_workout_duration"`
	WorkoutTypes       map[string]int `json:"workout_types"`
}

// WeeklyStats contains weekly workout statistics.
type WeeklyStats struct {
	WeekStart     time.Time      `json:"week_start"`
	WeekEnd       time.Time      `json:"week_end"`
	WorkoutCount  int            `json:"workout_count"`
	TotalVolume   float64        `json:"total_volume"`
	TotalDuration int            `json:"total_duration"`
	WorkoutTypes  map[string]int `json:"workout_types"` // push/pull/legs/cardio counts
}

// ExerciseHistoryEntry represents a past session for a specific exercise.
type ExerciseHistoryEntry struct {
	Date         time.Time        `json:"date"`
	WorkoutID    uuid.UUID        `json:"workout_id"`
	WorkoutType  string           `json:"workout_type"`
	Sets         []WorkoutSetData `json:"sets"`
	Volume       float64          `json:"volume"`
	Estimated1RM float64          `json:"estimated_1rm"`
}

// WorkoutAnalyticsService provides business logic for workout analytics.
type WorkoutAnalyticsService struct {
	db *gorm.DB
}

// NewWorkoutAnalyticsService creates a new workout analytics service.
func NewWorkoutAnalyticsService(db *gorm.DB) *WorkoutAnalyticsService {
	return &WorkoutAnalyticsService{db: db}
}

// CalculateEstimated1RM uses the Epley formula to estimate one-rep max.
// Formula: 1RM = weight * (1 + reps/30)
func CalculateEstimated1RM(weight float64, reps int) float64 {
	if reps <= 0 || weight <= 0 {
		return 0
	}
	if reps == 1 {
		return weight
	}
	return weight * (1 + float64(reps)/30.0)
}

// CalculateSetVolume returns the volume (weight * reps) for a completed set.
func CalculateSetVolume(weight float64, reps int) float64 {
	return weight * float64(reps)
}

// CalculateExerciseSessionVolume returns total volume for all completed sets in an exercise session.
func CalculateExerciseSessionVolume(sets []WorkoutSetData) float64 {
	var total float64
	for _, s := range sets {
		if s.Completed {
			total += CalculateSetVolume(s.Weight, s.Reps)
		}
	}
	return total
}

func startOfDayUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func startOfWeekUTC(t time.Time) time.Time {
	dayStart := startOfDayUTC(t)
	return dayStart.AddDate(0, 0, -int(dayStart.Weekday()))
}

// CalculateWorkoutVolume returns total volume for all exercises in a workout.
func CalculateWorkoutVolume(exercises []WorkoutExerciseSession) float64 {
	var total float64
	for _, ex := range exercises {
		total += CalculateExerciseSessionVolume(ex.Sets)
	}
	return total
}

// FindHeaviestSet returns the set with the highest weight.
func FindHeaviestSet(sets []WorkoutSetData) (WorkoutSetData, bool) {
	if len(sets) == 0 {
		return WorkoutSetData{}, false
	}
	var heaviest WorkoutSetData
	var found bool
	for _, s := range sets {
		if s.Completed && s.Weight > heaviest.Weight {
			heaviest = s
			found = true
		}
	}
	return heaviest, found
}

// FindBest1RM returns the set with the highest estimated 1RM.
func FindBest1RM(sets []WorkoutSetData) (float64, bool) {
	if len(sets) == 0 {
		return 0, false
	}
	var best float64
	var found bool
	for _, s := range sets {
		if s.Completed {
			est1RM := CalculateEstimated1RM(s.Weight, s.Reps)
			if est1RM > best {
				best = est1RM
				found = true
			}
		}
	}
	return best, found
}

// FindBestVolumeSet returns the set with the highest volume (weight * reps).
func FindBestVolumeSet(sets []WorkoutSetData) (float64, bool) {
	if len(sets) == 0 {
		return 0, false
	}
	var best float64
	var found bool
	for _, s := range sets {
		if s.Completed {
			vol := CalculateSetVolume(s.Weight, s.Reps)
			if vol > best {
				best = vol
				found = true
			}
		}
	}
	return best, found
}

// FindBestReps returns the highest rep count at any weight.
func FindBestReps(sets []WorkoutSetData) (int, bool) {
	if len(sets) == 0 {
		return 0, false
	}
	var best int
	var found bool
	for _, s := range sets {
		if s.Completed && s.Reps > best {
			best = s.Reps
			found = true
		}
	}
	return best, found
}

// GetUserPersonalRecords retrieves all personal records for a user grouped by exercise.
func (s *WorkoutAnalyticsService) GetUserPersonalRecords(userID uuid.UUID) ([]PersonalRecord, error) {
	// Get all workout exercises with their sets for this user
	var workoutExercises []models.WorkoutExercise
	err := s.db.
		Preload("Workout").
		Preload("Exercise").
		Preload("WorkoutSets").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workouts.user_id = ?", userID).
		Find(&workoutExercises).Error
	if err != nil {
		return nil, err
	}

	// Group by exercise and track records
	exerciseRecords := make(map[uuid.UUID]map[string]PersonalRecord)

	for _, we := range workoutExercises {
		sets := make([]WorkoutSetData, len(we.WorkoutSets))
		for i, s := range we.WorkoutSets {
			sets[i] = WorkoutSetData{
				Weight:    s.Weight,
				Reps:      s.Reps,
				RPE:       s.RPE,
				Completed: s.Completed,
			}
		}

		if _, ok := exerciseRecords[we.ExerciseID]; !ok {
			exerciseRecords[we.ExerciseID] = make(map[string]PersonalRecord)
		}

		// Check for heaviest set
		if heaviest, ok := FindHeaviestSet(sets); ok {
			current, exists := exerciseRecords[we.ExerciseID]["heaviest_set"]
			if !exists || heaviest.Weight > current.Value {
				exerciseRecords[we.ExerciseID]["heaviest_set"] = PersonalRecord{
						Type:         "heaviest_set",
						ExerciseID:   we.ExerciseID,
						ExerciseName: we.Exercise.Name,
						Value:        heaviest.Weight,
						Date:         we.Workout.Date,
						WorkoutID:    we.Workout.ID,
					}
				}
			}

		// Check for best 1RM
		if best1RM, ok := FindBest1RM(sets); ok {
			current, exists := exerciseRecords[we.ExerciseID]["best_1rm"]
			if !exists || best1RM > current.Value {
				exerciseRecords[we.ExerciseID]["best_1rm"] = PersonalRecord{
						Type:         "best_1rm",
						ExerciseID:   we.ExerciseID,
						ExerciseName: we.Exercise.Name,
						Value:        best1RM,
						Date:         we.Workout.Date,
						WorkoutID:    we.Workout.ID,
					}
				}
			}

		// Check for best volume set
		if bestVol, ok := FindBestVolumeSet(sets); ok {
			current, exists := exerciseRecords[we.ExerciseID]["best_volume"]
			if !exists || bestVol > current.Value {
				exerciseRecords[we.ExerciseID]["best_volume"] = PersonalRecord{
						Type:         "best_volume",
						ExerciseID:   we.ExerciseID,
						ExerciseName: we.Exercise.Name,
						Value:        bestVol,
						Date:         we.Workout.Date,
						WorkoutID:    we.Workout.ID,
					}
				}
			}

		// Check for best reps
		if bestReps, ok := FindBestReps(sets); ok {
			current, exists := exerciseRecords[we.ExerciseID]["best_reps"]
			if !exists || float64(bestReps) > current.Value {
				exerciseRecords[we.ExerciseID]["best_reps"] = PersonalRecord{
						Type:         "best_reps",
						ExerciseID:   we.ExerciseID,
						ExerciseName: we.Exercise.Name,
						Value:        float64(bestReps),
						Date:         we.Workout.Date,
						WorkoutID:    we.Workout.ID,
					}
				}
			}
	}

	// Flatten records
	var records []PersonalRecord
	for _, byType := range exerciseRecords {
		for _, pr := range byType {
			records = append(records, pr)
		}
	}
	return records, nil
}

// GetExerciseHistory returns the workout history for a specific exercise.
func (s *WorkoutAnalyticsService) GetExerciseHistory(userID, exerciseID uuid.UUID, limit int) ([]ExerciseHistoryEntry, error) {
	var workoutExercises []models.WorkoutExercise
	query := s.db.
		Preload("Workout").
		Preload("WorkoutSets").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workouts.user_id = ? AND workout_exercises.exercise_id = ?", userID, exerciseID).
		Order("workouts.date desc")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&workoutExercises).Error; err != nil {
		return nil, err
	}

	var history []ExerciseHistoryEntry
	for _, we := range workoutExercises {
		sets := make([]WorkoutSetData, len(we.WorkoutSets))
		for i, s := range we.WorkoutSets {
			sets[i] = WorkoutSetData{
				Weight:    s.Weight,
				Reps:      s.Reps,
				RPE:       s.RPE,
				Completed: s.Completed,
			}
		}

		var best1RM float64
		if val, ok := FindBest1RM(sets); ok {
			best1RM = val
		}

		history = append(history, ExerciseHistoryEntry{
			Date:         we.Workout.Date,
			WorkoutID:    we.Workout.ID,
			WorkoutType:  we.Workout.Type,
			Sets:         sets,
			Volume:       CalculateExerciseSessionVolume(sets),
			Estimated1RM: best1RM,
		})
	}
	return history, nil
}

// GetUserWorkoutStats returns aggregated workout statistics for a user.
func (s *WorkoutAnalyticsService) GetUserWorkoutStats(userID uuid.UUID) (*WorkoutStats, error) {
	var workouts []models.Workout
	if err := s.db.Where("user_id = ?", userID).Find(&workouts).Error; err != nil {
		return nil, err
	}

	stats := &WorkoutStats{
		TotalWorkouts: len(workouts),
		WorkoutTypes:  make(map[string]int),
	}

	for _, w := range workouts {
		stats.TotalDuration += w.Duration
		if w.Type != "" {
			stats.WorkoutTypes[w.Type]++
		}
	}

	if len(workouts) > 0 {
		stats.AvgWorkoutDuration = float64(stats.TotalDuration) / float64(len(workouts))
	}

	// Calculate total sets and volume
	var workoutExercises []models.WorkoutExercise
	err := s.db.
		Preload("WorkoutSets").
		Joins("JOIN workouts ON workouts.id = workout_exercises.workout_id").
		Where("workouts.user_id = ?", userID).
		Find(&workoutExercises).Error
	if err != nil {
		return nil, err
	}

	for _, we := range workoutExercises {
		for _, set := range we.WorkoutSets {
			if set.Completed {
				stats.TotalSets++
				stats.TotalVolume += CalculateSetVolume(set.Weight, set.Reps)
			}
		}
	}

	return stats, nil
}

// GetWeeklyStats returns workout statistics grouped by week.
func (s *WorkoutAnalyticsService) GetWeeklyStats(userID uuid.UUID, weeksBack int) ([]WeeklyStats, error) {
	if weeksBack <= 0 {
		weeksBack = 4
	}

	currentWeekStart := startOfWeekUTC(time.Now().UTC())
	windowStart := currentWeekStart.AddDate(0, 0, -7*(weeksBack-1))
	windowEnd := currentWeekStart.AddDate(0, 0, 7)

	var workouts []models.Workout
	err := s.db.Where("user_id = ? AND date >= ? AND date < ?", userID, windowStart, windowEnd).
		Order("date asc").
		Find(&workouts).Error
	if err != nil {
		return nil, err
	}

	// Group workouts by week
	weeklyMap := make(map[time.Time]*WeeklyStats)

	for _, w := range workouts {
		thisWeekStart := startOfWeekUTC(w.Date)

		if _, ok := weeklyMap[thisWeekStart]; !ok {
			weeklyMap[thisWeekStart] = &WeeklyStats{
				WeekStart:    thisWeekStart,
				WeekEnd:      thisWeekStart.AddDate(0, 0, 6),
				WorkoutTypes: make(map[string]int),
			}
		}

		weeklyMap[thisWeekStart].WorkoutCount++
		weeklyMap[thisWeekStart].TotalDuration += w.Duration
		if w.Type != "" {
			weeklyMap[thisWeekStart].WorkoutTypes[w.Type]++
		}
	}

	// Calculate volume for each week
	var workoutExercises []models.WorkoutExercise
	workoutIDs := make([]uuid.UUID, len(workouts))
	for i, w := range workouts {
		workoutIDs[i] = w.ID
	}

	if len(workoutIDs) > 0 {
		err = s.db.
			Preload("WorkoutSets").
			Where("workout_id IN ?", workoutIDs).
			Find(&workoutExercises).Error
		if err != nil {
			return nil, err
		}

		workoutVolumes := make(map[uuid.UUID]float64)
		for _, we := range workoutExercises {
			for _, set := range we.WorkoutSets {
				if set.Completed {
					workoutVolumes[we.WorkoutID] += CalculateSetVolume(set.Weight, set.Reps)
				}
			}
		}

		for _, w := range workouts {
			thisWeekStart := startOfWeekUTC(w.Date)
			if ws, ok := weeklyMap[thisWeekStart]; ok {
				ws.TotalVolume += workoutVolumes[w.ID]
			}
		}
	}

	// Convert to sorted slice
	var result []WeeklyStats
	for _, ws := range weeklyMap {
		result = append(result, *ws)
	}

	// Sort by week start
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].WeekStart.After(result[j].WeekStart) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// CompareWithPreviousWorkout compares the current workout with the previous workout for the same type.
func (s *WorkoutAnalyticsService) CompareWithPreviousWorkout(userID uuid.UUID, workoutID uuid.UUID) (map[string]interface{}, error) {
	// Get current workout
	var currentWorkout models.Workout
	if err := s.db.
		Preload("WorkoutExercises.Exercise").
		Preload("WorkoutExercises.WorkoutSets").
		First(&currentWorkout, "id = ? AND user_id = ?", workoutID, userID).Error; err != nil {
		return nil, err
	}

	if currentWorkout.Type == "" {
		return map[string]interface{}{
			"current_workout_id":  workoutID,
			"previous_workout_id": nil,
			"comparison":          nil,
		}, nil
	}

	// Find previous workout of same type
	var previousWorkout models.Workout
	err := s.db.
		Where("user_id = ? AND type = ? AND date < ? AND id != ?",
			userID, currentWorkout.Type, currentWorkout.Date, workoutID).
		Order("date desc").
		First(&previousWorkout).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return map[string]interface{}{
				"current_workout_id":  workoutID,
				"previous_workout_id": nil,
				"comparison":          nil,
			}, nil
		}
		return nil, err
	}

	// Load previous workout exercises
	if err := s.db.
		Preload("Exercise").
		Preload("WorkoutSets").
		Where("workout_id = ?", previousWorkout.ID).
		Find(&previousWorkout.WorkoutExercises).Error; err != nil {
		return nil, err
	}

	// Compare by exercise
	comparison := make(map[string]interface{})
	comparison["previous_workout_id"] = previousWorkout.ID
	comparison["previous_date"] = previousWorkout.Date

	currentVolume := make(map[uuid.UUID]float64)
	previousVolume := make(map[uuid.UUID]float64)

	for _, we := range currentWorkout.WorkoutExercises {
		for _, set := range we.WorkoutSets {
			if set.Completed {
				currentVolume[we.ExerciseID] += CalculateSetVolume(set.Weight, set.Reps)
			}
		}
	}

	for _, we := range previousWorkout.WorkoutExercises {
		for _, set := range we.WorkoutSets {
			if set.Completed {
				previousVolume[we.ExerciseID] += CalculateSetVolume(set.Weight, set.Reps)
			}
		}
	}

	exerciseComparison := make([]map[string]interface{}, 0)
	for _, we := range currentWorkout.WorkoutExercises {
		currVol := currentVolume[we.ExerciseID]
		prevVol := previousVolume[we.ExerciseID]
		var change float64
		if prevVol > 0 {
			change = ((currVol - prevVol) / prevVol) * 100
		}
		exerciseComparison = append(exerciseComparison, map[string]interface{}{
			"exercise_id":       we.ExerciseID,
			"exercise_name":     we.Exercise.Name,
			"current_volume":    currVol,
			"previous_volume":   prevVol,
			"volume_change_pct": math.Round(change*100) / 100,
		})
	}
	comparison["exercises"] = exerciseComparison

	return map[string]interface{}{
		"current_workout_id":  workoutID,
		"previous_workout_id": previousWorkout.ID,
		"comparison":          comparison,
	}, nil
}
