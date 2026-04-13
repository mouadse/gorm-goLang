package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"fitness-tracker/database"
	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
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

	switch seedMode() {
	case "never":
		log.Println("database seeding skipped (SEED_MODE=never)")
		return
	case "if-empty":
		needsSeed, err := databaseNeedsSeed(db)
		if err != nil {
			log.Fatalf("seed preflight failed: %v", err)
		}
		if !needsSeed {
			log.Println("existing seed data detected; skipping comprehensive seeding")
			return
		}
	}

	source := rand.New(rand.NewSource(42))

	log.Println("starting comprehensive database seeding...")

	exercises, err := seedExercises(db)
	if err != nil {
		log.Fatalf("failed to seed exercises: %v", err)
	}

	users, err := seedUsers(db)
	if err != nil {
		log.Fatalf("failed to seed users: %v", err)
	}

	foods, err := seedFoods(db)
	if err != nil {
		log.Fatalf("failed to seed foods: %v", err)
	}

	workouts, err := seedWorkouts(db, source, users, exercises)
	if err != nil {
		log.Fatalf("failed to seed workouts: %v", err)
	}

	if err := seedWorkoutCardioEntries(db, source, users, workouts); err != nil {
		log.Fatalf("failed to seed workout cardio entries: %v", err)
	}

	templates, err := seedWorkoutTemplates(db, source, users, exercises)
	if err != nil {
		log.Fatalf("failed to seed workout templates: %v", err)
	}

	if err := seedWorkoutPrograms(db, users, templates); err != nil {
		log.Fatalf("failed to seed workout programs: %v", err)
	}

	if err := seedMeals(db, source, users, foods); err != nil {
		log.Fatalf("failed to seed meals: %v", err)
	}

	if err := seedWeightEntries(db, users); err != nil {
		log.Fatalf("failed to seed weight entries: %v", err)
	}

	if err := seedFavoriteFoods(db, users, foods); err != nil {
		log.Fatalf("failed to seed favorite foods: %v", err)
	}

	if err := seedRecipes(db, source, users, foods); err != nil {
		log.Fatalf("failed to seed recipes: %v", err)
	}

	if err := seedNotifications(db, users); err != nil {
		log.Fatalf("failed to seed notifications: %v", err)
	}

	log.Println("database seeding completed successfully")
	log.Println("seeded data summary:")
	log.Println("  - Users: 12")
	log.Printf("  - Exercises: %d", len(exercises))
	log.Printf("  - Foods: %d", len(foods))
	log.Println("  - Nutrients: 19")
	log.Println("  - Workouts: 24")
	log.Println("  - Cardio entries: 12")
	log.Println("  - Workout templates: 6")
	log.Println("  - Workout programs: 2")
	log.Println("  - Meals: 36")
	log.Println("  - Weight entries: 48")
	log.Println("  - Favorite foods: 24")
	log.Println("  - Recipes: 4")
	log.Println("  - Notifications: 12")
}

func seedMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("SEED_MODE")))
	if mode == "" {
		return "if-empty"
	}

	switch mode {
	case "always", "if-empty", "never":
		return mode
	default:
		log.Printf("unknown SEED_MODE %q, defaulting to if-empty", mode)
		return "if-empty"
	}
}

func databaseNeedsSeed(db *gorm.DB) (bool, error) {
	checks := []struct {
		name  string
		model any
	}{
		{name: "users", model: &models.User{}},
		{name: "foods", model: &models.Food{}},
		{name: "exercises", model: &models.Exercise{}},
	}

	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Limit(1).Count(&count).Error; err != nil {
			return false, fmt.Errorf("count %s: %w", check.name, err)
		}
		if count == 0 {
			return true, nil
		}
	}

	return false, nil
}

func seedExercises(db *gorm.DB) ([]models.Exercise, error) {
	log.Println("  syncing exercises from exercise library service...")

	client := services.NewExerciseLibClient()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	readyResp, err := client.WaitUntilReady(ctx)
	if err != nil {
		return nil, fmt.Errorf("wait for exercise library readiness: %w", err)
	}
	log.Printf("  exercise library ready: %d exercises indexed", readyResp.ExercisesLoaded)

	catalog, err := client.GetAllExercises()
	if err != nil {
		return nil, fmt.Errorf("fetch exercises: %w", err)
	}
	if len(catalog) == 0 {
		return nil, fmt.Errorf("exercise library returned zero exercises")
	}

	exercises := make([]models.Exercise, 0, len(catalog))
	for _, item := range catalog {
		exercise := models.Exercise{
			ExerciseLibID:    item.ExerciseID,
			Name:             item.Name,
			Force:            item.Force,
			Level:            item.Level,
			Mechanic:         item.Mechanic,
			Equipment:        item.Equipment,
			Category:         item.Category,
			PrimaryMuscles:   strings.Join(item.PrimaryMuscles, ","),
			SecondaryMuscles: strings.Join(item.SecondaryMuscles, ","),
			Instructions:     strings.Join(item.Instructions, "\n"),
			ImageURL:         derefString(item.ImageURL),
			AltImageURL:      derefString(item.AltImageURL),
		}

		var persisted models.Exercise
		err := db.Where("exercise_lib_id = ? OR name = ?", exercise.ExerciseLibID, exercise.Name).First(&persisted).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := db.Create(&exercise).Error; err != nil {
					return nil, err
				}
				persisted = exercise
			} else {
				return nil, err
			}
		} else {
			// Found, update it
			if err := db.Model(&persisted).Updates(exercise).Error; err != nil {
				return nil, err
			}
		}
		exercises = append(exercises, persisted)
	}

	log.Printf("  synced %d exercises", len(exercises))
	return exercises, nil
}

func seedLegacyExercises(db *gorm.DB) ([]models.Exercise, error) {
	seeds := []models.Exercise{
		{Name: "Bench Press", PrimaryMuscles: "Chest", Equipment: "Barbell", Level: "Intermediate", Instructions: "Press the bar from chest to lockout."},
		{Name: "Back Squat", PrimaryMuscles: "Legs", Equipment: "Barbell", Level: "Intermediate", Instructions: "Squat to depth while keeping the torso braced."},
		{Name: "Deadlift", PrimaryMuscles: "Back", Equipment: "Barbell", Level: "Advanced", Instructions: "Drive through the floor and stand tall with the bar."},
		{Name: "Pull-Up", PrimaryMuscles: "Back", Equipment: "Bodyweight", Level: "Intermediate", Instructions: "Pull until the chin clears the bar."},
		{Name: "Overhead Press", PrimaryMuscles: "Shoulders", Equipment: "Barbell", Level: "Intermediate", Instructions: "Press vertically from shoulder rack position."},
		{Name: "Dumbbell Shoulder Press", PrimaryMuscles: "Shoulders", Equipment: "Dumbbell", Level: "Beginner", Instructions: "Press both dumbbells overhead while keeping your ribs down."},
		{Name: "Lateral Raise", PrimaryMuscles: "Shoulders", Equipment: "Dumbbell", Level: "Beginner", Instructions: "Raise the dumbbells out to shoulder height with a soft bend in the elbows."},
		{Name: "Dumbbell Row", PrimaryMuscles: "Back", Equipment: "Dumbbell", Level: "Beginner", Instructions: "Row toward the hip while staying square."},
		{Name: "Band Pull-Apart", PrimaryMuscles: "Back", Equipment: "Resistance Band", Level: "Beginner", Instructions: "Pull the band apart at chest height while keeping the shoulders down."},
		{Name: "Superman Hold", PrimaryMuscles: "Back", Equipment: "Bodyweight", Level: "Beginner", Instructions: "Lift the arms and legs slightly off the floor and hold with the core braced."},
		{Name: "Romanian Deadlift", PrimaryMuscles: "Hamstrings", Equipment: "Barbell", Level: "Intermediate", Instructions: "Hinge at the hips and keep the bar close."},
		{Name: "Walking Lunge", PrimaryMuscles: "Legs", Equipment: "Bodyweight", Level: "Beginner", Instructions: "Step forward and control the knee to the floor."},
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

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
	jordanDOB := time.Date(1990, 5, 12, 0, 0, 0, 0, time.UTC)
	taylorDOB := time.Date(1994, 9, 25, 0, 0, 0, 0, time.UTC)
	caseyDOB := time.Date(1985, 2, 18, 0, 0, 0, 0, time.UTC)
	morganDOB := time.Date(2000, 11, 3, 0, 0, 0, 0, time.UTC)
	rileyDOB := time.Date(1993, 8, 29, 0, 0, 0, 0, time.UTC)
	jamieDOB := time.Date(1989, 4, 7, 0, 0, 0, 0, time.UTC)
	quinnDOB := time.Date(1996, 12, 14, 0, 0, 0, 0, time.UTC)
	averyDOB := time.Date(1991, 1, 20, 0, 0, 0, 0, time.UTC)

	seeds := []models.User{
		{Email: "alex@example.com", PasswordHash: string(passwordHash), Name: "Alex Johnson", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=alex", Goal: "build_muscle", ActivityLevel: "moderately_active", Weight: 78, Height: 181, TDEE: 2600, DateOfBirth: &alexDOB, Age: ageFromDateOfBirth(alexDOB), Role: "admin"},
		{Email: "sarah@example.com", PasswordHash: string(passwordHash), Name: "Sarah Williams", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=sarah", Goal: "lose_fat", ActivityLevel: "lightly_active", Weight: 64, Height: 165, TDEE: 1850, DateOfBirth: &sarahDOB, Age: ageFromDateOfBirth(sarahDOB), Role: "user"},
		{Email: "mike@example.com", PasswordHash: string(passwordHash), Name: "Mike Chen", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=mike", Goal: "maintain", ActivityLevel: "active", Weight: 82, Height: 178, TDEE: 2750, DateOfBirth: &mikeDOB, Age: ageFromDateOfBirth(mikeDOB), Role: "user"},
		{Email: "emily@example.com", PasswordHash: string(passwordHash), Name: "Emily Davis", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=emily", Goal: "build_muscle", ActivityLevel: "moderately_active", Weight: 59, Height: 163, TDEE: 2100, DateOfBirth: &emilyDOB, Age: ageFromDateOfBirth(emilyDOB), Role: "user"},
		{Email: "jordan@example.com", PasswordHash: string(passwordHash), Name: "Jordan Smith", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=jordan", Goal: "lose_fat", ActivityLevel: "sedentary", Weight: 95, Height: 175, TDEE: 2100, DateOfBirth: &jordanDOB, Age: ageFromDateOfBirth(jordanDOB), Role: "user"},
		{Email: "taylor@example.com", PasswordHash: string(passwordHash), Name: "Taylor Brown", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=taylor", Goal: "maintain", ActivityLevel: "moderately_active", Weight: 70, Height: 170, TDEE: 2300, DateOfBirth: &taylorDOB, Age: ageFromDateOfBirth(taylorDOB), Role: "user"},
		{Email: "casey@example.com", PasswordHash: string(passwordHash), Name: "Casey Garcia", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=casey", Goal: "build_muscle", ActivityLevel: "active", Weight: 85, Height: 185, TDEE: 3000, DateOfBirth: &caseyDOB, Age: ageFromDateOfBirth(caseyDOB), Role: "user"},
		{Email: "morgan@example.com", PasswordHash: string(passwordHash), Name: "Morgan Miller", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=morgan", Goal: "lose_fat", ActivityLevel: "lightly_active", Weight: 68, Height: 168, TDEE: 1900, DateOfBirth: &morganDOB, Age: ageFromDateOfBirth(morganDOB), Role: "user"},
		{Email: "riley@example.com", PasswordHash: string(passwordHash), Name: "Riley Davis", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=riley", Goal: "maintain", ActivityLevel: "moderately_active", Weight: 75, Height: 178, TDEE: 2500, DateOfBirth: &rileyDOB, Age: ageFromDateOfBirth(rileyDOB), Role: "user"},
		{Email: "jamie@example.com", PasswordHash: string(passwordHash), Name: "Jamie Wilson", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=jamie", Goal: "build_muscle", ActivityLevel: "active", Weight: 80, Height: 180, TDEE: 2800, DateOfBirth: &jamieDOB, Age: ageFromDateOfBirth(jamieDOB), Role: "user"},
		{Email: "quinn@example.com", PasswordHash: string(passwordHash), Name: "Quinn Moore", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=quinn", Goal: "lose_fat", ActivityLevel: "lightly_active", Weight: 72, Height: 172, TDEE: 2000, DateOfBirth: &quinnDOB, Age: ageFromDateOfBirth(quinnDOB), Role: "user"},
		{Email: "avery@example.com", PasswordHash: string(passwordHash), Name: "Avery Taylor", Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=avery", Goal: "maintain", ActivityLevel: "moderately_active", Weight: 65, Height: 165, TDEE: 2100, DateOfBirth: &averyDOB, Age: ageFromDateOfBirth(averyDOB), Role: "user"},
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

func seedFoods(db *gorm.DB) ([]models.Food, error) {
	log.Println("  importing foods from USDA dataset...")

	datasetPath := services.USDAImportDatasetPath()
	if _, err := os.Stat(datasetPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("USDA dataset not found at %q; set USDA_DATASET_PATH or place the file at %q", datasetPath, services.DefaultUSDADatasetPath)
		}
		return nil, fmt.Errorf("stat USDA dataset %q: %w", datasetPath, err)
	}

	importer := services.NewUSDAImportService(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stats, err := importer.ImportFromFile(ctx, datasetPath)
	if err != nil {
		return nil, fmt.Errorf("import USDA foods from %q: %w", datasetPath, err)
	}

	var foods []models.Food
	if err := db.Where("source = ?", "usda").Order("name ASC").Find(&foods).Error; err != nil {
		return nil, fmt.Errorf("load imported USDA foods: %w", err)
	}
	if len(foods) == 0 {
		return nil, fmt.Errorf("USDA import completed from %q but no foods were found", datasetPath)
	}

	log.Printf("  USDA import ready: %d foods processed (%d new)", stats.FoodCount, stats.NewFoods)
	return foods, nil
}

func seedWorkouts(db *gorm.DB, source *rand.Rand, users []models.User, exercises []models.Exercise) ([]models.Workout, error) {
	log.Println("  seeding workouts...")

	baseDate := time.Now().UTC().Truncate(24 * time.Hour)
	workoutTypes := []string{"push", "pull", "legs", "full_body"}
	notes := []string{"Focus on form", "Felt heavy today", "RPE target hit", "Great energy today"}

	var persistedWorkouts []models.Workout

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
				return nil, err
			}
			workout = persistedWorkout
			persistedWorkouts = append(persistedWorkouts, workout)

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
					Notes:      notes[source.Intn(len(notes))],
				}

				var persistedWorkoutExercise models.WorkoutExercise
				if err := db.Where(&models.WorkoutExercise{
					WorkoutID:  workout.ID,
					ExerciseID: exercise.ID,
					Order:      exerciseIndex + 1,
				}).Assign(workoutExercise).FirstOrCreate(&persistedWorkoutExercise).Error; err != nil {
					return nil, err
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
						return nil, err
					}
				}
			}
		}
	}

	return persistedWorkouts, nil
}

func seedWorkoutCardioEntries(db *gorm.DB, source *rand.Rand, users []models.User, workouts []models.Workout) error {
	log.Println("  seeding workout cardio entries...")

	modalities := []string{"running", "cycling", "rowing", "elliptical"}

	for i := 0; i < 12 && i < len(workouts); i++ {
		workout := workouts[i]

		dist := 3.0 + float64(source.Intn(7))
		unit := "km"
		cals := 200 + source.Intn(300)
		hr := 130 + source.Intn(35)

		entry := models.WorkoutCardioEntry{
			WorkoutID:       workout.ID,
			Modality:        modalities[source.Intn(len(modalities))],
			DurationMinutes: 20 + source.Intn(25),
			Distance:        &dist,
			DistanceUnit:    &unit,
			CaloriesBurned:  &cals,
			AvgHeartRate:    &hr,
			Notes:           "Quick cardio after weights",
		}

		if err := db.Where("workout_id = ? AND modality = ?", entry.WorkoutID, entry.Modality).
			Assign(entry).
			FirstOrCreate(&models.WorkoutCardioEntry{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func seedWorkoutTemplates(db *gorm.DB, source *rand.Rand, users []models.User, exercises []models.Exercise) ([]models.WorkoutTemplate, error) {
	log.Println("  seeding workout templates...")

	templates := []models.WorkoutTemplate{
		{OwnerID: users[0].ID, Name: "Push Day Template", Type: "push", Notes: "Standard push workout"},
		{OwnerID: users[1].ID, Name: "Pull Day Template", Type: "pull", Notes: "Back and biceps focus"},
		{OwnerID: users[2].ID, Name: "Leg Day Template", Type: "legs", Notes: "Quads and hamstrings"},
		{OwnerID: users[4].ID, Name: "Upper Body Power", Type: "push", Notes: "Strength focus"},
		{OwnerID: users[5].ID, Name: "Lower Body Power", Type: "legs", Notes: "Squat focused"},
		{OwnerID: users[6].ID, Name: "Full Body Blast", Type: "full_body", Notes: "High intensity"},
	}

	var persistedTemplates []models.WorkoutTemplate

	for i, t := range templates {
		var template models.WorkoutTemplate
		if err := db.Where("owner_id = ? AND name = ?", t.OwnerID, t.Name).Assign(t).FirstOrCreate(&template).Error; err != nil {
			return nil, err
		}
		persistedTemplates = append(persistedTemplates, template)

		for j := 0; j < 4; j++ {
			exercise := exercises[(i*2+j)%len(exercises)]
			templateExercise := models.WorkoutTemplateExercise{
				TemplateID: template.ID,
				ExerciseID: exercise.ID,
				Order:      j + 1,
				Sets:       3,
				Reps:       10,
				Weight:     40,
				RestTime:   90,
				Notes:      "Control the eccentric",
			}

			var persistedTemplateExercise models.WorkoutTemplateExercise
			if err := db.Where("template_id = ? AND exercise_id = ? AND \"order\" = ?", templateExercise.TemplateID, templateExercise.ExerciseID, templateExercise.Order).
				Assign(templateExercise).
				FirstOrCreate(&persistedTemplateExercise).Error; err != nil {
				return nil, err
			}

			for setNum := 1; setNum <= 3; setNum++ {
				templateSet := models.WorkoutTemplateSet{
					TemplateExerciseID: persistedTemplateExercise.ID,
					SetNumber:          setNum,
					Reps:               10,
					Weight:             40,
					RestSeconds:        90,
				}
				if err := db.Where("template_exercise_id = ? AND set_number = ?", templateSet.TemplateExerciseID, templateSet.SetNumber).
					Assign(templateSet).
					FirstOrCreate(&models.WorkoutTemplateSet{}).Error; err != nil {
					return nil, err
				}
			}
		}
	}

	return persistedTemplates, nil
}

func seedWorkoutPrograms(db *gorm.DB, users []models.User, templates []models.WorkoutTemplate) error {
	log.Println("  seeding workout programs...")

	var admin models.User
	for _, u := range users {
		if u.Role == "admin" {
			admin = u
			break
		}
	}
	if admin.ID == uuid.Nil {
		admin = users[0]
	}

	program := models.WorkoutProgram{
		Name:        "12-Week Powerbuilding",
		Description: "A 12-week powerbuilding block combining strength and hypertrophy.",
		CreatedBy:   admin.ID,
		IsActive:    true,
	}

	if err := db.Where("name = ?", program.Name).Assign(program).FirstOrCreate(&program).Error; err != nil {
		return err
	}

	for weekNum := 1; weekNum <= 2; weekNum++ {
		week := models.ProgramWeek{
			ProgramID:  program.ID,
			WeekNumber: weekNum,
			Name:       "Phase 1: Accumulation",
		}
		if err := db.Where("program_id = ? AND week_number = ?", week.ProgramID, week.WeekNumber).Assign(week).FirstOrCreate(&week).Error; err != nil {
			return err
		}

		for dayNum := 1; dayNum <= 3; dayNum++ {
			var templateID *uuid.UUID
			if len(templates) > 0 {
				id := templates[dayNum%len(templates)].ID
				templateID = &id
			}

			session := models.ProgramSession{
				WeekID:            week.ID,
				DayNumber:         dayNum,
				WorkoutTemplateID: templateID,
				Notes:             "Focus on eccentric control",
			}
			if err := db.Where("week_id = ? AND day_number = ?", session.WeekID, session.DayNumber).Assign(session).FirstOrCreate(&models.ProgramSession{}).Error; err != nil {
				return err
			}
		}
	}

	// Assignments
	now := time.Now().UTC()
	assignments := []models.ProgramAssignment{
		{UserID: users[1].ID, ProgramID: program.ID, AssignedAt: now, Status: "assigned"},
		{UserID: users[2].ID, ProgramID: program.ID, AssignedAt: now.AddDate(0, 0, -5), StartedAt: &now, Status: "in_progress"},
	}

	for _, a := range assignments {
		if err := db.Where("user_id = ? AND program_id = ?", a.UserID, a.ProgramID).Assign(a).FirstOrCreate(&models.ProgramAssignment{}).Error; err != nil {
			return err
		}
	}

	return nil
}

func seedMeals(db *gorm.DB, source *rand.Rand, users []models.User, foods []models.Food) error {
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

			var persistedMeal models.Meal
			if err := db.Where("user_id = ? AND date = ? AND meal_type = ?", user.ID, mealDate, meal.MealType).
				Assign(meal).
				FirstOrCreate(&persistedMeal).Error; err != nil {
				return err
			}
			meal = persistedMeal

			// Add 2-3 food items to each meal
			numItems := 2 + source.Intn(2)
			for i := 0; i < numItems; i++ {
				food := foods[source.Intn(len(foods))]
				mealFood := models.MealFood{
					MealID:   meal.ID,
					FoodID:   food.ID,
					Quantity: float64(1 + source.Intn(3)), // 1-3 servings
				}

				if err := db.Where("meal_id = ? AND food_id = ?", meal.ID, food.ID).
					Assign(mealFood).
					FirstOrCreate(&models.MealFood{}).Error; err != nil {
					return err
				}
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

func seedFavoriteFoods(db *gorm.DB, users []models.User, foods []models.Food) error {
	log.Println("  seeding favorite foods...")

	if len(foods) == 0 {
		return fmt.Errorf("cannot seed favorite foods: foods list is empty")
	}
	if len(users) == 0 {
		return fmt.Errorf("cannot seed favorite foods: users list is empty")
	}

	for _, user := range users {
		for i := 0; i < 2; i++ {
			food := foods[(int(user.ID[0])+i)%len(foods)]
			fav := models.FavoriteFood{
				UserID: user.ID,
				FoodID: food.ID,
			}
			if err := db.Where("user_id = ? AND food_id = ?", fav.UserID, fav.FoodID).
				Assign(fav).
				FirstOrCreate(&models.FavoriteFood{}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func seedRecipes(db *gorm.DB, source *rand.Rand, users []models.User, foods []models.Food) error {
	log.Println("  seeding recipes...")

	recipeNames := []string{"Post-Workout Shake", "Chicken Rice Bowl", "Overnight Oats", "Omelette Deluxe"}

	for i, name := range recipeNames {
		user := users[i%len(users)]
		recipe := models.Recipe{
			UserID:   user.ID,
			Name:     name,
			Servings: 1 + source.Intn(3),
			Notes:    "Healthy and quick",
		}

		if err := db.Where("user_id = ? AND name = ?", recipe.UserID, recipe.Name).
			Assign(recipe).
			FirstOrCreate(&recipe).Error; err != nil {
			return err
		}

		for j := 0; j < 3; j++ {
			food := foods[(i+j)%len(foods)]
			item := models.RecipeItem{
				RecipeID: recipe.ID,
				FoodID:   food.ID,
				Quantity: 1.0 + float64(source.Intn(2)),
			}
			if err := db.Where("recipe_id = ? AND food_id = ?", item.RecipeID, item.FoodID).
				Assign(item).
				FirstOrCreate(&models.RecipeItem{}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func seedNotifications(db *gorm.DB, users []models.User) error {
	log.Println("  seeding notifications...")

	now := time.Now().UTC()

	notificationSeeds := []struct {
		Type    models.NotificationType
		Title   string
		Message string
		Read    bool
	}{
		{models.NotificationLowProtein, "Low Protein Alert", "You haven't reached your protein goal today.", false},
		{models.NotificationMissedMeal, "Missed Meal", "Don't forget to log your lunch!", true},
		{models.NotificationWorkoutReminder, "Workout Time", "Time for your Push session.", false},
		{models.NotificationRestDayWarning, "Rest Day", "Overtraining detected, consider a rest day.", false},
		{models.NotificationRecoveryWarning, "Recovery Alert", "Sleep quality was low, take it easy.", false},
		{models.NotificationGoalAlignment, "Goal Alignment", "Your current intake aligns with your weight loss goal.", true},
	}

	for i, user := range users {
		seed := notificationSeeds[i%len(notificationSeeds)]
		var readAt *time.Time
		if seed.Read {
			readAt = &now
		}

		notif := models.Notification{
			UserID:  user.ID,
			Type:    seed.Type,
			Title:   seed.Title,
			Message: seed.Message,
			ReadAt:  readAt,
		}

		if err := db.Where("user_id = ? AND type = ? AND title = ?", notif.UserID, notif.Type, notif.Title).
			Assign(notif).
			FirstOrCreate(&models.Notification{}).Error; err != nil {
			return err
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
