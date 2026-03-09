package api

import (
	_ "embed"
	"net/http"
)

var (
	//go:embed openapi.yaml
	openAPISpec []byte

	//go:embed swagger_ui.html
	swaggerUIHTML []byte
)

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}

func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(swaggerUIHTML)
}
