package api

import (
	"errors"
	"net/http"
	"strings"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	errInvalidCredentials = errors.New("invalid email or password")
)

type authSessionResponse struct {
	Token             string      `json:"token"`
	AccessToken       string      `json:"access_token"`
	RefreshToken      string      `json:"refresh_token"`
	ExpiresIn         int64       `json:"expires_in"`
	User              models.User `json:"user"`
	TwoFactorRequired bool        `json:"two_factor_required,omitempty"`
	TwoFactorToken    string      `json:"two_factor_token,omitempty"`
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

type twoFactorCodeRequest struct {
	Code string `json:"code"`
}

type twoFactorSetupResponse struct {
	Secret   string `json:"secret"`
	OTPURL   string `json:"otp_url"`
	Verified bool   `json:"verified"`
}

type twoFactorVerifyResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
	Verified      bool     `json:"verified"`
}

func (s *Server) handleLoginWithSessions(w http.ResponseWriter, r *http.Request) {
	s.handleLoginRequest(w, r, false)
}

func (s *Server) handleRecoverWithTwoFactor(w http.ResponseWriter, r *http.Request) {
	s.handleLoginRequest(w, r, true)
}

func (s *Server) handleLoginRequest(w http.ResponseWriter, r *http.Request, requireRecovery bool) {
	var req services.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.TwoFactorToken) == "" {
		if strings.TrimSpace(req.Email) == "" {
			writeError(w, http.StatusBadRequest, errors.New("email is required"))
			return
		}
		if strings.TrimSpace(req.Password) == "" {
			writeError(w, http.StatusBadRequest, errors.New("password is required"))
			return
		}
	}

	user, challengeToken, err := s.resolveLoginUser(req)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDuplicateLoginUser):
			writeError(w, http.StatusConflict, err)
		case errors.Is(err, services.ErrLegacyPasswordHash):
			writeError(w, http.StatusConflict, err)
		case errors.Is(err, errInvalidTwoFactorToken):
			writeError(w, http.StatusUnauthorized, err)
		default:
			writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		}
		return
	}
	if challengeToken == "" {
		email, err := services.NormalizeEmail(req.Email)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.authSvc.BackfillNormalizedEmail(&user, email); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	twoFactorEnabled, err := s.twoFactorSvc.IsEnabled(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if challengeToken != "" && !twoFactorEnabled {
		s.twoFactorTokens.Delete(challengeToken)
		writeError(w, http.StatusUnauthorized, errInvalidTwoFactorToken)
		return
	}

	if twoFactorEnabled {
		recoveryCode := strings.TrimSpace(req.RecoveryCode)
		totpCode := strings.TrimSpace(req.TOTPCode)
		limitKey := s.twoFactorAttemptKey(r, user.ID, requireRecovery, recoveryCode != "")

		if requireRecovery && recoveryCode == "" {
			writeError(w, http.StatusBadRequest, errors.New("recovery_code is required"))
			return
		}

		switch {
		case recoveryCode != "":
			if !s.twoFactorLimit.Allow(limitKey) {
				writeError(w, http.StatusTooManyRequests, errTooManyTwoFactorAttempts)
				return
			}

			userAgent, ipAddress := services.ExtractSessionInfo(r)
			tokens, err := s.completeRecoveryCodeLogin(user.ID, recoveryCode, userAgent, ipAddress)
			if err != nil {
				if errors.Is(err, models.ErrInvalidRecoveryCode) {
					s.twoFactorLimit.RegisterFailure(limitKey)
					writeError(w, http.StatusUnauthorized, err)
					return
				}
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			s.twoFactorLimit.Reset(limitKey)
			if challengeToken != "" {
				s.twoFactorTokens.Delete(challengeToken)
			}
			writeJSON(w, http.StatusOK, authSessionResponse{
				Token:        tokens.AccessToken,
				AccessToken:  tokens.AccessToken,
				RefreshToken: tokens.RefreshToken,
				ExpiresIn:    tokens.ExpiresIn,
				User:         user,
			})
			return
		case totpCode != "":
			if requireRecovery {
				writeError(w, http.StatusBadRequest, errors.New("recovery_code is required"))
				return
			}
			if !s.twoFactorLimit.Allow(limitKey) {
				writeError(w, http.StatusTooManyRequests, errTooManyTwoFactorAttempts)
				return
			}
			if err := s.twoFactorSvc.VerifyLoginTOTP(user.ID, totpCode); err != nil {
				if errors.Is(err, models.ErrInvalidTOTP) || errors.Is(err, models.ErrInvalidTOTPFormat) {
					s.twoFactorLimit.RegisterFailure(limitKey)
				}
				if errors.Is(err, models.ErrInvalidTOTP) {
					writeError(w, http.StatusUnauthorized, err)
					return
				}
				if errors.Is(err, models.ErrInvalidTOTPFormat) {
					writeError(w, http.StatusBadRequest, err)
					return
				}
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			s.twoFactorLimit.Reset(limitKey)
		default:
			token := challengeToken
			if token == "" {
				token, err = s.twoFactorTokens.Issue(user.ID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err)
					return
				}
			}
			writeJSON(w, http.StatusAccepted, authSessionResponse{
				User:              user,
				TwoFactorRequired: true,
				TwoFactorToken:    token,
			})
			return
		}
	}

	userAgent, ipAddress := services.ExtractSessionInfo(r)
	tokens, err := s.authSvc.CreateSession(user.ID, userAgent, ipAddress)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if challengeToken != "" {
		s.twoFactorTokens.Delete(challengeToken)
	}

	writeJSON(w, http.StatusOK, authSessionResponse{
		Token:        tokens.AccessToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         user,
	})
}

func (s *Server) resolveLoginUser(req services.LoginRequest) (models.User, string, error) {
	challengeToken := strings.TrimSpace(req.TwoFactorToken)
	if challengeToken != "" {
		userID, err := s.twoFactorTokens.Resolve(challengeToken)
		if err != nil {
			return models.User{}, "", err
		}

		var user models.User
		if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
			return models.User{}, "", errInvalidTwoFactorToken
		}
		return user, challengeToken, nil
	}

	email, err := services.NormalizeEmail(req.Email)
	if err != nil {
		return models.User{}, "", err
	}

	password, err := services.RequirePassword(req.Password)
	if err != nil {
		return models.User{}, "", err
	}

	user, err := s.authSvc.LookupLoginUser(email)
	if err != nil {
		if errors.Is(err, services.ErrDuplicateLoginUser) {
			return models.User{}, "", err
		}
		return models.User{}, "", errInvalidCredentials
	}

	if err := services.ComparePassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, services.ErrLegacyPasswordHash) {
			return models.User{}, "", err
		}
		return models.User{}, "", errInvalidCredentials
	}

	return user, "", nil
}

func (s *Server) completeRecoveryCodeLogin(userID uuid.UUID, recoveryCode, userAgent, ipAddress string) (*services.AuthTokens, error) {
	var tokens *services.AuthTokens
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.twoFactorSvc.ConsumeRecoveryCodeTx(tx, userID, recoveryCode); err != nil {
			return err
		}

		issued, err := s.authSvc.CreateSessionTx(tx, userID, userAgent, ipAddress)
		if err != nil {
			return err
		}
		tokens = issued
		return nil
	})
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (s *Server) twoFactorAttemptKey(r *http.Request, userID uuid.UUID, requireRecovery bool, usingRecovery bool) string {
	switch {
	case requireRecovery || usingRecovery:
		return "recovery_login:" + userID.String()
	default:
		return "totp_login:" + userID.String()
	}
}

func (s *Server) handleSetupTwoFactor(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var user models.User
	if err := s.db.Select("email").First(&user, "id = ?", userID).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	secret, otpURL, err := s.twoFactorSvc.BeginSetup(userID, user.Email)
	if err != nil {
		if errors.Is(err, models.Err2FAAlreadyEnabled) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, twoFactorSetupResponse{
		Secret:   secret.Secret,
		OTPURL:   otpURL,
		Verified: secret.Verified,
	})
}

func (s *Server) handleVerifyTwoFactor(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req twoFactorCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, errors.New("code is required"))
		return
	}
	limitKey := "verify_setup:" + userID.String()
	if !s.twoFactorLimit.Allow(limitKey) {
		writeError(w, http.StatusTooManyRequests, errTooManyTwoFactorAttempts)
		return
	}

	recoveryCodes, err := s.twoFactorSvc.VerifySetup(userID, req.Code)
	if err != nil {
		if errors.Is(err, models.ErrInvalidTOTP) || errors.Is(err, models.ErrInvalidTOTPFormat) {
			s.twoFactorLimit.RegisterFailure(limitKey)
		}
		if errors.Is(err, models.ErrInvalidTOTP) {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		if errors.Is(err, models.ErrInvalidTOTPFormat) {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, models.Err2FANotEnabled) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.twoFactorLimit.Reset(limitKey)

	writeJSON(w, http.StatusOK, twoFactorVerifyResponse{
		RecoveryCodes: recoveryCodes,
		Verified:      true,
	})
}

func (s *Server) handleDisableTwoFactor(w http.ResponseWriter, r *http.Request) {
	userID, err := authenticatedUserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	var req twoFactorCodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, errors.New("code is required"))
		return
	}
	limitKey := "disable_2fa:" + userID.String()
	if !s.twoFactorLimit.Allow(limitKey) {
		writeError(w, http.StatusTooManyRequests, errTooManyTwoFactorAttempts)
		return
	}

	if err := s.twoFactorSvc.Disable(userID, req.Code); err != nil {
		if errors.Is(err, models.ErrInvalidTOTP) || errors.Is(err, models.ErrInvalidTOTPFormat) {
			s.twoFactorLimit.RegisterFailure(limitKey)
		}
		if errors.Is(err, models.ErrInvalidTOTP) {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		if errors.Is(err, models.ErrInvalidTOTPFormat) {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, models.Err2FANotEnabled) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.twoFactorLimit.Reset(limitKey)

	writeJSON(w, http.StatusNoContent, nil)
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
