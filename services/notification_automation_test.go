package services

import (
	"context"
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNotificationAutomationServiceProcessDueNotifications(t *testing.T) {
	now := time.Date(2026, time.April, 10, 21, 0, 0, 0, time.UTC)
	db := openServicesTestDB(t,
		&models.User{},
		&models.Notification{},
		&models.Meal{},
		&models.MealFood{},
		&models.Food{},
		&models.Workout{},
		&models.WorkoutExercise{},
		&models.WorkoutSet{},
		&models.Exercise{},
	)

	lowProteinUser := createTestUser(t, db)
	lowProteinUser.Goal = "build_muscle"
	if err := db.Save(&lowProteinUser).Error; err != nil {
		t.Fatalf("save low protein user: %v", err)
	}

	food := models.Food{
		Name:          "Small Yogurt",
		ServingSize:   100,
		ServingUnit:   "g",
		Calories:      120,
		Protein:       5,
		Carbohydrates: 15,
		Fat:           4,
	}
	if err := db.Create(&food).Error; err != nil {
		t.Fatalf("create food: %v", err)
	}
	meal := models.Meal{UserID: lowProteinUser.ID, Date: startOfDayUTC(now), MealType: "breakfast"}
	if err := db.Create(&meal).Error; err != nil {
		t.Fatalf("create meal: %v", err)
	}
	if err := db.Create(&models.MealFood{MealID: meal.ID, FoodID: food.ID, Quantity: 1}).Error; err != nil {
		t.Fatalf("create meal food: %v", err)
	}

	exercise := models.Exercise{Name: "Heavy Row", PrimaryMuscles: "Back"}
	if err := db.Create(&exercise).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	workout := models.Workout{UserID: lowProteinUser.ID, Date: startOfDayUTC(now), Type: "pull", Duration: 60}
	if err := db.Create(&workout).Error; err != nil {
		t.Fatalf("create workout: %v", err)
	}
	workoutExercise := models.WorkoutExercise{WorkoutID: workout.ID, ExerciseID: exercise.ID, Order: 1}
	if err := db.Create(&workoutExercise).Error; err != nil {
		t.Fatalf("create workout exercise: %v", err)
	}
	if err := db.Create(&models.WorkoutSet{WorkoutExerciseID: workoutExercise.ID, SetNumber: 1, Reps: 60, Weight: 100, Completed: true}).Error; err != nil {
		t.Fatalf("create workout set: %v", err)
	}

	inactiveUser := models.User{
		Email:         "inactive-notifications@example.com",
		PasswordHash:  "hash",
		Name:          "Inactive User",
		Age:           30,
		Weight:        80,
		Height:        180,
		Goal:          "maintain",
		ActivityLevel: "active",
	}
	if err := db.Create(&inactiveUser).Error; err != nil {
		t.Fatalf("create inactive user: %v", err)
	}

	svc := NewNotificationAutomationService(db)
	created, err := svc.ProcessDueNotifications(context.Background(), now)
	if err != nil {
		t.Fatalf("process due notifications: %v", err)
	}
	if created < 4 {
		t.Fatalf("expected at least 4 notifications, got %d", created)
	}

	requireNotificationType(t, db, lowProteinUser.ID, models.NotificationLowProtein)
	requireNotificationType(t, db, lowProteinUser.ID, models.NotificationRecoveryWarning)
	requireNotificationType(t, db, inactiveUser.ID, models.NotificationMissedMeal)
	requireNotificationType(t, db, inactiveUser.ID, models.NotificationWorkoutReminder)

	createdAgain, err := svc.ProcessDueNotifications(context.Background(), now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("process duplicate window notifications: %v", err)
	}
	if createdAgain != 0 {
		t.Fatalf("expected duplicate window to create 0 notifications, got %d", createdAgain)
	}
}

func requireNotificationType(t *testing.T, db *gorm.DB, userID uuid.UUID, notifType models.NotificationType) {
	t.Helper()

	var count int64
	if err := db.Model(&models.Notification{}).Where("user_id = ? AND type = ?", userID, notifType).Count(&count).Error; err != nil {
		t.Fatalf("count %s notifications: %v", notifType, err)
	}
	if count == 0 {
		t.Fatalf("expected %s notification for user %s", notifType, userID)
	}
}
