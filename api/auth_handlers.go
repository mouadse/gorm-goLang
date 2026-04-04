package api

import (
	"errors"
	"net/http"

	"fitness-tracker/models"
	"fitness-tracker/services"
)

type authSessionResponse struct {
	Token        string      `json:"token"`
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	User         models.User `json:"user"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type sessionResponse struct {
	ID        string `json:"id"`
	UserAgent string `json:"user_agent"`
	LastIP    string `json:"last_ip"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
	AllSessions  bool   `json:"all_sessions,omitempty"`
}

func (s *Server) handleLoginWithSessions(w http.ResponseWriter, r *http.Request) {
	var req services.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	email, err := services.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	password, err := services.RequirePassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.authSvc.LookupLoginUser(email)
	if err != nil {
		if errors.Is(err, services.ErrDuplicateLoginUser) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		return
	}

	if err := services.ComparePassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, services.ErrLegacyPasswordHash) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		return
	}

	if err := s.authSvc.BackfillNormalizedEmail(&user, email); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	userAgent, ipAddress := services.ExtractSessionInfo(r)
	tokens, err := s.authSvc.CreateSession(user.ID, userAgent, ipAddress)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, authSessionResponse{
		Token:        tokens.AccessToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         user,
	})
}

func (s *Server) handleRegisterWithSessions(w http.ResponseWriter, r *http.Request) {
	var req services.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.authSvc.CreateLocalUser(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	userAgent, ipAddress := services.ExtractSessionInfo(r)
	tokens, err := s.authSvc.CreateSession(user.ID, userAgent, ipAddress)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, authSessionResponse{
		Token:        tokens.AccessToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         user,
	})
}

func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("refresh_token is required"))
		return
	}

	userAgent, ipAddress := services.ExtractSessionInfo(r)
	tokens, err := s.authSvc.RefreshSession(req.RefreshToken, userAgent, ipAddress)
	if err != nil {
		if errors.Is(err, services.ErrTokenRevoked) {
			writeError(w, http.StatusUnauthorized, errors.New("token has been revoked"))
			return
		}
		if errors.Is(err, services.ErrInvalidToken) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid or expired refresh token"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, refreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if !req.AllSessions && req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("must provide either refresh_token or all_sessions"))
		return
	}

	if req.AllSessions {
		if err := s.authSvc.DeleteAllUserSessions(userID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	} else if req.RefreshToken != "" {
		if err := s.authSvc.RevokeUserSession(userID, req.RefreshToken); err != nil {
			if errors.Is(err, services.ErrSessionNotFound) {
				writeError(w, http.StatusNotFound, err)
				return
			}
			if errors.Is(err, services.ErrUnauthorizedAccess) {
				writeError(w, http.StatusForbidden, errors.New("cannot revoke another user's session"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	sessions, err := s.authSvc.GetUserSessions(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	response := make([]sessionResponse, len(sessions))
	for i, s := range sessions {
		response[i] = sessionResponse{
			ID:        s.ID.String(),
			UserAgent: s.UserAgent,
			LastIP:    s.LastIP,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ExpiresAt: s.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	sessionID, err := parsePathUUID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.authSvc.DeleteSession(userID, sessionID); err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}
