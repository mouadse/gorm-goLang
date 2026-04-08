package services

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
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
