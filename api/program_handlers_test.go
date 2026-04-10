package api_test

import (
	"fitness-tracker/api"
	"net/http"
	"testing"

	"fitness-tracker/models"

	"gorm.io/gorm"
)

type programFixture struct {
	db        *gorm.DB
	handler   http.Handler
	adminAuth authEnvelope
	userAuth  authEnvelope
	program   models.WorkoutProgram
	template  models.WorkoutTemplate
}

func newProgramFixture(t *testing.T) programFixture {
	t.Helper()

	db, handler := newTestApp(t)

	adminAuth := registerTestUser(t, handler, "program-admin@example.com", "Program Admin", "password123")
	if err := db.Model(&models.User{}).Where("id = ?", adminAuth.User.ID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	userAuth := registerTestUser(t, handler, "program-user@example.com", "Program User", "password123")

	exercise := requestJSONAuth[models.Exercise](t, handler, adminAuth.AccessToken, http.MethodPost, "/v1/exercises", map[string]any{
		"name":            "Program Bench Press",
		"primary_muscles": "Chest",
		"equipment":       "Barbell",
	}, http.StatusCreated)

	template := requestJSONAuth[models.WorkoutTemplate](t, handler, adminAuth.AccessToken, http.MethodPost, "/v1/workout-templates", map[string]any{
		"owner_id": adminAuth.User.ID,
		"name":     "Program Push Template",
		"type":     "push",
		"notes":    "Template notes",
		"exercises": []map[string]any{
			{
				"exercise_id": exercise.ID,
				"order":       1,
				"sets":        1,
				"reps":        8,
				"weight":      80,
				"set_entries": []map[string]any{
					{"set_number": 1, "reps": 8, "weight": 80},
				},
			},
		},
	}, http.StatusCreated)

	program := requestJSONAuth[models.WorkoutProgram](t, handler, adminAuth.AccessToken, http.MethodPost, "/v1/programs", map[string]any{
		"name":        "12 Week Strength",
		"description": "Strength block",
		"weeks": []map[string]any{
			{
				"week_number": 1,
				"name":        "Base",
				"sessions": []map[string]any{
					{
						"day_number":          1,
						"workout_template_id": template.ID,
						"notes":               "Program session notes",
					},
				},
			},
		},
	}, http.StatusCreated)

	return programFixture{
		db:        db,
		handler:   handler,
		adminAuth: adminAuth,
		userAuth:  userAuth,
		program:   program,
		template:  template,
	}
}

func TestProgramCRUDAssignmentAndSessionApply(t *testing.T) {
	fixture := newProgramFixture(t)
	handler := fixture.handler
	adminAuth := fixture.adminAuth
	userAuth := fixture.userAuth
	program := fixture.program

	requestErrorAuth(t, handler, userAuth.AccessToken, http.MethodPost, "/v1/programs", map[string]any{
		"name": "Unauthorized Program",
	}, http.StatusForbidden)

	if len(program.Weeks) != 1 || len(program.Weeks[0].Sessions) != 1 {
		t.Fatalf("expected created program with one week/session, got %#v", program.Weeks)
	}

	assignment := requestJSONAuth[models.ProgramAssignment](t, handler, adminAuth.AccessToken, http.MethodPost, "/v1/programs/"+program.ID.String()+"/assignments", map[string]any{
		"user_id": userAuth.User.ID,
	}, http.StatusCreated)
	if assignment.UserID != userAuth.User.ID || assignment.ProgramID != program.ID {
		t.Fatalf("unexpected assignment user/program: %#v", assignment)
	}

	assignments := requestJSONAuth[api.PaginatedResponse[models.ProgramAssignment]](t, handler, userAuth.AccessToken, http.MethodGet, "/v1/program-assignments", nil, http.StatusOK).Data
	if len(assignments) != 1 {
		t.Fatalf("expected one user assignment, got %d", len(assignments))
	}
	if len(assignments[0].Program.Weeks) != 1 {
		t.Fatalf("expected assigned program details to be preloaded")
	}

	sessionID := program.Weeks[0].Sessions[0].ID
	workout := requestJSONAuth[models.Workout](t, handler, userAuth.AccessToken, http.MethodPost, "/v1/program-sessions/"+sessionID.String()+"/apply", map[string]any{
		"date": "2026-04-10",
	}, http.StatusCreated)
	if workout.UserID != userAuth.User.ID {
		t.Fatalf("expected workout user %s, got %s", userAuth.User.ID, workout.UserID)
	}
	if len(workout.WorkoutExercises) != 1 {
		t.Fatalf("expected applied template exercise, got %d", len(workout.WorkoutExercises))
	}
	if workout.Notes != "Template notes\n\nProgram notes: Program session notes" {
		t.Fatalf("unexpected workout notes %q", workout.Notes)
	}

	updated := requestJSONAuth[models.ProgramAssignment](t, handler, userAuth.AccessToken, http.MethodPatch, "/v1/program-assignments/"+assignment.ID.String()+"/status", map[string]any{
		"status": "completed",
	}, http.StatusOK)
	if updated.Status != "completed" || updated.CompletedAt == nil {
		t.Fatalf("expected completed assignment with completed_at, got %#v", updated)
	}
}

func TestUpdateProgramSessionHandlesNullTemplateClear(t *testing.T) {
	fixture := newProgramFixture(t)

	sessionID := fixture.program.Weeks[0].Sessions[0].ID
	updated := requestJSONAuth[models.ProgramSession](t, fixture.handler, fixture.adminAuth.AccessToken, http.MethodPatch, "/v1/program-sessions/"+sessionID.String(), map[string]any{
		"workout_template_id": nil,
	}, http.StatusOK)
	if updated.WorkoutTemplateID != nil {
		t.Fatalf("expected workout_template_id to be cleared, got %v", *updated.WorkoutTemplateID)
	}
	if updated.Template != nil {
		t.Fatalf("expected template preload to be cleared, got %#v", updated.Template)
	}

	var stored models.ProgramSession
	if err := fixture.db.First(&stored, "id = ?", sessionID).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}
	if stored.WorkoutTemplateID != nil {
		t.Fatalf("expected stored workout_template_id to be nil, got %v", *stored.WorkoutTemplateID)
	}
}

func TestApplyProgramSessionDoesNotStartAssignmentOnFailedApply(t *testing.T) {
	fixture := newProgramFixture(t)

	assignment := requestJSONAuth[models.ProgramAssignment](t, fixture.handler, fixture.adminAuth.AccessToken, http.MethodPost, "/v1/programs/"+fixture.program.ID.String()+"/assignments", map[string]any{
		"user_id": fixture.userAuth.User.ID,
	}, http.StatusCreated)

	if err := fixture.db.Delete(&models.WorkoutTemplate{}, "id = ?", fixture.template.ID).Error; err != nil {
		t.Fatalf("delete template: %v", err)
	}

	sessionID := fixture.program.Weeks[0].Sessions[0].ID
	requestErrorAuth(t, fixture.handler, fixture.userAuth.AccessToken, http.MethodPost, "/v1/program-sessions/"+sessionID.String()+"/apply", map[string]any{
		"date": "2026-04-10",
	}, http.StatusNotFound)

	var stored models.ProgramAssignment
	if err := fixture.db.First(&stored, "id = ?", assignment.ID).Error; err != nil {
		t.Fatalf("load assignment: %v", err)
	}
	if stored.Status != "assigned" {
		t.Fatalf("expected assignment status to remain assigned, got %q", stored.Status)
	}
	if stored.StartedAt != nil {
		t.Fatalf("expected assignment started_at to remain nil, got %v", *stored.StartedAt)
	}

	var workoutCount int64
	if err := fixture.db.Model(&models.Workout{}).Where("user_id = ?", fixture.userAuth.User.ID).Count(&workoutCount).Error; err != nil {
		t.Fatalf("count workouts: %v", err)
	}
	if workoutCount != 0 {
		t.Fatalf("expected no workouts to be created, got %d", workoutCount)
	}
}

func TestReactivatingProgramAssignmentClearsCompletedAtAndRemainsActive(t *testing.T) {
	fixture := newProgramFixture(t)

	assignment := requestJSONAuth[models.ProgramAssignment](t, fixture.handler, fixture.adminAuth.AccessToken, http.MethodPost, "/v1/programs/"+fixture.program.ID.String()+"/assignments", map[string]any{
		"user_id": fixture.userAuth.User.ID,
	}, http.StatusCreated)

	completed := requestJSONAuth[models.ProgramAssignment](t, fixture.handler, fixture.userAuth.AccessToken, http.MethodPatch, "/v1/program-assignments/"+assignment.ID.String()+"/status", map[string]any{
		"status": "completed",
	}, http.StatusOK)
	if completed.CompletedAt == nil {
		t.Fatalf("expected completed assignment to have completed_at")
	}

	reactivated := requestJSONAuth[models.ProgramAssignment](t, fixture.handler, fixture.userAuth.AccessToken, http.MethodPatch, "/v1/program-assignments/"+assignment.ID.String()+"/status", map[string]any{
		"status": "in_progress",
	}, http.StatusOK)
	if reactivated.Status != "in_progress" {
		t.Fatalf("expected reactivated status in_progress, got %q", reactivated.Status)
	}
	if reactivated.CompletedAt != nil {
		t.Fatalf("expected completed_at to be cleared on reactivation, got %v", *reactivated.CompletedAt)
	}

	requestErrorAuth(t, fixture.handler, fixture.adminAuth.AccessToken, http.MethodPost, "/v1/programs/"+fixture.program.ID.String()+"/assignments", map[string]any{
		"user_id": fixture.userAuth.User.ID,
	}, http.StatusConflict)

	var stored models.ProgramAssignment
	if err := fixture.db.First(&stored, "id = ?", assignment.ID).Error; err != nil {
		t.Fatalf("load assignment: %v", err)
	}
	if stored.CompletedAt != nil {
		t.Fatalf("expected stored completed_at to be nil after reactivation, got %v", *stored.CompletedAt)
	}
}
