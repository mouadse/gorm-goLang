package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fitness-tracker/metrics"
)

// RAGClient proxies requests to the dedicated RAG backend service.
type RAGClient struct {
	baseURL    string
	httpClient *http.Client
	metrics    *metrics.Metrics
}

// RAGAPIError captures non-2xx responses returned by the RAG service.
type RAGAPIError struct {
	StatusCode int
	Body       string
}

func (e *RAGAPIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body == "" {
		return fmt.Sprintf("rag returned %d", e.StatusCode)
	}
	return fmt.Sprintf("rag returned %d: %s", e.StatusCode, e.Body)
}

// RAGQueryRequest is the public query payload forwarded to the RAG service.
type RAGQueryRequest struct {
	Query          string `json:"query"`
	Mode           string `json:"mode,omitempty"`
	ExactSearch    *bool  `json:"exact_search,omitempty"`
	SimilarityTopK *int   `json:"similarity_top_k,omitempty"`
	SparseTopK     *int   `json:"sparse_top_k,omitempty"`
	IncludeSources bool   `json:"include_sources,omitempty"`
}

// RAGQuerySource describes one retrieved passage returned by the RAG service.
type RAGQuerySource struct {
	Score      *float64 `json:"score,omitempty"`
	FileName   string   `json:"file_name,omitempty"`
	SourceStem string   `json:"source_stem,omitempty"`
	PageLabel  string   `json:"page_label,omitempty"`
	PageNumber *int     `json:"page_number,omitempty"`
	Excerpt    string   `json:"excerpt,omitempty"`
}

// RAGQueryResponse is the query result returned by the RAG service.
type RAGQueryResponse struct {
	Answer   string           `json:"answer"`
	ModeUsed string           `json:"mode_used"`
	Sources  []RAGQuerySource `json:"sources,omitempty"`
}

// RAGHealthResponse reports service liveness.
type RAGHealthResponse struct {
	Status string `json:"status"`
}

// RAGCacheStatsResponse reports query-cache metrics from the RAG service.
type RAGCacheStatsResponse struct {
	Size       int `json:"size"`
	MaxSize    int `json:"max_size"`
	TTLSeconds int `json:"ttl_seconds"`
	Hits       int `json:"hits"`
	Misses     int `json:"misses"`
}

// RAGCacheClearResponse reports how many cache entries were cleared.
type RAGCacheClearResponse struct {
	Cleared int `json:"cleared"`
}

// NewRAGClient builds a client using environment configuration.
func NewRAGClient(m ...*metrics.Metrics) *RAGClient {
	return NewRAGClientWithBaseURL(getEnvOrDefault("RAG_API_URL", "http://localhost:8088"), m...)
}

// NewRAGClientWithBaseURL builds a client for an explicit base URL.
func NewRAGClientWithBaseURL(baseURL string, m ...*metrics.Metrics) *RAGClient {
	return NewRAGClientWithHTTPClient(baseURL, &http.Client{Timeout: 90 * time.Second}, m...)
}

// NewRAGClientWithHTTPClient builds a client for an explicit base URL and HTTP client.
func NewRAGClientWithHTTPClient(baseURL string, httpClient *http.Client, m ...*metrics.Metrics) *RAGClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 90 * time.Second}
	}
	client := &RAGClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
	}
	if len(m) > 0 {
		client.metrics = m[0]
	}
	return client
}

// Query proxies a query request to the RAG backend.
func (c *RAGClient) Query(ctx context.Context, req RAGQueryRequest) (*RAGQueryResponse, error) {
	var resp RAGQueryResponse
	if err := c.do(ctx, http.MethodPost, "/query", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Health returns the backend liveness state.
func (c *RAGClient) Health(ctx context.Context) (*RAGHealthResponse, error) {
	var resp RAGHealthResponse
	if err := c.do(ctx, http.MethodGet, "/health", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CacheStats returns current RAG cache metrics.
func (c *RAGClient) CacheStats(ctx context.Context) (*RAGCacheStatsResponse, error) {
	var resp RAGCacheStatsResponse
	if err := c.do(ctx, http.MethodGet, "/cache/stats", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ClearCache clears the backend query cache.
func (c *RAGClient) ClearCache(ctx context.Context) (*RAGCacheClearResponse, error) {
	var resp RAGCacheClearResponse
	if err := c.do(ctx, http.MethodPost, "/cache/clear", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *RAGClient) do(ctx context.Context, method, path string, body any, target any) error {
	start := time.Now()
	var reqBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.metrics != nil {
			c.metrics.ExtServiceErrors.WithLabelValues("rag", "connection").Inc()
		}
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	status := fmt.Sprintf("%d", resp.StatusCode)
	if c.metrics != nil {
		c.metrics.ExtServiceRequests.WithLabelValues("rag", method, status).Inc()
		c.metrics.ExtServiceDuration.WithLabelValues("rag").Observe(time.Since(start).Seconds())
	}

	if resp.StatusCode >= 400 {
		if c.metrics != nil {
			c.metrics.ExtServiceErrors.WithLabelValues("rag", status).Inc()
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		return &RAGAPIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
	}

	return json.NewDecoder(resp.Body).Decode(target)
}
