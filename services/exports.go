// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExportFormat represents the format for data export.
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

// ExportStatus represents the status of an export job.
type ExportStatus string

const (
	ExportPending    ExportStatus = "pending"
	ExportProcessing ExportStatus = "processing"
	ExportCompleted  ExportStatus = "completed"
	ExportFailed     ExportStatus = "failed"
)

// ExportJob represents a user data export job.
type ExportJob struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID    `gorm:"type:uuid;not null;index" json:"user_id"`
	Format      ExportFormat `gorm:"type:varchar(20);not null" json:"format"`
	Status      ExportStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	FilePath    string       `gorm:"type:varchar(512)" json:"file_path"`
	Error       string       `gorm:"type:text" json:"error,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
}

// BeforeCreate sets a new UUID before inserting.
func (e *ExportJob) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// UserDataExport contains all user data for export.
type UserDataExport struct {
	ExportID           string                    `json:"export_id"`
	UserID             string                    `json:"user_id"`
	ExportedAt         time.Time                 `json:"exported_at"`
	Format             string                    `json:"format"`
	User               *models.User              `json:"user"`
	Workouts           []WorkoutExport           `json:"workouts"`
	Meals              []MealExport              `json:"meals"`
	WeightEntries      []WeightEntryExport       `json:"weight_entries"`
	FavoriteFoods      []FavoriteFoodExport      `json:"favorite_foods"`
	Recipes            []RecipeExport            `json:"recipes"`
	WorkoutTemplates   []WorkoutTemplateExport   `json:"workout_templates"`
	WorkoutPrograms    []WorkoutProgramExport    `json:"workout_programs"`
	ProgramAssignments []ProgramAssignmentExport `json:"program_assignments"`
	Notifications      []NotificationExport      `json:"notifications"`
}

// WorkoutExport represents a workout in the export format.
type WorkoutExport struct {
	ID        uuid.UUID               `json:"id"`
	Date      time.Time               `json:"date"`
	Duration  int                     `json:"duration"`
	Type      string                  `json:"type"`
	Notes     string                  `json:"notes"`
	Exercises []WorkoutExerciseExport `json:"exercises"`
	CreatedAt time.Time               `json:"created_at"`
}

// WorkoutExerciseExport represents a workout exercise in the export format.
type WorkoutExerciseExport struct {
	ExerciseID   uuid.UUID          `json:"exercise_id"`
	ExerciseName string             `json:"exercise_name"`
	Order        int                `json:"order"`
	Sets         int                `json:"sets"`
	Reps         int                `json:"reps"`
	Weight       float64            `json:"weight"`
	RestTime     int                `json:"rest_time"`
	Notes        string             `json:"notes"`
	SetEntries   []WorkoutSetExport `json:"set_entries"`
}

// WorkoutSetExport represents a workout set in the export format.
type WorkoutSetExport struct {
	SetNumber   int     `json:"set_number"`
	Reps        int     `json:"reps"`
	Weight      float64 `json:"weight"`
	RPE         float64 `json:"rpe"`
	RestSeconds int     `json:"rest_seconds"`
	Completed   bool    `json:"completed"`
}

// MealExport represents a meal in the export format.
type MealExport struct {
	ID            uuid.UUID        `json:"id"`
	MealType      string           `json:"meal_type"`
	Date          time.Time        `json:"date"`
	Notes         string           `json:"notes"`
	Foods         []MealFoodExport `json:"foods"`
	TotalCalories float64          `json:"total_calories"`
	TotalProtein  float64          `json:"total_protein"`
	TotalCarbs    float64          `json:"total_carbs"`
	TotalFat      float64          `json:"total_fat"`
	CreatedAt     time.Time        `json:"created_at"`
}

// MealFoodExport represents a meal food in the export format.
type MealFoodExport struct {
	FoodID   uuid.UUID `json:"food_id"`
	FoodName string    `json:"food_name"`
	Quantity float64   `json:"quantity"`
	Calories float64   `json:"calories"`
	Protein  float64   `json:"protein"`
	Carbs    float64   `json:"carbs"`
	Fat      float64   `json:"fat"`
}

// WeightEntryExport represents a weight entry in the export format.
type WeightEntryExport struct {
	ID        uuid.UUID `json:"id"`
	Weight    float64   `json:"weight"`
	Date      time.Time `json:"date"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// FavoriteFoodExport represents a favorite food in the export format.
type FavoriteFoodExport struct {
	ID        uuid.UUID `json:"id"`
	FoodID    uuid.UUID `json:"food_id"`
	FoodName  string    `json:"food_name"`
	CreatedAt time.Time `json:"created_at"`
}

// RecipeExport represents a recipe in the export format.
type RecipeExport struct {
	ID        uuid.UUID          `json:"id"`
	Name      string             `json:"name"`
	Servings  int                `json:"servings"`
	Notes     string             `json:"notes"`
	Items     []RecipeItemExport `json:"items"`
	CreatedAt time.Time          `json:"created_at"`
}

// RecipeItemExport represents a recipe item in the export format.
type RecipeItemExport struct {
	FoodID   uuid.UUID `json:"food_id"`
	FoodName string    `json:"food_name"`
	Quantity float64   `json:"quantity"`
}

// WorkoutTemplateExport represents a workout template in the export format.
type WorkoutTemplateExport struct {
	ID        uuid.UUID                       `json:"id"`
	Name      string                          `json:"name"`
	Type      string                          `json:"type"`
	Notes     string                          `json:"notes"`
	Exercises []WorkoutTemplateExerciseExport `json:"exercises"`
	CreatedAt time.Time                       `json:"created_at"`
}

// WorkoutTemplateExerciseExport represents a template exercise in the export format.
type WorkoutTemplateExerciseExport struct {
	ExerciseID   uuid.UUID                  `json:"exercise_id"`
	ExerciseName string                     `json:"exercise_name"`
	Order        int                        `json:"order"`
	Sets         int                        `json:"sets"`
	Reps         int                        `json:"reps"`
	Weight       float64                    `json:"weight"`
	RestTime     int                        `json:"rest_time"`
	Notes        string                     `json:"notes"`
	SetTemplates []WorkoutTemplateSetExport `json:"set_templates"`
}

// WorkoutTemplateSetExport represents a template set in the export format.
type WorkoutTemplateSetExport struct {
	SetNumber   int     `json:"set_number"`
	Reps        int     `json:"reps"`
	Weight      float64 `json:"weight"`
	RestSeconds int     `json:"rest_seconds"`
}

// WorkoutProgramExport represents a workout program in the export format.
type WorkoutProgramExport struct {
	ID          uuid.UUID           `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	IsActive    bool                `json:"is_active"`
	Weeks       []ProgramWeekExport `json:"weeks"`
	CreatedAt   time.Time           `json:"created_at"`
}

// ProgramWeekExport represents a program week in the export format.
type ProgramWeekExport struct {
	WeekNumber int                    `json:"week_number"`
	Name       string                 `json:"name"`
	Sessions   []ProgramSessionExport `json:"sessions"`
}

// ProgramSessionExport represents a program session in the export format.
type ProgramSessionExport struct {
	DayNumber    int    `json:"day_number"`
	TemplateName string `json:"template_name,omitempty"`
	Notes        string `json:"notes"`
}

// ProgramAssignmentExport represents a program assignment in the export format.
type ProgramAssignmentExport struct {
	ID          uuid.UUID  `json:"id"`
	ProgramID   uuid.UUID  `json:"program_id"`
	ProgramName string     `json:"program_name"`
	AssignedAt  time.Time  `json:"assigned_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string     `json:"status"`
}

// NotificationExport represents a notification in the export format.
type NotificationExport struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	PayloadJSON string     `json:"payload_json,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// DeletionRequest represents a user deletion request.
type DeletionRequest struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	RequestedAt time.Time  `json:"requested_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Status      string     `json:"status"` // pending, processed
}

// BeforeCreate sets a new UUID before inserting.
func (d *DeletionRequest) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// ExportService provides business logic for data exports and GDPR workflows.
type ExportService struct {
	db *gorm.DB
}

// NewExportService creates a new export service.
func NewExportService(db *gorm.DB) *ExportService {
	return &ExportService{db: db}
}

// CreateExportJob creates a new export job for a user.
func (s *ExportService) CreateExportJob(userID uuid.UUID, format ExportFormat) (*ExportJob, error) {
	job := ExportJob{
		UserID: userID,
		Format: format,
		Status: ExportPending,
	}

	if err := s.db.Create(&job).Error; err != nil {
		return nil, err
	}

	return &job, nil
}

// GetExportJob retrieves an export job by ID.
func (s *ExportService) GetExportJob(userID, jobID uuid.UUID) (*ExportJob, error) {
	var job ExportJob
	err := s.db.First(&job, "id = ? AND user_id = ?", jobID, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("export job not found")
		}
		return nil, err
	}
	return &job, nil
}

// ListExportJobs lists all export jobs for a user.
func (s *ExportService) ListExportJobs(userID uuid.UUID) ([]ExportJob, error) {
	var jobs []ExportJob
	err := s.db.Where("user_id = ?", userID).Order("created_at desc").Find(&jobs).Error
	return jobs, err
}

// ListPendingJobs returns pending export jobs for worker processing.
func (s *ExportService) ListPendingJobs(limit int) ([]ExportJob, error) {
	if limit <= 0 {
		limit = 10
	}

	var jobs []ExportJob
	err := s.db.
		Where("status = ?", ExportPending).
		Order("created_at asc").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

// ProcessExportJob processes an export job and generates the export data.
func (s *ExportService) ProcessExportJob(jobID uuid.UUID) error {
	var job ExportJob
	if err := s.db.First(&job, "id = ?", jobID).Error; err != nil {
		return err
	}

	// Update status to processing
	job.Status = ExportProcessing
	s.db.Save(&job)

	// Generate export data
	_, err := s.generateExportData(job.UserID, job.Format)
	if err != nil {
		job.Status = ExportFailed
		job.Error = err.Error()
		s.db.Save(&job)
		return err
	}

	// Store export (in a real system, this would be saved to a file or cloud storage)
	job.FilePath = fmt.Sprintf("exports/%s/%s.%s", job.UserID, job.ID, job.Format)
	job.Status = ExportCompleted
	now := time.Now().UTC()
	job.CompletedAt = &now

	if err := s.db.Save(&job).Error; err != nil {
		return err
	}

	_, err = NewNotificationService(s.db).CreateIfNotDuplicate(
		job.UserID,
		models.NotificationExportReady,
		"Export ready",
		"Your data export is ready to download.",
		map[string]interface{}{
			"export_id": job.ID.String(),
			"format":    string(job.Format),
			"file_path": job.FilePath,
		},
		0,
	)
	if err != nil {
		return fmt.Errorf("create export ready notification: %w", err)
	}

	return nil
}

// generateExportData generates the user data export.
func (s *ExportService) generateExportData(userID uuid.UUID, format ExportFormat) (*UserDataExport, error) {
	export := &UserDataExport{
		ExportID:   uuid.New().String(),
		UserID:     userID.String(),
		ExportedAt: time.Now().UTC(),
		Format:     string(format),
	}

	// Get user
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	export.User = &user

	// Get workouts with exercises and sets
	var workouts []models.Workout
	err := s.db.
		Preload("WorkoutExercises.Exercise").
		Preload("WorkoutExercises.WorkoutSets").
		Where("user_id = ?", userID).
		Order("date desc").
		Find(&workouts).Error
	if err != nil {
		return nil, err
	}

	for _, w := range workouts {
		workoutExport := WorkoutExport{
			ID:        w.ID,
			Date:      w.Date,
			Duration:  w.Duration,
			Type:      w.Type,
			Notes:     w.Notes,
			CreatedAt: w.CreatedAt,
		}

		for _, we := range w.WorkoutExercises {
			exerciseExport := WorkoutExerciseExport{
				ExerciseID:   we.ExerciseID,
				ExerciseName: we.Exercise.Name,
				Order:        we.Order,
				Sets:         we.Sets,
				Reps:         we.Reps,
				Weight:       we.Weight,
				RestTime:     we.RestTime,
				Notes:        we.Notes,
			}

			for _, set := range we.WorkoutSets {
				exerciseExport.SetEntries = append(exerciseExport.SetEntries, WorkoutSetExport{
					SetNumber:   set.SetNumber,
					Reps:        set.Reps,
					Weight:      set.Weight,
					RPE:         set.RPE,
					RestSeconds: set.RestSeconds,
					Completed:   set.Completed,
				})
			}

			workoutExport.Exercises = append(workoutExport.Exercises, exerciseExport)
		}

		export.Workouts = append(export.Workouts, workoutExport)
	}

	// Get meals with foods
	var meals []models.Meal
	err = s.db.
		Preload("Items.Food").
		Where("user_id = ?", userID).
		Order("date desc").
		Find(&meals).Error
	if err != nil {
		return nil, err
	}

	for _, m := range meals {
		m.CalculateTotals()
		mealExport := MealExport{
			ID:            m.ID,
			MealType:      m.MealType,
			Date:          m.Date,
			Notes:         m.Notes,
			TotalCalories: m.TotalCalories,
			TotalProtein:  m.TotalProtein,
			TotalCarbs:    m.TotalCarbs,
			TotalFat:      m.TotalFat,
			CreatedAt:     m.CreatedAt,
		}

		for _, item := range m.Items {
			mealExport.Foods = append(mealExport.Foods, MealFoodExport{
				FoodID:   item.FoodID,
				FoodName: item.Food.Name,
				Quantity: item.Quantity,
				Calories: item.Food.Calories * item.Quantity,
				Protein:  item.Food.Protein * item.Quantity,
				Carbs:    item.Food.Carbohydrates * item.Quantity,
				Fat:      item.Food.Fat * item.Quantity,
			})
		}

		export.Meals = append(export.Meals, mealExport)
	}

	// Get weight entries
	var weightEntries []models.WeightEntry
	err = s.db.Where("user_id = ?", userID).Order("date desc").Find(&weightEntries).Error
	if err != nil {
		return nil, err
	}

	for _, we := range weightEntries {
		export.WeightEntries = append(export.WeightEntries, WeightEntryExport{
			ID:        we.ID,
			Weight:    we.Weight,
			Date:      we.Date,
			Notes:     we.Notes,
			CreatedAt: we.CreatedAt,
		})
	}

	// Get favorite foods
	var favoriteFoods []models.FavoriteFood
	err = s.db.
		Preload("Food").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&favoriteFoods).Error
	if err != nil {
		return nil, err
	}

	for _, ff := range favoriteFoods {
		export.FavoriteFoods = append(export.FavoriteFoods, FavoriteFoodExport{
			ID:        ff.ID,
			FoodID:    ff.FoodID,
			FoodName:  ff.Food.Name,
			CreatedAt: ff.CreatedAt,
		})
	}

	// Get recipes with items
	var recipes []models.Recipe
	err = s.db.
		Preload("Items.Food").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&recipes).Error
	if err != nil {
		return nil, err
	}

	for _, r := range recipes {
		recipeExport := RecipeExport{
			ID:        r.ID,
			Name:      r.Name,
			Servings:  r.Servings,
			Notes:     r.Notes,
			CreatedAt: r.CreatedAt,
		}

		for _, item := range r.Items {
			recipeExport.Items = append(recipeExport.Items, RecipeItemExport{
				FoodID:   item.FoodID,
				FoodName: item.Food.Name,
				Quantity: item.Quantity,
			})
		}

		export.Recipes = append(export.Recipes, recipeExport)
	}

	// Get workout templates with exercises and sets
	var templates []models.WorkoutTemplate
	err = s.db.
		Preload("WorkoutTemplateExercises.Exercise").
		Preload("WorkoutTemplateExercises.WorkoutTemplateSets").
		Where("owner_id = ?", userID).
		Order("created_at desc").
		Find(&templates).Error
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		templateExport := WorkoutTemplateExport{
			ID:        t.ID,
			Name:      t.Name,
			Type:      t.Type,
			Notes:     t.Notes,
			CreatedAt: t.CreatedAt,
		}

		for _, e := range t.WorkoutTemplateExercises {
			exerciseExport := WorkoutTemplateExerciseExport{
				ExerciseID:   e.ExerciseID,
				ExerciseName: e.Exercise.Name,
				Order:        e.Order,
				Sets:         e.Sets,
				Reps:         e.Reps,
				Weight:       e.Weight,
				RestTime:     e.RestTime,
				Notes:        e.Notes,
			}

			for _, s := range e.WorkoutTemplateSets {
				exerciseExport.SetTemplates = append(exerciseExport.SetTemplates, WorkoutTemplateSetExport{
					SetNumber:   s.SetNumber,
					Reps:        s.Reps,
					Weight:      s.Weight,
					RestSeconds: s.RestSeconds,
				})
			}

			templateExport.Exercises = append(templateExport.Exercises, exerciseExport)
		}

		export.WorkoutTemplates = append(export.WorkoutTemplates, templateExport)
	}

	// Get workout programs created by user
	var programs []models.WorkoutProgram
	err = s.db.
		Preload("Weeks.Sessions").
		Where("created_by = ?", userID).
		Order("created_at desc").
		Find(&programs).Error
	if err != nil {
		return nil, err
	}

	for _, p := range programs {
		programExport := WorkoutProgramExport{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			IsActive:    p.IsActive,
			CreatedAt:   p.CreatedAt,
		}

		for _, w := range p.Weeks {
			weekExport := ProgramWeekExport{
				WeekNumber: w.WeekNumber,
				Name:       w.Name,
			}

			for _, s := range w.Sessions {
				templateName := ""
				if s.Template != nil {
					templateName = s.Template.Name
				}
				weekExport.Sessions = append(weekExport.Sessions, ProgramSessionExport{
					DayNumber:    s.DayNumber,
					TemplateName: templateName,
					Notes:        s.Notes,
				})
			}

			programExport.Weeks = append(programExport.Weeks, weekExport)
		}

		export.WorkoutPrograms = append(export.WorkoutPrograms, programExport)
	}

	// Get program assignments
	var assignments []models.ProgramAssignment
	err = s.db.
		Preload("Program").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&assignments).Error
	if err != nil {
		return nil, err
	}

	for _, a := range assignments {
		programName := ""
		if a.Program.ID != uuid.Nil {
			programName = a.Program.Name
		}
		export.ProgramAssignments = append(export.ProgramAssignments, ProgramAssignmentExport{
			ID:          a.ID,
			ProgramID:   a.ProgramID,
			ProgramName: programName,
			AssignedAt:  a.AssignedAt,
			StartedAt:   a.StartedAt,
			CompletedAt: a.CompletedAt,
			Status:      a.Status,
		})
	}

	// Get notifications
	var notifications []models.Notification
	err = s.db.
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&notifications).Error
	if err != nil {
		return nil, err
	}

	for _, n := range notifications {
		export.Notifications = append(export.Notifications, NotificationExport{
			ID:          n.ID.String(),
			Type:        string(n.Type),
			Title:       n.Title,
			Message:     n.Message,
			PayloadJSON: n.PayloadJSON,
			ReadAt:      n.ReadAt,
			CreatedAt:   n.CreatedAt,
		})
	}

	return export, nil
}

// GetExportData returns the export data in the requested format.
func (s *ExportService) GetExportData(userID, jobID uuid.UUID) ([]byte, string, error) {
	job, err := s.GetExportJob(userID, jobID)
	if err != nil {
		return nil, "", err
	}

	if job.Status != ExportCompleted {
		return nil, "", errors.New("export job not completed")
	}

	exportData, err := s.generateExportData(userID, job.Format)
	if err != nil {
		return nil, "", err
	}

	switch job.Format {
	case ExportJSON:
		data, err := json.MarshalIndent(exportData, "", "  ")
		return data, "application/json", err
	case ExportCSV:
		data, err := exportToCSV(exportData)
		return data, "text/csv", err
	default:
		data, err := json.MarshalIndent(exportData, "", "  ")
		return data, "application/json", err
	}
}

// exportToCSV converts the export data to CSV format.
// It creates multiple CSV sections for all user data.
func exportToCSV(export *UserDataExport) ([]byte, error) {
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write user profile section
	writer.Write([]string{"# USER PROFILE"})
	writer.Write([]string{"field", "value"})
	writer.Write([]string{"user_id", export.UserID})
	writer.Write([]string{"export_id", export.ExportID})
	writer.Write([]string{"exported_at", export.ExportedAt.Format(time.RFC3339)})
	if export.User != nil {
		writer.Write([]string{"email", export.User.Email})
		writer.Write([]string{"name", export.User.Name})
		writer.Write([]string{"goal", export.User.Goal})
		writer.Write([]string{"activity_level", export.User.ActivityLevel})
	}
	writer.Write([]string{}) // Empty line

	// Write workouts section
	writer.Write([]string{"# WORKOUTS"})
	writer.Write([]string{"workout_id", "date", "duration", "type", "notes", "exercise_name", "sets", "reps", "weight", "set_number", "set_reps", "set_weight", "completed"})
	for _, w := range export.Workouts {
		for _, e := range w.Exercises {
			for _, s := range e.SetEntries {
				writer.Write([]string{
					w.ID.String(),
					w.Date.Format(time.RFC3339),
					fmt.Sprintf("%d", w.Duration),
					w.Type,
					w.Notes,
					e.ExerciseName,
					fmt.Sprintf("%d", e.Sets),
					fmt.Sprintf("%d", e.Reps),
					fmt.Sprintf("%.2f", e.Weight),
					fmt.Sprintf("%d", s.SetNumber),
					fmt.Sprintf("%d", s.Reps),
					fmt.Sprintf("%.2f", s.Weight),
					fmt.Sprintf("%t", s.Completed),
				})
			}
		}
	}
	writer.Write([]string{}) // Empty line

	// Write meals section
	writer.Write([]string{"# MEALS"})
	writer.Write([]string{"meal_id", "date", "meal_type", "notes", "total_calories", "total_protein", "total_carbs", "total_fat", "food_name", "quantity", "food_calories"})
	for _, m := range export.Meals {
		for _, f := range m.Foods {
			writer.Write([]string{
				m.ID.String(),
				m.Date.Format(time.RFC3339),
				m.MealType,
				m.Notes,
				fmt.Sprintf("%.2f", m.TotalCalories),
				fmt.Sprintf("%.2f", m.TotalProtein),
				fmt.Sprintf("%.2f", m.TotalCarbs),
				fmt.Sprintf("%.2f", m.TotalFat),
				f.FoodName,
				fmt.Sprintf("%.2f", f.Quantity),
				fmt.Sprintf("%.2f", f.Calories),
			})
		}
	}
	writer.Write([]string{}) // Empty line

	// Write weight entries section
	writer.Write([]string{"# WEIGHT ENTRIES"})
	writer.Write([]string{"entry_id", "date", "weight", "notes"})
	for _, we := range export.WeightEntries {
		writer.Write([]string{
			we.ID.String(),
			we.Date.Format(time.RFC3339),
			fmt.Sprintf("%.2f", we.Weight),
			we.Notes,
		})
	}
	writer.Write([]string{}) // Empty line

	// Write favorite foods section
	writer.Write([]string{"# FAVORITE FOODS"})
	writer.Write([]string{"favorite_id", "food_id", "food_name", "created_at"})
	for _, ff := range export.FavoriteFoods {
		writer.Write([]string{
			ff.ID.String(),
			ff.FoodID.String(),
			ff.FoodName,
			ff.CreatedAt.Format(time.RFC3339),
		})
	}
	writer.Write([]string{}) // Empty line

	// Write recipes section
	writer.Write([]string{"# RECIPES"})
	writer.Write([]string{"recipe_id", "name", "servings", "notes", "food_name", "quantity"})
	for _, r := range export.Recipes {
		for _, item := range r.Items {
			writer.Write([]string{
				r.ID.String(),
				r.Name,
				fmt.Sprintf("%d", r.Servings),
				r.Notes,
				item.FoodName,
				fmt.Sprintf("%.2f", item.Quantity),
			})
		}
	}
	writer.Write([]string{}) // Empty line

	// Write workout templates section
	writer.Write([]string{"# WORKOUT TEMPLATES"})
	writer.Write([]string{"template_id", "name", "type", "notes", "exercise_name", "order", "sets", "reps", "weight", "set_number", "set_reps", "set_weight"})
	for _, t := range export.WorkoutTemplates {
		for _, e := range t.Exercises {
			for _, s := range e.SetTemplates {
				writer.Write([]string{
					t.ID.String(),
					t.Name,
					t.Type,
					t.Notes,
					e.ExerciseName,
					fmt.Sprintf("%d", e.Order),
					fmt.Sprintf("%d", e.Sets),
					fmt.Sprintf("%d", e.Reps),
					fmt.Sprintf("%.2f", e.Weight),
					fmt.Sprintf("%d", s.SetNumber),
					fmt.Sprintf("%d", s.Reps),
					fmt.Sprintf("%.2f", s.Weight),
				})
			}
		}
	}
	writer.Write([]string{}) // Empty line

	// Write workout programs section
	writer.Write([]string{"# WORKOUT PROGRAMS"})
	writer.Write([]string{"program_id", "name", "description", "is_active", "week_number", "week_name", "day_number", "session_notes"})
	for _, p := range export.WorkoutPrograms {
		for _, w := range p.Weeks {
			for _, s := range w.Sessions {
				writer.Write([]string{
					p.ID.String(),
					p.Name,
					p.Description,
					fmt.Sprintf("%t", p.IsActive),
					fmt.Sprintf("%d", w.WeekNumber),
					w.Name,
					fmt.Sprintf("%d", s.DayNumber),
					s.Notes,
				})
			}
		}
	}
	writer.Write([]string{}) // Empty line

	// Write program assignments section
	writer.Write([]string{"# PROGRAM ASSIGNMENTS"})
	writer.Write([]string{"assignment_id", "program_id", "program_name", "assigned_at", "started_at", "completed_at", "status"})
	for _, a := range export.ProgramAssignments {
		startedAt := ""
		if a.StartedAt != nil {
			startedAt = a.StartedAt.Format(time.RFC3339)
		}
		completedAt := ""
		if a.CompletedAt != nil {
			completedAt = a.CompletedAt.Format(time.RFC3339)
		}
		writer.Write([]string{
			a.ID.String(),
			a.ProgramID.String(),
			a.ProgramName,
			a.AssignedAt.Format(time.RFC3339),
			startedAt,
			completedAt,
			a.Status,
		})
	}
	writer.Write([]string{}) // Empty line

	// Write notifications section
	writer.Write([]string{"# NOTIFICATIONS"})
	writer.Write([]string{"notification_id", "type", "title", "message", "payload_json", "read_at", "created_at"})
	for _, n := range export.Notifications {
		readAt := ""
		if n.ReadAt != nil {
			readAt = n.ReadAt.Format(time.RFC3339)
		}
		writer.Write([]string{
			n.ID,
			n.Type,
			n.Title,
			n.Message,
			n.PayloadJSON,
			readAt,
			n.CreatedAt.Format(time.RFC3339),
		})
	}

	writer.Flush()
	if writer.Error() != nil {
		return nil, writer.Error()
	}

	return []byte(buf.String()), nil
}

// CreateDeletionRequest creates a deletion request for a user.
func (s *ExportService) CreateDeletionRequest(userID uuid.UUID) (*DeletionRequest, error) {
	// Check if there's already a pending request
	var existing DeletionRequest
	err := s.db.First(&existing, "user_id = ? AND processed_at IS NULL", userID).Error
	if err == nil {
		return nil, errors.New("deletion request already pending")
	}

	request := DeletionRequest{
		UserID:      userID,
		RequestedAt: time.Now().UTC(),
		Status:      "pending",
	}

	if err := s.db.Create(&request).Error; err != nil {
		return nil, err
	}

	return &request, nil
}

// ProcessDeletionRequest processes a deletion request.
// This permanently deletes all user data.
func (s *ExportService) ProcessDeletionRequest(userID uuid.UUID) error {
	// Start transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete all workout-related data
		var workoutIDs []uuid.UUID
		if err := tx.Model(&models.Workout{}).Where("user_id = ?", userID).Pluck("id", &workoutIDs).Error; err != nil {
			return err
		}

		if len(workoutIDs) > 0 {
			// Delete workout cardio entries
			if err := tx.Where("workout_id IN ?", workoutIDs).Delete(&models.WorkoutCardioEntry{}).Error; err != nil {
				return err
			}
			// Delete workout sets through workout exercises
			if err := tx.Exec("DELETE FROM workout_sets WHERE workout_exercise_id IN (SELECT id FROM workout_exercises WHERE workout_id IN ?)", workoutIDs).Error; err != nil {
				return err
			}
			if err := tx.Where("workout_id IN ?", workoutIDs).Delete(&models.WorkoutExercise{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", workoutIDs).Delete(&models.Workout{}).Error; err != nil {
				return err
			}
		}

		// Delete meal foods
		if err := tx.Exec("DELETE FROM meal_foods WHERE meal_id IN (SELECT id FROM meals WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}

		// Delete meals
		if err := tx.Where("user_id = ?", userID).Delete(&models.Meal{}).Error; err != nil {
			return err
		}

		// Delete weight entries
		if err := tx.Where("user_id = ?", userID).Delete(&models.WeightEntry{}).Error; err != nil {
			return err
		}

		// Delete favorite foods
		if err := tx.Where("user_id = ?", userID).Delete(&models.FavoriteFood{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&models.TwoFactorSecret{}).Error; err != nil {
			return err
		}

		// Delete recipe items and recipes
		if err := tx.Exec("DELETE FROM recipe_items WHERE recipe_id IN (SELECT id FROM recipes WHERE user_id = ?)", userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.Recipe{}).Error; err != nil {
			return err
		}

		// Delete workout templates (owner_id)
		var templateIDs []uuid.UUID
		if err := tx.Model(&models.WorkoutTemplate{}).Where("owner_id = ?", userID).Pluck("id", &templateIDs).Error; err != nil {
			return err
		}
		if len(templateIDs) > 0 {
			// Delete template sets through template exercises
			if err := tx.Exec("DELETE FROM workout_template_sets WHERE template_exercise_id IN (SELECT id FROM workout_template_exercises WHERE template_id IN ?)", templateIDs).Error; err != nil {
				return err
			}
			if err := tx.Where("template_id IN ?", templateIDs).Delete(&models.WorkoutTemplateExercise{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", templateIDs).Delete(&models.WorkoutTemplate{}).Error; err != nil {
				return err
			}
		}

		// Delete program assignments (user_id)
		if err := tx.Where("user_id = ?", userID).Delete(&models.ProgramAssignment{}).Error; err != nil {
			return err
		}

		// Delete workout programs created by user (created_by)
		var programIDs []uuid.UUID
		if err := tx.Model(&models.WorkoutProgram{}).Where("created_by = ?", userID).Pluck("id", &programIDs).Error; err != nil {
			return err
		}
		if len(programIDs) > 0 {
			// Delete program sessions through weeks
			if err := tx.Exec("DELETE FROM program_sessions WHERE week_id IN (SELECT id FROM program_weeks WHERE program_id IN ?)", programIDs).Error; err != nil {
				return err
			}
			if err := tx.Where("program_id IN ?", programIDs).Delete(&models.ProgramWeek{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", programIDs).Delete(&models.WorkoutProgram{}).Error; err != nil {
				return err
			}
		}

		// Delete export jobs (but keep deletion request record)
		if err := tx.Where("user_id = ?", userID).Delete(&ExportJob{}).Error; err != nil {
			return err
		}

		// Delete refresh tokens
		if err := tx.Where("user_id = ?", userID).Delete(&RefreshToken{}).Error; err != nil {
			return err
		}

		// Delete sessions
		if err := tx.Where("user_id = ?", userID).Delete(&UserSession{}).Error; err != nil {
			return err
		}

		// Delete notifications
		if err := tx.Where("user_id = ?", userID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}

		// Finally, delete the user
		if err := tx.Delete(&models.User{}, "id = ?", userID).Error; err != nil {
			return err
		}

		// Mark deletion request as processed
		now := time.Now().UTC()
		return tx.Model(&DeletionRequest{}).
			Where("user_id = ? AND processed_at IS NULL", userID).
			Updates(map[string]interface{}{
				"processed_at": now,
				"status":       "processed",
			}).Error
	})
}

// CancelDeletionRequest cancels a pending deletion request.
func (s *ExportService) CancelDeletionRequest(userID uuid.UUID) error {
	result := s.db.Where("user_id = ? AND processed_at IS NULL", userID).Delete(&DeletionRequest{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("no pending deletion request")
	}
	return nil
}
