package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/models"

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

	log.Println("🌱 Starting database seeding...")

	if err := seedExercises(db); err != nil {
		log.Fatalf("failed to seed exercises: %v", err)
	}

	if err := seedFoods(db); err != nil {
		log.Fatalf("failed to seed foods: %v", err)
	}

	users, err := seedUsers(db)
	if err != nil {
		log.Fatalf("failed to seed users: %v", err)
	}

	if err := seedWorkouts(db, users); err != nil {
		log.Fatalf("failed to seed workouts: %v", err)
	}

	if err := seedMeals(db, users); err != nil {
		log.Fatalf("failed to seed meals: %v", err)
	}

	if err := seedWeightEntries(db, users); err != nil {
		log.Fatalf("failed to seed weight entries: %v", err)
	}

	if err := seedFriendships(db, users); err != nil {
		log.Fatalf("failed to seed friendships: %v", err)
	}

	if err := seedWorkoutPrograms(db, users); err != nil {
		log.Fatalf("failed to seed workout programs: %v", err)
	}

	log.Println("✅ Database seeding completed successfully!")
	log.Println("\n📊 Seeded Data Summary:")
	log.Println("  - Exercises: 20")
	log.Println("  - Foods: 30")
	log.Println("  - Users: 10")
	log.Println("  - Workouts: ~30")
	log.Println("  - Meals: ~50")
	log.Println("  - Weight Entries: ~40")
	log.Println("  - Friendships: ~15")
	log.Println("  - Workout Programs: 3")
}

func seedExercises(db *gorm.DB) error {
	log.Println("  🏋️  Seeding exercises...")

	exercises := []models.Exercise{
		{Name: "Bench Press", MuscleGroup: "Chest", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Lie on bench, press bar up from chest"},
		{Name: "Squat", MuscleGroup: "Legs", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Bar on shoulders, squat down and up"},
		{Name: "Deadlift", MuscleGroup: "Back", Equipment: "Barbell", Difficulty: "Advanced", Instructions: "Lift bar from ground to hip level"},
		{Name: "Overhead Press", MuscleGroup: "Shoulders", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Press bar overhead from shoulders"},
		{Name: "Barbell Row", MuscleGroup: "Back", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Bend over, row bar to chest"},
		{Name: "Pull-up", MuscleGroup: "Back", Equipment: "Bodyweight", Difficulty: "Intermediate", Instructions: "Pull body up to bar"},
		{Name: "Dumbbell Curl", MuscleGroup: "Arms", Equipment: "Dumbbell", Difficulty: "Beginner", Instructions: "Curl dumbbells up to shoulders"},
		{Name: "Tricep Extension", MuscleGroup: "Arms", Equipment: "Dumbbell", Difficulty: "Beginner", Instructions: "Extend arms overhead with dumbbell"},
		{Name: "Leg Press", MuscleGroup: "Legs", Equipment: "Machine", Difficulty: "Beginner", Instructions: "Press weight away with legs"},
		{Name: "Lat Pulldown", MuscleGroup: "Back", Equipment: "Machine", Difficulty: "Beginner", Instructions: "Pull bar down to chest"},
		{Name: "Chest Fly", MuscleGroup: "Chest", Equipment: "Dumbbell", Difficulty: "Intermediate", Instructions: "Open arms wide, bring together over chest"},
		{Name: "Leg Curl", MuscleGroup: "Legs", Equipment: "Machine", Difficulty: "Beginner", Instructions: "Curl legs toward glutes"},
		{Name: "Leg Extension", MuscleGroup: "Legs", Equipment: "Machine", Difficulty: "Beginner", Instructions: "Extend legs forward"},
		{Name: "Calf Raise", MuscleGroup: "Legs", Equipment: "Machine", Difficulty: "Beginner", Instructions: "Raise heels up on toes"},
		{Name: "Plank", MuscleGroup: "Core", Equipment: "Bodyweight", Difficulty: "Beginner", Instructions: "Hold body straight in plank position"},
		{Name: "Crunch", MuscleGroup: "Core", Equipment: "Bodyweight", Difficulty: "Beginner", Instructions: "Curl upper body toward knees"},
		{Name: "Russian Twist", MuscleGroup: "Core", Equipment: "Dumbbell", Difficulty: "Intermediate", Instructions: "Twist torso side to side"},
		{Name: "Incline Bench Press", MuscleGroup: "Chest", Equipment: "Barbell", Difficulty: "Intermediate", Instructions: "Press bar on incline bench"},
		{Name: "Dumbbell Shoulder Press", MuscleGroup: "Shoulders", Equipment: "Dumbbell", Difficulty: "Intermediate", Instructions: "Press dumbbells overhead"},
		{Name: "Face Pull", MuscleGroup: "Shoulders", Equipment: "Cable", Difficulty: "Beginner", Instructions: "Pull cable toward face"},
	}

	for _, ex := range exercises {
		if err := db.FirstOrCreate(&ex, models.Exercise{Name: ex.Name}).Error; err != nil {
			return err
		}
	}

	return nil
}

func seedFoods(db *gorm.DB) error {
	log.Println("  🍎 Seeding foods...")

	foods := []models.Food{
		{Name: "Chicken Breast", Brand: "Generic", Calories: 165, Protein: 31, Carbs: 0, Fats: 3.6, Fiber: 0, Verified: true},
		{Name: "Brown Rice", Brand: "Generic", Calories: 112, Protein: 2.6, Carbs: 24, Fats: 0.9, Fiber: 1.8, Verified: true},
		{Name: "Broccoli", Brand: "Generic", Calories: 34, Protein: 2.8, Carbs: 7, Fats: 0.4, Fiber: 2.6, Verified: true},
		{Name: "Salmon", Brand: "Generic", Calories: 208, Protein: 20, Carbs: 0, Fats: 13, Fiber: 0, Verified: true},
		{Name: "Eggs", Brand: "Generic", Calories: 155, Protein: 13, Carbs: 1.1, Fats: 11, Fiber: 0, Verified: true},
		{Name: "Oatmeal", Brand: "Quaker", Calories: 68, Protein: 2.4, Carbs: 12, Fats: 1.4, Fiber: 1.7, Verified: true},
		{Name: "Banana", Brand: "Generic", Calories: 89, Protein: 1.1, Carbs: 23, Fats: 0.3, Fiber: 2.6, Verified: true},
		{Name: "Greek Yogurt", Brand: "Fage", Calories: 59, Protein: 10, Carbs: 3.6, Fats: 0.4, Fiber: 0, Verified: true},
		{Name: "Almonds", Brand: "Generic", Calories: 579, Protein: 21, Carbs: 22, Fats: 49, Fiber: 12.5, Verified: true},
		{Name: "Sweet Potato", Brand: "Generic", Calories: 86, Protein: 1.6, Carbs: 20, Fats: 0.1, Fiber: 3, Verified: true},
		{Name: "Tuna", Brand: "Generic", Calories: 132, Protein: 28, Carbs: 0, Fats: 1, Fiber: 0, Verified: true},
		{Name: "Avocado", Brand: "Generic", Calories: 160, Protein: 2, Carbs: 9, Fats: 15, Fiber: 7, Verified: true},
		{Name: "Spinach", Brand: "Generic", Calories: 23, Protein: 2.9, Carbs: 3.6, Fats: 0.4, Fiber: 2.2, Verified: true},
		{Name: "Beef Steak", Brand: "Generic", Calories: 271, Protein: 26, Carbs: 0, Fats: 19, Fiber: 0, Verified: true},
		{Name: "Pasta", Brand: "Barilla", Calories: 131, Protein: 5, Carbs: 25, Fats: 1.1, Fiber: 1.8, Verified: true},
		{Name: "Milk", Brand: "Generic", Calories: 42, Protein: 3.4, Carbs: 5, Fats: 1, Fiber: 0, Verified: true},
		{Name: "Cheese", Brand: "Kraft", Calories: 402, Protein: 25, Carbs: 1.3, Fats: 33, Fiber: 0, Verified: true},
		{Name: "Apple", Brand: "Generic", Calories: 52, Protein: 0.3, Carbs: 14, Fats: 0.2, Fiber: 2.4, Verified: true},
		{Name: "Carrots", Brand: "Generic", Calories: 41, Protein: 0.9, Carbs: 10, Fats: 0.2, Fiber: 2.8, Verified: true},
		{Name: "Turkey Breast", Brand: "Generic", Calories: 135, Protein: 30, Carbs: 0, Fats: 1, Fiber: 0, Verified: true},
		{Name: "Quinoa", Brand: "Generic", Calories: 120, Protein: 4.4, Carbs: 21, Fats: 1.9, Fiber: 2.8, Verified: true},
		{Name: "Blueberries", Brand: "Generic", Calories: 57, Protein: 0.7, Carbs: 14, Fats: 0.3, Fiber: 2.4, Verified: true},
		{Name: "Peanut Butter", Brand: "Jif", Calories: 588, Protein: 25, Carbs: 20, Fats: 50, Fiber: 6, Verified: true},
		{Name: "Whey Protein", Brand: "Optimum Nutrition", Calories: 120, Protein: 24, Carbs: 3, Fats: 1, Fiber: 0, Verified: true},
		{Name: "Rice Cakes", Brand: "Quaker", Calories: 387, Protein: 7.1, Carbs: 81, Fats: 2.8, Fiber: 1.2, Verified: true},
		{Name: "Cottage Cheese", Brand: "Daisy", Calories: 98, Protein: 11, Carbs: 3.4, Fats: 4.3, Fiber: 0, Verified: true},
		{Name: "Asparagus", Brand: "Generic", Calories: 20, Protein: 2.2, Carbs: 3.9, Fats: 0.1, Fiber: 2.1, Verified: true},
		{Name: "Bell Pepper", Brand: "Generic", Calories: 31, Protein: 1, Carbs: 6, Fats: 0.3, Fiber: 2.1, Verified: true},
		{Name: "Ground Beef", Brand: "Generic", Calories: 250, Protein: 26, Carbs: 0, Fats: 15, Fiber: 0, Verified: true},
		{Name: "Potato", Brand: "Generic", Calories: 77, Protein: 2, Carbs: 17, Fats: 0.1, Fiber: 2.2, Verified: true},
	}

	for _, food := range foods {
		if err := db.FirstOrCreate(&food, models.Food{Name: food.Name}).Error; err != nil {
			return err
		}
	}

	return nil
}

func seedUsers(db *gorm.DB) ([]models.User, error) {
	log.Println("  👤 Seeding users...")

	usersData := []struct {
		name          string
		email         string
		goal          string
		activityLevel string
		weight        float64
		height        float64
		tdee          int
	}{
		{"Alex Johnson", "alex@example.com", "build_muscle", "moderately_active", 75.0, 180.0, 2600},
		{"Sarah Williams", "sarah@example.com", "lose_fat", "lightly_active", 65.0, 165.0, 1800},
		{"Mike Chen", "mike@example.com", "build_muscle", "very_active", 82.0, 178.0, 3000},
		{"Emily Davis", "emily@example.com", "maintain", "sedentary", 58.0, 160.0, 1600},
		{"James Wilson", "james@example.com", "build_muscle", "moderately_active", 88.0, 185.0, 2800},
		{"Lisa Brown", "lisa@example.com", "lose_fat", "lightly_active", 70.0, 168.0, 1900},
		{"David Lee", "david@example.com", "maintain", "active", 78.0, 175.0, 2400},
		{"Anna Garcia", "anna@example.com", "build_muscle", "very_active", 62.0, 162.0, 2200},
		{"Tom Martinez", "tom@example.com", "lose_fat", "moderately_active", 95.0, 182.0, 2500},
		{"Jessica Taylor", "jessica@example.com", "maintain", "lightly_active", 60.0, 163.0, 1750},
	}

	var users []models.User
	for _, u := range usersData {
		user := models.User{
			Email:         u.email,
			PasswordHash:  "$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // placeholder
			Name:          u.name,
			Goal:          u.goal,
			ActivityLevel: u.activityLevel,
			Weight:        u.weight,
			Height:        u.height,
			TDEE:          u.tdee,
		}
		if err := db.FirstOrCreate(&user, models.User{Email: u.email}).Error; err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func seedWorkouts(db *gorm.DB, users []models.User) error {
	log.Println("  💪 Seeding workouts...")

	var exercises []models.Exercise
	if err := db.Find(&exercises).Error; err != nil {
		return err
	}

	workoutTypes := []string{"push", "pull", "legs", "cardio", "upper", "lower"}

	for _, user := range users {
		numWorkouts := 3 + rand.Intn(4) // 3-6 workouts per user

		for i := 0; i < numWorkouts; i++ {
			workoutDate := time.Now().AddDate(0, 0, -rand.Intn(30))
			workoutType := workoutTypes[rand.Intn(len(workoutTypes))]

			workout := models.Workout{
				UserID:   user.ID,
				Date:     workoutDate,
				Duration: 45 + rand.Intn(60),
				Notes:    fmt.Sprintf("%s workout - feeling good!", workoutType),
				Type:     workoutType,
			}

			if err := db.Create(&workout).Error; err != nil {
				return err
			}

			// Add 3-6 exercises per workout
			numExercises := 3 + rand.Intn(4)
			for j := 0; j < numExercises; j++ {
				exercise := exercises[rand.Intn(len(exercises))]

				workoutExercise := models.WorkoutExercise{
					WorkoutID:  workout.ID,
					ExerciseID: exercise.ID,
					Order:      j + 1,
					Sets:       3 + rand.Intn(3),
					Reps:       8 + rand.Intn(8),
					Weight:     float64(20 + rand.Intn(100)),
					RestTime:   60 + rand.Intn(120),
					Notes:      "",
				}

				if err := db.Create(&workoutExercise).Error; err != nil {
					return err
				}

				// Add sets for this exercise
				for setNum := 1; setNum <= workoutExercise.Sets; setNum++ {
					workoutSet := models.WorkoutSet{
						WorkoutExerciseID: workoutExercise.ID,
						SetNumber:         setNum,
						Reps:              workoutExercise.Reps + rand.Intn(5) - 2,
						Weight:            workoutExercise.Weight,
						RPE:               7 + float64(rand.Intn(3)),
						RestSeconds:       workoutExercise.RestTime,
						Completed:         true,
					}
					if err := db.Create(&workoutSet).Error; err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func seedMeals(db *gorm.DB, users []models.User) error {
	log.Println("  🍽️  Seeding meals...")

	var foods []models.Food
	if err := db.Find(&foods).Error; err != nil {
		return err
	}

	mealTypes := []string{"breakfast", "lunch", "dinner", "snack"}

	for _, user := range users {
		numDays := 5 + rand.Intn(5)

		for day := 0; day < numDays; day++ {
			mealDate := time.Now().AddDate(0, 0, -day)

			for _, mealType := range mealTypes {
				// 70% chance to have this meal type
				if rand.Float64() < 0.7 {
					meal := models.Meal{
						UserID:   user.ID,
						MealType: mealType,
						Date:     mealDate,
						Notes:    "",
					}

					if err := db.Create(&meal).Error; err != nil {
						return err
					}

					// Add 2-5 foods to this meal
					numFoods := 2 + rand.Intn(4)
					for i := 0; i < numFoods; i++ {
						food := foods[rand.Intn(len(foods))]
						quantity := float64(50 + rand.Intn(200))
						units := []string{"g", "ml", "serving", "oz", "cup"}

						mealFood := models.MealFood{
							MealID:   meal.ID,
							FoodID:   food.ID,
							Quantity: quantity,
							Unit:     units[rand.Intn(len(units))],
						}
						if err := db.Create(&mealFood).Error; err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func seedWeightEntries(db *gorm.DB, users []models.User) error {
	log.Println("  ⚖️  Seeding weight entries...")

	for _, user := range users {
		numEntries := 5 + rand.Intn(10)
		baseWeight := user.Weight

		for i := 0; i < numEntries; i++ {
			entryDate := time.Now().AddDate(0, 0, -i*3).Truncate(24 * time.Hour)
			weight := baseWeight + (rand.Float64()*4 - 2) // +/- 2kg variation

			entry := models.WeightEntry{
				UserID: user.ID,
				Date:   entryDate,
				Weight: weight,
				Notes:  "",
			}

			// Check if entry already exists for this user on this date
			var existing models.WeightEntry
			err := db.Where("user_id = ? AND date = ?", user.ID, entryDate).First(&existing).Error
			if err == nil {
				// Entry already exists, skip
				continue
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}

			if err := db.Create(&entry).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func seedFriendships(db *gorm.DB, users []models.User) error {
	log.Println("  🤝 Seeding friendships...")

	statuses := []string{"pending", "accepted"}

	for i, user := range users {
		numFriends := 2 + rand.Intn(4)

		for j := 0; j < numFriends; j++ {
			friendIndex := (i + j + 1) % len(users)
			friend := users[friendIndex]

			status := statuses[rand.Intn(len(statuses))]

			friendship := models.Friendship{
				UserID:      user.ID,
				FriendID:    friend.ID,
				RequesterID: user.ID,
				Status:      status,
			}

			// Use FirstOrCreate to avoid duplicates (handled by unique index)
			err := db.Where("user_id = ? AND friend_id = ?", friendship.UserID, friendship.FriendID).
				Or("user_id = ? AND friend_id = ?", friendship.FriendID, friendship.UserID).
				First(&models.Friendship{}).Error

			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&friendship).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func seedWorkoutPrograms(db *gorm.DB, users []models.User) error {
	log.Println("  📋 Seeding workout programs...")

	// Find first user as admin/creator
	if len(users) == 0 {
		return nil
	}
	admin := users[0]

	programs := []struct {
		name        string
		description string
		duration    int
	}{
		{
			name:        "12-Week Strength Builder",
			description: "Progressive strength program focusing on compound lifts. Perfect for building a solid foundation.",
			duration:    12,
		},
		{
			name:        "8-Week Fat Loss Circuit",
			description: "High-intensity circuit training to maximize calorie burn while preserving muscle mass.",
			duration:    8,
		},
		{
			name:        "6-Week Beginner Foundation",
			description: "Learn proper form and build consistency with this introductory program.",
			duration:    6,
		},
	}

	for _, p := range programs {
		program := models.WorkoutProgram{
			Name:          p.name,
			Description:   p.description,
			DurationWeeks: p.duration,
			CreatedBy:     admin.ID,
		}

		if err := db.FirstOrCreate(&program, models.WorkoutProgram{Name: p.name}).Error; err != nil {
			return err
		}

		// Enroll 3-6 random users
		numEnrollments := 3 + rand.Intn(4)
		for i := 0; i < numEnrollments; i++ {
			user := users[rand.Intn(len(users))]

			enrollment := models.ProgramEnrollment{
				UserID:           user.ID,
				WorkoutProgramID: program.ID,
				Status:           "active",
				StartedOn:        time.Now().AddDate(0, 0, -rand.Intn(30)),
				CurrentWeek:      1 + rand.Intn(p.duration),
			}

			// Check if enrollment exists
			err := db.Where("user_id = ? AND workout_program_id = ?", user.ID, program.ID).
				First(&models.ProgramEnrollment{}).Error

			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&enrollment).Error; err != nil {
					return err
				}

				// Create some progress entries (3 days per week)
				for week := 1; week <= enrollment.CurrentWeek; week++ {
					for day := 1; day <= 3; day++ {
						completed := rand.Float64() < 0.7
						progress := models.ProgramProgress{
							ProgramEnrollmentID: enrollment.ID,
							WeekNumber:          week,
							DayNumber:           day,
							Completed:           completed,
							Notes:               fmt.Sprintf("Week %d Day %d", week, day),
						}
						if err := db.Create(&progress).Error; err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
