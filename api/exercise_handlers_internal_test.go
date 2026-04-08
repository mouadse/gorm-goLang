package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fitness-tracker/services"
)

func TestWriteExerciseLibProxyErrorPreservesUpstreamStatus(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writeExerciseLibProxyError(recorder, "exercise library search", &services.ExerciseLibAPIError{
		StatusCode: http.StatusBadRequest,
		Body:       `{"detail":"query must not be empty"}`,
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if !strings.Contains(body["error"], "query must not be empty") {
		t.Fatalf("expected upstream validation detail, got %q", body["error"])
	}
}
