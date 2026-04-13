package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestExerciseLibClientDoReturnsTypedAPIError(t *testing.T) {
	t.Parallel()

	client := &ExerciseLibClient{
		baseURL: "http://exercise-lib.invalid",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"detail":"query must not be empty"}`)),
				}, nil
			}),
		},
	}

	err := client.do("POST", "/search", LibSearchRequest{Query: "", TopK: 8}, &LibSearchResponse{})
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *ExerciseLibAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected ExerciseLibAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "query must not be empty") {
		t.Fatalf("expected upstream validation detail, got %q", apiErr.Body)
	}
}

func TestExerciseLibClientWaitUntilReadyFailsFastOnCatalogError(t *testing.T) {
	t.Parallel()

	requests := 0
	client := &ExerciseLibClient{
		baseURL: "http://exercise-lib.invalid",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"detail": {
							"status": "error",
							"catalog_status": "error",
							"ready": false,
							"indexed": false,
							"exercises_loaded": 0,
							"embedding_model": "BAAI/bge-small-en-v1.5",
							"error": "exercise dataset is missing"
						}
					}`)),
				}, nil
			}),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.WaitUntilReady(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected terminal readiness error, got timeout: %v", err)
	}
	if !strings.Contains(err.Error(), "catalog_status=error") {
		t.Fatalf("expected catalog error status in message, got %v", err)
	}
	if !strings.Contains(err.Error(), "exercise dataset is missing") {
		t.Fatalf("expected readiness error detail in message, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected fail-fast after one readiness check, got %d requests", requests)
	}
}
