package metrics

import (
	"log"
	"net/http"
)

// StartMetricsServer starts an HTTP server serving the Prometheus metrics endpoint.
// This is used by the worker process which doesn't share the API server.
func StartMetricsServer(addr string, m *Metrics) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	go func() {
		log.Printf("metrics server listening on %s", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()
}