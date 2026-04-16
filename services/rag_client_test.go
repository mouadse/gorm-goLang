package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRAGClientDoReturnsTypedAPIError(t *testing.T) {
	t.Parallel()

	client := &RAGClient{
		baseURL: "http://rag.invalid",
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

	err := client.do(context.Background(), http.MethodPost, "/query", RAGQueryRequest{Query: ""}, &RAGQueryResponse{})
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *RAGAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected RAGAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "query must not be empty") {
		t.Fatalf("expected upstream validation detail, got %q", apiErr.Body)
	}
}
