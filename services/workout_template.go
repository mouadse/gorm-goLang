package services

import (
	"errors"
	"time"

	"fitness-tracker/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorkoutTemplateService struct {
	db *gorm.DB
}

func NewWorkoutTemplateService(db *gorm.DB) *WorkoutTemplateService {
	return &WorkoutTemplateService{db: db}
}

func (s *WorkoutTemplateService) ApplyTemplate(templateID uuid.UUID, userID uuid.UUID, workoutDate time.Time) (*models.Workout, error) {
	var template models.WorkoutTemplate
	err := s.db.Preload("WorkoutTemplateExercises.Exercise").
		Preload("WorkoutTemplateExercises.WorkoutTemplateSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		}).
		Preload("WorkoutTemplateExercises", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc")
		}).
		First(&template, "id = ?", templateID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, err
	}

	var workout models.Workout
	err = s.db.Transaction(func(tx *gorm.DB) error {
		workout = models.Workout{
			UserID: userID,
			Date:   workoutDate,
			Type:   template.Type,
			Notes:  template.Notes,
		}

		if err := tx.Create(&workout).Error; err != nil {
			return err
		}

		for _, templateExercise := range template.WorkoutTemplateExercises {
			workoutExercise := models.WorkoutExercise{
				WorkoutID:  workout.ID,
				ExerciseID: templateExercise.ExerciseID,
				Order:      templateExercise.Order,
				Sets:       templateExercise.Sets,
				Reps:       templateExercise.Reps,
				Weight:     templateExercise.Weight,
				RestTime:   templateExercise.RestTime,
				Notes:      templateExercise.Notes,
			}

			if err := tx.Create(&workoutExercise).Error; err != nil {
				return err
			}

			for _, templateSet := range templateExercise.WorkoutTemplateSets {
				workoutSet := models.WorkoutSet{
					WorkoutExerciseID: workoutExercise.ID,
					SetNumber:         templateSet.SetNumber,
					Reps:              templateSet.Reps,
					Weight:            templateSet.Weight,
					RestSeconds:       templateSet.RestSeconds,
					Completed:         true,
				}

				if err := tx.Create(&workoutSet).Error; err != nil {
					return err
				}
			}

			workout.WorkoutExercises = append(workout.WorkoutExercises, workoutExercise)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var loadedWorkout models.Workout
	err = s.db.Preload("WorkoutExercises.Exercise").
		Preload("WorkoutExercises.WorkoutSets", func(db *gorm.DB) *gorm.DB {
			return db.Order("set_number asc")
		}).
		Preload("WorkoutExercises", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" asc")
		}).
		First(&loadedWorkout, "id = ?", workout.ID).Error
	if err != nil {
		return nil, err
	}

	return &loadedWorkout, nil
}
