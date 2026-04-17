package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"fitness-tracker/models"
	"fitness-tracker/services"
)

func TestAuthenticateRejectsQueryTokenWithSpoofedWebSocketHeaders(t *testing.T) {
	db, _ := newAdminTestServer(t)

	user := models.User{
		Email:        "ws-query-auth@example.com",
		PasswordHash: "hash",
		Name:         "WS Query Auth",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := services.GenerateJWTWithVersion(user.ID, user.AuthVersion, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	handler := Authenticate(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard/realtime?access_token="+url.QueryEscape(token), nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
}

func TestAuthenticateWebSocketQueryTokenAllowsHandshakeQueryToken(t *testing.T) {
	db, _ := newAdminTestServer(t)

	user := models.User{
		Email:        "query-only-auth@example.com",
		PasswordHash: "hash",
		Name:         "Query Only Auth",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := services.GenerateJWTWithVersion(user.ID, user.AuthVersion, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	handler := authenticateWebSocketQueryToken(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard/realtime?access_token="+url.QueryEscape(token), nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusNoContent, recorder.Code, recorder.Body.String())
	}
}

func TestAuthenticateWebSocketQueryTokenRejectsQueryTokenWithoutHandshake(t *testing.T) {
	db, _ := newAdminTestServer(t)

	user := models.User{
		Email:        "query-only-auth@example.com",
		PasswordHash: "hash",
		Name:         "Query Only Auth",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := services.GenerateJWTWithVersion(user.ID, user.AuthVersion, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	handler := authenticateWebSocketQueryToken(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dashboard/realtime?access_token="+url.QueryEscape(token), nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusUnauthorized, recorder.Code, recorder.Body.String())
	}
}
