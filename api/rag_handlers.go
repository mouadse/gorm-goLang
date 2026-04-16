package api

import (
	"errors"
	"fmt"
	"net/http"

	"fitness-tracker/services"
)

func writeRAGProxyError(w http.ResponseWriter, context string, err error) {
	var upstreamErr *services.RAGAPIError
	if errors.As(err, &upstreamErr) {
		writeError(w, upstreamErr.StatusCode, fmt.Errorf("%s: %w", context, upstreamErr))
		return
	}

	writeError(w, http.StatusBadGateway, fmt.Errorf("%s: %w", context, err))
}

func (s *Server) handleRAGQuery(w http.ResponseWriter, r *http.Request) {
	var req services.RAGQueryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := s.ragSvc.Query(r.Context(), req)
	if err != nil {
		writeRAGProxyError(w, "rag query", err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRAGHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ragSvc.Health(r.Context())
	if err != nil {
		writeRAGProxyError(w, "rag health", err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRAGCacheStats(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ragSvc.CacheStats(r.Context())
	if err != nil {
		writeRAGProxyError(w, "rag cache stats", err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRAGCacheClear(w http.ResponseWriter, r *http.Request) {
	resp, err := s.ragSvc.ClearCache(r.Context())
	if err != nil {
		writeRAGProxyError(w, "rag cache clear", err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
