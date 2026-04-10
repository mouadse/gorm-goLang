package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"
)

func TestExportJobFlow(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "export@example.com", "ExportUser", "password123")

	t.Run("create export job", func(t *testing.T) {
		job := requestJSONAuth[services.ExportJob](t, server, userAuth.AccessToken, http.MethodPost, "/v1/exports", map[string]string{
			"format": "json",
		}, http.StatusAccepted)

		if job.Status != services.ExportPending && job.Status != services.ExportProcessing && job.Status != services.ExportCompleted {
			t.Fatalf("unexpected export status: %s", job.Status)
		}

		// Wait briefly for async processing
		time.Sleep(100 * time.Millisecond)

		// Get job status
		fetched := requestJSONAuth[services.ExportJob](t, server, userAuth.AccessToken, http.MethodGet, "/v1/exports/"+job.ID.String(), nil, http.StatusOK)

		if fetched.Status != services.ExportCompleted {
			// Might be still processing, let's wait a bit more and retry
			time.Sleep(500 * time.Millisecond)
			fetched = requestJSONAuth[services.ExportJob](t, server, userAuth.AccessToken, http.MethodGet, "/v1/exports/"+job.ID.String(), nil, http.StatusOK)
		}

		if fetched.Status != services.ExportCompleted {
			t.Fatalf("expected export to be completed, got %s", fetched.Status)
		}

		notifications := requestJSONAuth[[]map[string]any](t, server, userAuth.AccessToken, http.MethodGet, "/v1/notifications", nil, http.StatusOK)
		foundExportReady := false
		for _, notification := range notifications {
			if notification["type"] == string(models.NotificationExportReady) {
				foundExportReady = true
				break
			}
		}
		if !foundExportReady {
			t.Fatalf("expected export_ready notification after completed export, got %#v", notifications)
		}

		// Download data
		req, _ := http.NewRequest(http.MethodGet, "/v1/exports/"+job.ID.String()+"?download=true", nil)
		req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d for download, got %d", http.StatusOK, w.Code)
		}

		var data services.UserDataExport
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatalf("failed to decode downloaded json: %v", err)
		}

		if data.UserID != userAuth.User.ID.String() {
			t.Fatalf("expected export user ID %s, got %s", userAuth.User.ID, data.UserID)
		}
	})
}

func TestAccountDeletionFlow(t *testing.T) {
	t.Parallel()
	server := newTestServer(t)
	userAuth := registerTestUser(t, server, "delete_me@example.com", "DeleteUser", "password123")

	t.Run("create deletion request", func(t *testing.T) {
		req := requestJSONAuth[services.DeletionRequest](t, server, userAuth.AccessToken, http.MethodPost, "/v1/account/delete-request", nil, http.StatusAccepted)

		if req.Status != "processed" {
			t.Fatalf("expected status processed, got %s", req.Status)
		}

		// Validate user is deleted by failing to get profile
		reqUser, _ := http.NewRequest(http.MethodGet, "/v1/users/"+userAuth.User.ID.String(), nil)
		reqUser.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, reqUser)

		if w.Code != http.StatusNotFound && w.Code != http.StatusUnauthorized {
			t.Fatalf("expected user to be not found or unauthorized, got status %d", w.Code)
		}
	})
}
