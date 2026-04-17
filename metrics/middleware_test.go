package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
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

func TestMiddlewarePreservesWebSocketUpgradeInterfaces(t *testing.T) {
	t.Parallel()

	m := New()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		defer conn.Close()

		if err := conn.WriteMessage(websocket.TextMessage, []byte("ok")); err != nil {
			t.Errorf("write websocket message: %v", err)
		}
	})))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	messageType, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	if messageType != websocket.TextMessage {
		t.Fatalf("expected text message, got %d", messageType)
	}
	if string(payload) != "ok" {
		t.Fatalf("expected websocket payload %q, got %q", "ok", string(payload))
	}
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
