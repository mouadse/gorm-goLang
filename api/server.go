package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/metrics"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Server struct {
	db              *gorm.DB
	mux             *http.ServeMux
	metrics         *metrics.Metrics
	cors            corsConfig
	authSvc         *services.AuthService
	analyticsSvc    *services.WorkoutAnalyticsService
	adherenceSvc    *services.AdherenceService
	importSvc       *services.USDAImportService
	exportSvc       *services.ExportService
	notificationSvc *services.NotificationService
	twoFactorSvc    *services.TwoFactorService
	twoFactorLimit  *twoFactorAttemptLimiter
	twoFactorTokens *twoFactorChallengeStore
	adminSvc        *services.AdminDashboardService
	llmClient       services.LLMClient
	coachSvc        *services.CoachService
	exerciseLibSvc  *services.ExerciseLibClient
	leaderboardSvc  *services.LeaderboardService
}

func NewServer(db *gorm.DB) *Server {
	m := metrics.New()
	adminSvc := services.NewAdminDashboardService(db, m)

	server := &Server{
		db:              db,
		mux:             http.NewServeMux(),
		metrics:         m,
		cors:            newCORSConfigFromEnv(),
		authSvc:         services.NewAuthService(db),
		analyticsSvc:    services.NewWorkoutAnalyticsService(db),
		adherenceSvc:    services.NewAdherenceService(db),
		importSvc:       services.NewUSDAImportService(db),
		exportSvc:       services.NewExportService(db, m),
		notificationSvc: services.NewNotificationService(db, m),
		twoFactorSvc:    services.NewTwoFactorService(db),
		twoFactorLimit:  newTwoFactorAttemptLimiter(),
		twoFactorTokens: newTwoFactorChallengeStore(),
		adminSvc:        adminSvc,
		llmClient:       services.NewOpenRouterClient("", ""),
		exerciseLibSvc:  services.NewExerciseLibClient(m),
		leaderboardSvc:  services.NewLeaderboardService(db),
	}
	server.coachSvc = services.NewCoachService(db, server.analyticsSvc, server.adherenceSvc, services.NewNutritionTargetService(db), services.NewIntegrationRulesService(db), server.notificationSvc, server.exerciseLibSvc)

	// Setup DB metrics: GORM callback plugin + periodic connection pool stats
	if err := metrics.NewGORMCallbackPlugin(m).Initialize(db); err != nil {
		panic(fmt.Sprintf("failed to register GORM metrics plugin: %v", err))
	}
	if sqlDB, err := db.DB(); err == nil {
		ticker := time.NewTicker(10 * time.Second)
		go m.TrackDBConnStats(sqlDB, ticker.C)
	}

	server.registerRoutes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.cors.Middleware(s.metrics.Middleware(s.mux))
}

func (s *Server) registerRoutes() {
	// Metrics endpoint (not instrumented to avoid recursion)
	s.mux.Handle("GET /metrics", s.metrics.Handler())

	protected := func(pattern string, handler http.HandlerFunc) {
		s.mux.Handle(pattern, Authenticate(s.db, http.HandlerFunc(handler)))
	}

	s.mux.HandleFunc("GET /livez", s.handleLive)
	s.mux.HandleFunc("GET /healthz", s.handleReady)
	s.mux.HandleFunc("GET /readyz", s.handleReady)
	s.mux.HandleFunc("GET /openapi.yaml", s.handleOpenAPISpec)
	s.mux.HandleFunc("GET /docs", s.handleSwaggerUI)
	s.mux.HandleFunc("GET /docs/", s.handleSwaggerUI)
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("GET /register", s.handleRegisterPage)

	// Serve static frontend files
	s.mux.Handle("/", http.FileServer(http.Dir("frontend")))

	s.mux.HandleFunc("POST /v1/auth/register", s.handleRegisterWithSessions)
	s.mux.HandleFunc("POST /v1/auth/login", s.handleLoginWithSessions)
	s.mux.HandleFunc("POST /v1/auth/refresh", s.handleRefreshToken)
	protected("POST /v1/auth/2fa/setup", s.handleSetupTwoFactor)
	protected("POST /v1/auth/2fa/verify", s.handleVerifyTwoFactor)
	protected("POST /v1/auth/2fa/disable", s.handleDisableTwoFactor)
	s.mux.HandleFunc("POST /v1/auth/2fa/recover", s.handleRecoverWithTwoFactor)
	protected("POST /v1/auth/logout", s.handleLogout)
	protected("GET /v1/auth/sessions", s.handleGetSessions)
	protected("DELETE /v1/auth/sessions/{id}", s.handleDeleteSession)

	// Users
	s.mux.HandleFunc("POST /v1/users", s.handleCreateUser)
	protected("GET /v1/users", s.handleListUsers)
	protected("GET /v1/users/{id}", s.handleGetUser)
	protected("PATCH /v1/users/{id}", s.handleUpdateUser)
	protected("DELETE /v1/users/{id}", s.handleDeleteUser)
	protected("GET /v1/users/{user_id}/summary", s.handleGetDailySummary)
	protected("GET /v1/summary", s.handleGetDailySummary)
	protected("GET /v1/users/{user_id}/weekly-summary", s.handleGetWeeklySummary)
	protected("GET /v1/weekly-summary", s.handleGetWeeklySummary)
	protected("GET /v1/users/{user_id}/recommendations", s.handleGetRecommendations)
	protected("GET /v1/recommendations", s.handleGetRecommendations)
	protected("GET /v1/users/{user_id}/nutrition-targets", s.handleGetUserNutritionTargets)

	// Phase 2: Analytics & Adherence
	protected("GET /v1/users/{user_id}/records", s.handleGetUserRecords)
	protected("GET /v1/users/{user_id}/workout-stats", s.handleGetUserWorkoutStats)
	protected("GET /v1/users/{user_id}/activity-calendar", s.handleGetUserActivityCalendar)
	protected("GET /v1/users/{user_id}/streaks", s.handleGetUserStreaks)

	// Chat & AI Coach
	protected("POST /v1/chat", s.handleChat)
	protected("GET /v1/chat/history", s.handleChatHistory)
	protected("POST /v1/chat/feedback", s.handleChatFeedback)
	protected("GET /v1/users/{user_id}/coach-summary", s.handleCoachSummary)

	// Exercises
	protected("POST /v1/exercises", s.handleCreateExercise)
	s.mux.Handle("GET /v1/leaderboard", Authenticate(s.db, http.HandlerFunc(s.handleGetLeaderboard)))
	s.mux.HandleFunc("GET /v1/exercises", s.handleListExercises)
	s.mux.HandleFunc("GET /v1/exercises/{id}", s.handleGetExercise)
	protected("PATCH /v1/exercises/{id}", s.handleUpdateExercise)
	protected("DELETE /v1/exercises/{id}", s.handleDeleteExercise)
	protected("GET /v1/exercises/{id}/history", s.handleGetExerciseHistory)

	// Exercise library proxy endpoints
	s.mux.HandleFunc("POST /v1/exercises/search", s.handleSearchExercises)
	s.mux.HandleFunc("POST /v1/exercises/program", s.handleGenerateProgram)
	s.mux.HandleFunc("GET /v1/exercises/library-meta", s.handleExerciseLibraryMeta)
	s.mux.HandleFunc("GET /v1/exercise-images/{path...}", s.handleExerciseImageProxy)

	// Weight entries
	protected("POST /v1/weight-entries", s.handleCreateWeightEntry)
	protected("GET /v1/weight-entries", s.handleListWeightEntries)
	protected("GET /v1/users/{user_id}/weight-entries", s.handleListWeightEntries)
	protected("POST /v1/users/{user_id}/weight-entries", s.handleCreateWeightEntry)
	protected("GET /v1/weight-entries/{id}", s.handleGetWeightEntry)
	protected("PATCH /v1/weight-entries/{id}", s.handleUpdateWeightEntry)
	protected("DELETE /v1/weight-entries/{id}", s.handleDeleteWeightEntry)

	// Workouts
	protected("POST /v1/workouts", s.handleCreateWorkout)
	protected("GET /v1/workouts", s.handleListWorkouts)
	protected("GET /v1/users/{user_id}/workouts", s.handleListWorkouts)
	protected("POST /v1/users/{user_id}/workouts", s.handleCreateWorkout)
	protected("GET /v1/workouts/{id}", s.handleGetWorkout)
	protected("PATCH /v1/workouts/{id}", s.handleUpdateWorkout)
	protected("DELETE /v1/workouts/{id}", s.handleDeleteWorkout)

	// Workout exercises
	protected("POST /v1/workout-exercises", s.handleCreateWorkoutExercise)
	protected("GET /v1/workout-exercises", s.handleListWorkoutExercises)
	protected("GET /v1/workouts/{id}/exercises", s.handleListWorkoutExercises)
	protected("POST /v1/workouts/{id}/exercises", s.handleAddWorkoutExercise)
	protected("GET /v1/workout-exercises/{id}", s.handleGetWorkoutExercise)
	protected("PATCH /v1/workout-exercises/{id}", s.handleUpdateWorkoutExercise)
	protected("DELETE /v1/workout-exercises/{id}", s.handleDeleteWorkoutExercise)

	// Workout sets
	protected("POST /v1/workout-sets", s.handleCreateWorkoutSet)
	protected("GET /v1/workout-sets", s.handleListWorkoutSets)
	protected("GET /v1/workout-exercises/{id}/sets", s.handleListWorkoutSets)
	protected("POST /v1/workout-exercises/{id}/sets", s.handleCreateWorkoutSet)
	protected("GET /v1/workout-sets/{id}", s.handleGetWorkoutSet)
	protected("PATCH /v1/workout-sets/{id}", s.handleUpdateWorkoutSet)
	protected("DELETE /v1/workout-sets/{id}", s.handleDeleteWorkoutSet)

	// Meals
	protected("POST /v1/meals", s.handleCreateMeal)
	protected("GET /v1/meals", s.handleListMeals)
	protected("GET /v1/users/{user_id}/meals", s.handleListMeals)
	protected("POST /v1/users/{user_id}/meals", s.handleCreateMeal)
	protected("GET /v1/meals/{id}", s.handleGetMeal)
	protected("PATCH /v1/meals/{id}", s.handleUpdateMeal)
	protected("DELETE /v1/meals/{id}", s.handleDeleteMeal)
	protected("GET /v1/meals/recent", s.handleGetRecentMeals)
	protected("POST /v1/meals/{id}/clone", s.handleCloneMeal)

	// Foods
	s.mux.HandleFunc("GET /v1/foods", s.handleListFoods)
	s.mux.HandleFunc("GET /v1/foods/{id}", s.handleGetFood)
	protected("POST /v1/foods", s.handleCreateFood)
	protected("PATCH /v1/foods/{id}", s.handleUpdateFood)
	protected("DELETE /v1/foods/{id}", s.handleDeleteFood)
	protected("GET /v1/foods/recent", s.handleGetRecentFoods)
	protected("POST /v1/foods/{id}/favorite", s.handleFavoriteFood)
	protected("DELETE /v1/foods/{id}/favorite", s.handleUnfavoriteFood)
	protected("GET /v1/users/{user_id}/favorites", s.handleGetFavorites)

	// Meal foods
	protected("POST /v1/meal-foods", s.handleCreateMealFood)
	protected("GET /v1/meals/{id}/foods", s.handleListMealFoods)
	protected("POST /v1/meals/{id}/foods", s.handleCreateMealFood)
	protected("GET /v1/meal-foods/{id}", s.handleGetMealFood)
	protected("PATCH /v1/meal-foods/{id}", s.handleUpdateMealFood)
	protected("DELETE /v1/meal-foods/{id}", s.handleDeleteMealFood)

	// Workout cardio entries
	protected("GET /v1/workouts/{id}/cardio", s.handleListWorkoutCardio)
	protected("POST /v1/workouts/{id}/cardio", s.handleCreateCardioEntry)
	protected("GET /v1/workout-cardio/{id}", s.handleGetCardioEntry)
	protected("PATCH /v1/workout-cardio/{id}", s.handleUpdateCardioEntry)
	protected("DELETE /v1/workout-cardio/{id}", s.handleDeleteCardioEntry)

	// Workout templates
	protected("POST /v1/workout-templates", s.handleCreateTemplate)
	protected("GET /v1/workout-templates", s.handleListTemplates)
	protected("GET /v1/workout-templates/{id}", s.handleGetTemplate)
	protected("PATCH /v1/workout-templates/{id}", s.handleUpdateTemplate)
	protected("DELETE /v1/workout-templates/{id}", s.handleDeleteTemplate)
	protected("POST /v1/workout-templates/{id}/apply", s.handleApplyTemplate)

	// Workout programs: user-side assignment retrieval and application
	protected("GET /v1/program-assignments", s.handleListProgramAssignments)
	protected("GET /v1/users/{user_id}/program-assignments", s.handleListProgramAssignments)
	protected("GET /v1/program-assignments/{id}", s.handleGetProgramAssignment)
	protected("PATCH /v1/program-assignments/{id}/status", s.handleUpdateOwnProgramAssignmentStatus)
	protected("POST /v1/program-sessions/{id}/apply", s.handleApplyProgramSession)

	// Recipes
	protected("POST /v1/recipes", s.handleCreateRecipe)
	protected("GET /v1/recipes", s.handleListRecipes)
	protected("GET /v1/recipes/{id}", s.handleGetRecipe)
	protected("PATCH /v1/recipes/{id}", s.handleUpdateRecipe)
	protected("DELETE /v1/recipes/{id}", s.handleDeleteRecipe)
	protected("GET /v1/recipes/{id}/nutrition", s.handleGetRecipeNutrition)
	protected("POST /v1/recipes/{id}/log-to-meal", s.handleLogRecipeToMeal)

	// Admin Dashboard
	admin := func(pattern string, handler http.HandlerFunc) {
		s.mux.Handle(pattern, Authenticate(s.db, RequireAdmin(s.db, http.HandlerFunc(handler))))
	}

	admin("GET /v1/admin/dashboard/summary", s.handleAdminDashboardSummary)
	admin("GET /v1/admin/dashboard/trends", s.handleAdminDashboardTrends)
	admin("GET /v1/admin/users/stats", s.handleAdminUserStats)
	admin("GET /v1/admin/users/growth", s.handleAdminUserGrowth)
	admin("GET /v1/admin/workouts/stats", s.handleAdminWorkoutStats)
	admin("GET /v1/admin/workouts/exercises/popular", s.handleAdminPopularExercises)
	admin("GET /v1/admin/nutrition/stats", s.handleAdminNutritionStats)
	admin("GET /v1/admin/moderation/stats", s.handleAdminModerationStats)
	admin("GET /v1/admin/system/health", s.handleAdminSystemHealth)
	admin("GET /v1/admin/audit-logs", s.handleAdminListAuditLogs)
	s.mux.Handle("GET /v1/admin/dashboard/realtime", Authenticate(s.db, RequireAdmin(s.db, http.HandlerFunc(s.handleAdminRealtimeWS))))

	// Admin: User Management
	admin("GET /v1/admin/users", s.handleAdminListUsers)
	admin("GET /v1/admin/users/{id}", s.handleAdminGetUser)
	admin("PATCH /v1/admin/users/{id}", s.handleAdminUpdateUser)
	admin("DELETE /v1/admin/users/{id}", s.handleAdminDeleteUser)
	admin("POST /v1/admin/users/{id}/ban", s.handleAdminBanUser)
	admin("POST /v1/admin/users/{id}/unban", s.handleAdminUnbanUser)

	// Admin: Workout Programs
	admin("POST /v1/programs", s.handleCreateProgram)
	admin("GET /v1/programs", s.handleListPrograms)
	admin("GET /v1/programs/{id}", s.handleGetProgram)
	admin("PATCH /v1/programs/{id}", s.handleUpdateProgram)
	admin("DELETE /v1/programs/{id}", s.handleDeleteProgram)
	admin("POST /v1/programs/{id}/weeks", s.handleCreateProgramWeek)
	admin("POST /v1/programs/{id}/assignments", s.handleCreateProgramAssignment)
	admin("GET /v1/programs/{id}/assignments", s.handleListProgramAssignmentsForProgram)
	admin("GET /v1/program-weeks/{id}", s.handleGetProgramWeek)
	admin("PATCH /v1/program-weeks/{id}", s.handleUpdateProgramWeek)
	admin("DELETE /v1/program-weeks/{id}", s.handleDeleteProgramWeek)
	admin("POST /v1/program-weeks/{id}/sessions", s.handleCreateProgramSession)
	admin("GET /v1/program-sessions/{id}", s.handleGetProgramSession)
	admin("PATCH /v1/program-sessions/{id}", s.handleUpdateProgramSession)
	admin("DELETE /v1/program-sessions/{id}", s.handleDeleteProgramSession)
	admin("PATCH /v1/admin/program-assignments/{id}", s.handleAdminUpdateProgramAssignment)
	admin("DELETE /v1/admin/program-assignments/{id}", s.handleDeleteProgramAssignment)

	// Admin: USDA Food Import
	s.mux.Handle("POST /v1/admin/import-usda", Authenticate(s.db, RequireAdmin(s.db, http.HandlerFunc(s.handleImportUSDA))))

	// Exports & GDPR
	protected("POST /v1/exports", s.handleCreateExportJob)
	protected("GET /v1/exports/{id}", s.handleGetExportJob)
	protected("POST /v1/account/delete-request", s.handleCreateDeletionRequest)

	// Notifications
	protected("GET /v1/notifications", s.handleListNotifications)
	protected("PATCH /v1/notifications/{id}/read", s.handleMarkNotificationRead)
	protected("PATCH /v1/notifications/read-all", s.handleMarkAllNotificationsRead)
	protected("GET /v1/notifications/unread-count", s.handleGetUnreadNotificationCount)
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	health, err := s.adminSvc.GetSystemHealth(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}

	status, _ := health["status"].(string)
	if status != "healthy" {
		writeJSON(w, http.StatusServiceUnavailable, health)
		return
	}

	writeJSON(w, http.StatusOK, health)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, validationProblem(status, err))
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	if decoder.More() {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func parsePathUUID(r *http.Request, field string) (uuid.UUID, error) {
	value := strings.TrimSpace(r.PathValue(field))
	if value == "" {
		return uuid.Nil, fmt.Errorf("missing path parameter: %s", field)
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid for %s", field)
	}
	return parsed, nil
}

func parseRequiredUUID(field, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, fmt.Errorf("%s is required", field)
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid for %s", field)
	}
	return parsed, nil
}
