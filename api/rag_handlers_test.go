package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fitness-tracker/database"
	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func setupRAGHandlersTestServer(t *testing.T) (*gorm.DB, *Server) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return db, NewServer(db)
}

func issueBearerTokenForUser(t *testing.T, db *gorm.DB, role string) string {
	t.Helper()

	user := models.User{
		ID:           uuid.New(),
		Email:        role + "@example.com",
		Name:         role,
		PasswordHash: "x",
		Role:         role,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	session, err := authSvc.CreateSession(user.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	return session.AccessToken
}

func TestRAGQueryEndpointProxiesUpstreamResponse(t *testing.T) {
	t.Parallel()

	_, server := setupRAGHandlersTestServer(t)

	server.ragSvc = services.NewRAGClientWithHTTPClient("http://rag.internal", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/query" {
				t.Fatalf("expected /query path, got %s", r.URL.Path)
			}

			var req services.RAGQueryRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode upstream request: %v", err)
			}
			if req.Query != "What is Kamal?" {
				t.Fatalf("expected proxied query, got %q", req.Query)
			}

			body, err := json.Marshal(services.RAGQueryResponse{
				Answer:   "Kamal is a deployment tool.",
				ModeUsed: "hybrid",
				Sources: []services.RAGQuerySource{
					{FileName: "kamal.pdf", PageNumber: intPtr(7)},
				},
			})
			if err != nil {
				t.Fatalf("marshal upstream response: %v", err)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(string(body))),
			}, nil
		}),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/rag/query", strings.NewReader(`{"query":"What is Kamal?"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp services.RAGQueryResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Answer != "Kamal is a deployment tool." {
		t.Fatalf("unexpected answer %q", resp.Answer)
	}
	if resp.ModeUsed != "hybrid" {
		t.Fatalf("unexpected mode %q", resp.ModeUsed)
	}
	if len(resp.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(resp.Sources))
	}
}

func TestRAGCacheClearEndpointRequiresAdminAndProxiesResponse(t *testing.T) {
	t.Parallel()

	db, server := setupRAGHandlersTestServer(t)

	server.ragSvc = services.NewRAGClientWithHTTPClient("http://rag.internal", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/cache/clear" {
				t.Fatalf("expected /cache/clear path, got %s", r.URL.Path)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"cleared":3}`)),
			}, nil
		}),
	})

	unauthReq := httptest.NewRequest(http.MethodPost, "/v1/rag/cache/clear", nil)
	unauthRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthRR, unauthReq)
	if unauthRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", unauthRR.Code)
	}

	adminToken := issueBearerTokenForUser(t, db, "admin")

	req := httptest.NewRequest(http.MethodPost, "/v1/rag/cache/clear", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()

	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp services.RAGCacheClearResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Cleared != 3 {
		t.Fatalf("unexpected cleared count %d", resp.Cleared)
	}
}

func intPtr(v int) *int {
	return &v
}
