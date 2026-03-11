package main

import (
	"log"
	"math/rand"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	source := rand.New(rand.NewSource(42))

	log.Println("starting lean database seeding...")

	exercises, err := seedExercises(db)
	if err != nil {
		log.Fatalf("failed to seed exercises: %v", err)
	}

	users, err := seedUsers(db)
	if err != nil {
		log.Fatalf("failed to seed users: %v", err)
	}

	if err := seedWorkouts(db, source, users, exercises); err != nil {
		log.Fatalf("failed to seed workouts: %v", err)
	}

	if err := seedMeals(db, source, users); err != nil {
		log.Fatalf("failed to seed meals: %v", err)
	}

	if err := seedWeightEntries(db, users); err != nil {
		log.Fatalf("failed to seed weight entries: %v", err)
	}

	log.Println("lean database seeding completed successfully")
	log.Println("seeded data summary:")
	log.Println("  - Users: 4")
	log.Println("  - Exercises: 8")
	log.Println("  - Workouts: 8")
	log.Println("  - Meals: 12")
	log.Println("  - Weight entries: 16")
}

func seedExercises(db *gorm.DB) ([]models.Exercise, error) {
	log.Println("  seeding exercises...")

	seeds := []models.Exercise{
		{Name: "Bench Press", MuscleGroup: "Chest", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Press the bar from chest to lockout.", VideoURL: "https://www.youtube.com/watch?v=gRVjAtPip0Y"},
		{Name: "Back Squat", MuscleGroup: "Legs", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Squat to depth while keeping the torso braced.", VideoURL: "https://www.youtube.com/watch?v=ultWZbUMPL8"},
		{Name: "Deadlift", MuscleGroup: "Back", Equipment: "Barbell", Difficulty: "Advanced", Instructions: "Drive through the floor and stand tall with the bar.", VideoURL: "https://www.youtube.com/watch?v=op9kVnSso6Q"},
		{Name: "Pull-Up", MuscleGroup: "Back", Equipment: "Bodyweight", Difficulty: "Intermediate", Instructions: "Pull until the chin clears the bar.", VideoURL: "https://www.youtube.com/watch?v=eGo4IYlbE5g"},
		{Name: "Overhead Press", MuscleGroup: "Shoulders", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Press vertically from shoulder rack position.", VideoURL: "https://www.youtube.com/watch?v=2yjwXTZQDDI"},
		{Name: "Dumbbell Row", MuscleGroup: "Back", Equipment: "Dumbbell", Difficulty: "Beginner", Instructions: "Row toward the hip while staying square.", VideoURL: "https://www.youtube.com/watch?v=pYcpY20QaE8"},
		{Name: "Romanian Deadlift", MuscleGroup: "Hamstrings", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Hinge at the hips and keep the bar close.", VideoURL: "https://www.youtube.com/watch?v=JC5UYl3qPTs"},
		{Name: "Walking Lunge", MuscleGroup: "Legs", Equipment: "Bodyweight", Difficulty: "Beginner", Instructions: "Step forward and control the knee to the floor.", VideoURL: "https://www.youtube.com/watch?v=QOVaHwm-Q6U"},
	}

	exercises := make([]models.Exercise, 0, len(seeds))
	for _, seed := range seeds {
		var exercise models.Exercise
		if err := db.Where("name = ?", seed.Name).Assign(seed).FirstOrCreate(&exercise).Error; err != nil {
			return nil, err
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}

func seedUsers(db *gorm.DB) ([]models.User, error) {
	log.Println("  seeding users...")

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create birth dates for users
	alexDOB := time.Date(1992, 3, 15, 0, 0, 0, 0, time.UTC)
	sarahDOB := time.Date(1995, 7, 22, 0, 0, 0, 0, time.UTC)
	mikeDOB := time.Date(1988, 11, 8, 0, 0, 0, 0, time.UTC)
	emilyDOB := time.Date(1998, 1, 30, 0, 0, 0, 0, time.UTC)

	seeds := []models.User{
		{Email: "alex@example.com", PasswordHash: string(passwordHash), Name: "Alex Johnson", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=alex", Goal: "build_muscle", ActivityLevel: "moderately_active", Weight: 78, Height: 181, TDEE: 2600, DateOfBirth: &alexDOB, Age: ageFromDateOfBirth(alexDOB)},
		{Email: "sarah@example.com", PasswordHash: string(passwordHash), Name: "Sarah Williams", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=sarah", Goal: "lose_fat", ActivityLevel: "lightly_active", Weight: 64, Height: 165, TDEE: 1850, DateOfBirth: &sarahDOB, Age: ageFromDateOfBirth(sarahDOB)},
		{Email: "mike@example.com", PasswordHash: string(passwordHash), Name: "Mike Chen", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=mike", Goal: "maintain", ActivityLevel: "active", Weight: 82, Height: 178, TDEE: 2750, DateOfBirth: &mikeDOB, Age: ageFromDateOfBirth(mikeDOB)},
		{Email: "emily@example.com", PasswordHash: string(passwordHash), Name: "Emily Davis", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=emily", Goal: "build_muscle", ActivityLevel: "moderately_active", Weight: 59, Height: 163, TDEE: 2100, DateOfBirth: &emilyDOB, Age: ageFromDateOfBirth(emilyDOB)},
	}

	users := make([]models.User, 0, len(seeds))
	for _, seed := range seeds {
		var user models.User
		if err := db.Where("email = ?", seed.Email).Assign(seed).FirstOrCreate(&user).Error; err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func seedWorkouts(db *gorm.DB, source *rand.Rand, users []models.User, exercises []models.Exercise) error {
	log.Println("  seeding workouts...")

	baseDate := time.Now().UTC().Truncate(24 * time.Hour)
	workoutTypes := []string{"push", "pull", "legs", "full_body"}

	for index, user := range users {
		for offset := 0; offset < 2; offset++ {
			workoutDate := baseDate.AddDate(0, 0, -(index*3 + offset))
			workout := models.Workout{
				UserID:   user.ID,
				Date:     workoutDate,
				Duration: 45 + source.Intn(30),
				Notes:    "Seeded training session",
				Type:     workoutTypes[(index+offset)%len(workoutTypes)],
			}

			var persistedWorkout models.Workout
			if err := db.Where("user_id = ? AND date = ? AND type = ?", user.ID, workoutDate, workout.Type).
				Assign(workout).
				FirstOrCreate(&persistedWorkout).Error; err != nil {
				return err
			}
			workout = persistedWorkout

			for exerciseIndex := 0; exerciseIndex < 3; exerciseIndex++ {
				exercise := exercises[(index+offset+exerciseIndex)%len(exercises)]
				workoutExercise := models.WorkoutExercise{
					WorkoutID:  workout.ID,
					ExerciseID: exercise.ID,
					Order:      exerciseIndex + 1,
					Sets:       3,
					Reps:       6 + source.Intn(6),
					Weight:     float64(20 + source.Intn(60)),
					RestTime:   60 + source.Intn(60),
					Notes:      "",
				}

				var persistedWorkoutExercise models.WorkoutExercise
				if err := db.Where(&models.WorkoutExercise{
					WorkoutID:  workout.ID,
					ExerciseID: exercise.ID,
					Order:      exerciseIndex + 1,
				}).Assign(workoutExercise).FirstOrCreate(&persistedWorkoutExercise).Error; err != nil {
					return err
				}
				workoutExercise = persistedWorkoutExercise

				for setNumber := 1; setNumber <= workoutExercise.Sets; setNumber++ {
					workoutSet := models.WorkoutSet{
						WorkoutExerciseID: workoutExercise.ID,
						SetNumber:         setNumber,
						Reps:              workoutExercise.Reps,
						Weight:            workoutExercise.Weight,
						RPE:               6.5 + float64(source.Intn(4)),
						RestSeconds:       workoutExercise.RestTime,
						Completed:         true,
					}

					if err := db.Where("workout_exercise_id = ? AND set_number = ?", workoutExercise.ID, setNumber).
						Assign(workoutSet).
						FirstOrCreate(&models.WorkoutSet{}).Error; err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func seedMeals(db *gorm.DB, source *rand.Rand, users []models.User) error {
	log.Println("  seeding meals...")

	baseDate := time.Now().UTC().Truncate(24 * time.Hour)
	mealTypes := []string{"breakfast", "lunch", "dinner"}
	mealNotes := []string{
		"Protein-forward meal",
		"Quick recovery meal",
		"Simple whole-food meal",
	}

	for index, user := range users {
		for dayOffset := 0; dayOffset < 3; dayOffset++ {
			mealDate := baseDate.AddDate(0, 0, -dayOffset)
			meal := models.Meal{
				UserID:   user.ID,
				MealType: mealTypes[(index+dayOffset)%len(mealTypes)],
				Date:     mealDate,
				Notes:    mealNotes[source.Intn(len(mealNotes))],
			}

			if err := db.Where("user_id = ? AND date = ? AND meal_type = ?", user.ID, mealDate, meal.MealType).
				Assign(meal).
				FirstOrCreate(&models.Meal{}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func seedWeightEntries(db *gorm.DB, users []models.User) error {
	log.Println("  seeding weight entries...")

	baseDate := time.Now().UTC().Truncate(24 * time.Hour)

	for index, user := range users {
		for weekOffset := 0; weekOffset < 4; weekOffset++ {
			entryDate := baseDate.AddDate(0, 0, -7*weekOffset)
			entry := models.WeightEntry{
				UserID: user.ID,
				Weight: user.Weight + float64(index-weekOffset)/4,
				Date:   entryDate,
				Notes:  "Weekly check-in",
			}

			if err := db.Where("user_id = ? AND date = ?", user.ID, entryDate).
				Assign(entry).
				FirstOrCreate(&models.WeightEntry{}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func ageFromDateOfBirth(dateOfBirth time.Time) int {
	now := time.Now().UTC()
	age := now.Year() - dateOfBirth.Year()
	if now.Month() < dateOfBirth.Month() || (now.Month() == dateOfBirth.Month() && now.Day() < dateOfBirth.Day()) {
		age--
	}
	if age < 0 {
		return 0
	}
	return age
}
