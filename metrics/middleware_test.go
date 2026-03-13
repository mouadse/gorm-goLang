package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestMiddlewareUsesMatchedRoutePattern(t *testing.T) {
	t.Parallel()

	m := New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := m.Middleware(mux)
	request := httptest.NewRequest(http.MethodGet, "/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}

	assertCounterValue(t, m.RequestsTotal, []string{http.MethodGet, "/v1/users/{id}", "204"}, 1)
}

func TestMiddlewareUsesUnmatchedLabelForUnknownRoutes(t *testing.T) {
	t.Parallel()

	m := New()
	handler := m.Middleware(http.NewServeMux())
	request := httptest.NewRequest(http.MethodGet, "/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}

	assertCounterValue(t, m.RequestsTotal, []string{http.MethodGet, "unmatched", "404"}, 1)
}

func assertCounterValue(t *testing.T, counter *prometheus.CounterVec, labels []string, want float64) {
	t.Helper()

	metric := &dto.Metric{}
	if err := counter.WithLabelValues(labels...).Write(metric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if got := metric.GetCounter().GetValue(); got != want {
		t.Fatalf("expected counter %v, got %v", want, got)
	}
}
