package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fitness-tracker/services"
)

func TestWriteRAGProxyErrorPreservesUpstreamStatus(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writeRAGProxyError(recorder, "rag query", &services.RAGAPIError{
		StatusCode: http.StatusBadRequest,
		Body:       `{"detail":"query must not be empty"}`,
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	message, _ := body["error"].(string)
	if !strings.Contains(message, "query must not be empty") {
		t.Fatalf("expected upstream validation detail, got %q", message)
	}
}
