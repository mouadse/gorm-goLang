package api_test

import (
	"fitness-tracker/api"
	"net/http"
	"testing"
	"time"

	"fitness-tracker/models"
	"fitness-tracker/services"

	"github.com/pquerna/otp/totp"
)

func TestRefreshTokenSuccess(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "refresh@example.com",
		PasswordHash: passwordHash,
		Name:         "Refresh User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	resp := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusOK)

	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Error("expected access_token in refresh response")
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Error("expected refresh_token in refresh response")
	}
	if resp["expires_in"] == nil {
		t.Error("expected expires_in in refresh response")
	}
}

func TestRefreshWithRevokedToken(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "revoked@example.com",
		PasswordHash: passwordHash,
		Name:         "Revoked User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := authSvc.RevokeSession(tokens.RefreshToken); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusUnauthorized)
	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error, got %q", errResp["error"])
	}
}

func TestRefreshWithInvalidToken(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	refreshReq := map[string]any{
		"refresh_token": "invalid-token",
	}

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusUnauthorized)
	if errResp["error"] != "invalid or expired refresh token" {
		t.Errorf("expected invalid token error, got %q", errResp["error"])
	}
}

func TestRefreshAfterExpiry(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "expired@example.com",
		PasswordHash: passwordHash,
		Name:         "Expired User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := services.GenerateSecureToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	tokenHash, err := services.HashToken(token)
	if err != nil {
		t.Fatalf("hash token: %v", err)
	}

	expiredTime := time.Now().UTC().Add(-24 * time.Hour)
	refreshToken := services.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
		ExpiresAt: expiredTime,
		CreatedAt: expiredTime.Add(-7 * 24 * time.Hour),
	}
	if err := db.Create(&refreshToken).Error; err != nil {
		t.Fatalf("create expired refresh token: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": token,
	}

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusUnauthorized)
	if errResp["error"] != "invalid or expired refresh token" {
		t.Errorf("expected expired token error, got %q", errResp["error"])
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "logout@example.com",
		PasswordHash: passwordHash,
		Name:         "Logout User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	logoutReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	expectStatusAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusNoContent)

	refreshReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	errResp := requestErrorAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusUnauthorized)
	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error after logout, got %q", errResp["error"])
	}
}

func TestLogoutAllSessions(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "alllogout@example.com",
		PasswordHash: passwordHash,
		Name:         "All Logout User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)

	tokens1, err := authSvc.CreateSession(user.ID, "device1", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}

	tokens2, err := authSvc.CreateSession(user.ID, "device2", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessionsBefore := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsBefore) < 2 {
		t.Errorf("expected at least 2 sessions before logout, got %d", len(sessionsBefore))
	}

	logoutReq := map[string]any{
		"all_sessions": true,
	}

	expectStatusAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusNoContent)
	expectStatusAuth(t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusUnauthorized)

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": tokens1.RefreshToken,
	}, http.StatusUnauthorized)
	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error for first token, got %q", errResp["error"])
	}

	errResp = requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": tokens2.RefreshToken,
	}, http.StatusUnauthorized)
	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error for second token, got %q", errResp["error"])
	}
}

func TestMultipleSessionsPerUser(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "multisession@example.com",
		PasswordHash: passwordHash,
		Name:         "Multi Session User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)

	_, err = authSvc.CreateSession(user.ID, "device1", "192.168.1.1")
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}

	_, err = authSvc.CreateSession(user.ID, "device2", "192.168.1.2")
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	_, err = authSvc.CreateSession(user.ID, "device3", "192.168.1.3")
	if err != nil {
		t.Fatalf("create session 3: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessions := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	userAgents := make(map[string]bool)
	for _, s := range sessions {
		userAgents[s.UserAgent] = true
	}
	if !userAgents["device1"] || !userAgents["device2"] || !userAgents["device3"] {
		t.Error("expected to find all three device user agents in sessions")
	}
}

func TestDeleteSingleSession(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "delsession@example.com",
		PasswordHash: passwordHash,
		Name:         "Delete Session User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)

	_, err = authSvc.CreateSession(user.ID, "device1", "192.168.1.1")
	if err != nil {
		t.Fatalf("create session 1: %v", err)
	}

	_, err = authSvc.CreateSession(user.ID, "device2", "192.168.1.2")
	if err != nil {
		t.Fatalf("create session 2: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessions := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	sessionToDelete := sessions[1]

	expectStatusAuth(t, handler, accessToken, http.MethodDelete, "/v1/auth/sessions/"+sessionToDelete.ID, nil, http.StatusNoContent)

	sessionsAfter := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsAfter) != 1 {
		t.Errorf("expected 1 session after delete, got %d", len(sessionsAfter))
	}

	for _, s := range sessionsAfter {
		if s.ID == sessionToDelete.ID {
			t.Error("deleted session should not be present")
		}
	}
}

func TestSessionsRequireAuth(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	expectStatus(t, handler, http.MethodGet, "/v1/auth/sessions", nil, http.StatusUnauthorized)
	expectStatus(t, handler, http.MethodDelete, "/v1/auth/sessions/some-id", nil, http.StatusUnauthorized)
	expectStatus(t, handler, http.MethodPost, "/v1/auth/logout", nil, http.StatusUnauthorized)
}

func TestLoginWithSessions(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	loginReq := map[string]any{
		"email":    "login-session@example.com",
		"password": "password123",
	}

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/login", loginReq, http.StatusUnauthorized)
	if errResp["error"] != "invalid email or password" {
		t.Errorf("expected invalid credentials error, got %q", errResp["error"])
	}
}

func TestRegisterWithSessions(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	registerReq := map[string]any{
		"email":    "new-session@example.com",
		"password": "password123",
		"name":     "New User",
	}

	resp := requestJSON[authSessionResponse](t, handler, http.MethodPost, "/v1/auth/register", registerReq, http.StatusCreated)

	if resp.AccessToken == "" {
		t.Error("expected access_token in register response")
	}
	if resp.Token == "" {
		t.Error("expected legacy token field in register response")
	}
	if resp.Token != resp.AccessToken {
		t.Error("expected legacy token field to mirror access_token")
	}
	if resp.RefreshToken == "" {
		t.Error("expected refresh_token in register response")
	}
	if resp.User.Email != "new-session@example.com" {
		t.Errorf("expected user email, got %q", resp.User.Email)
	}
}

func TestRegisterWithSessionsReturnsFieldValidationErrors(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":         "",
		"password":      "short",
		"name":          "",
		"date_of_birth": "2026/04/10",
		"age":           -1,
		"weight":        -2,
		"height":        -3,
		"tdee":          -4,
	}, http.StatusBadRequest)

	if errResp["title"] != "Validation failed" {
		t.Fatalf("expected validation title, got %#v", errResp["title"])
	}
	if errResp["error"] != "one or more fields are invalid" {
		t.Fatalf("expected validation summary, got %#v", errResp["error"])
	}

	fields := errorFieldMap(t, errResp)
	if fields["email"] != "email is required" {
		t.Fatalf("expected email field error, got %q", fields["email"])
	}
	if fields["password"] != "password must be at least 8 characters" {
		t.Fatalf("expected password field error, got %q", fields["password"])
	}
	if fields["name"] != "name is required" {
		t.Fatalf("expected name field error, got %q", fields["name"])
	}
	if fields["date_of_birth"] != "date_of_birth must be 2006-01-02" {
		t.Fatalf("expected date_of_birth field error, got %q", fields["date_of_birth"])
	}
	if fields["age"] != "age cannot be negative" {
		t.Fatalf("expected age field error, got %q", fields["age"])
	}
	if fields["weight"] != "weight cannot be negative" {
		t.Fatalf("expected weight field error, got %q", fields["weight"])
	}
	if fields["height"] != "height cannot be negative" {
		t.Fatalf("expected height field error, got %q", fields["height"])
	}
	if fields["tdee"] != "tdee cannot be negative" {
		t.Fatalf("expected tdee field error, got %q", fields["tdee"])
	}
}

func TestLoginReturnsFieldValidationErrors(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "",
		"password": "short",
	}, http.StatusBadRequest)

	if errResp["title"] != "Validation failed" {
		t.Fatalf("expected validation title, got %#v", errResp["title"])
	}
	if errResp["error"] != "one or more fields are invalid" {
		t.Fatalf("expected validation summary, got %#v", errResp["error"])
	}

	fields := errorFieldMap(t, errResp)
	if fields["email"] != "email is required" {
		t.Fatalf("expected email field error, got %q", fields["email"])
	}
	if fields["password"] != "password must be at least 8 characters" {
		t.Fatalf("expected password field error, got %q", fields["password"])
	}
}

func TestTokenRotationOnRefresh(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "rotate@example.com",
		PasswordHash: passwordHash,
		Name:         "Rotate User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	initialTokens, err := authSvc.CreateSession(user.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": initialTokens.RefreshToken,
	}

	refreshedTokens := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusOK)

	if refreshedTokens["refresh_token"] == initialTokens.RefreshToken {
		t.Error("refresh token should be rotated, not the same")
	}

	if refreshedTokens["access_token"] == "" {
		t.Error("expected new access_token")
	}

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusUnauthorized)
	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error after rotation, got %q", errResp["error"])
	}
}

func TestTwoFactorSetupLoginDisableFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "twofactor@example.com", "Two Factor", "password123")

	setup := requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)
	if setup.Secret == "" {
		t.Fatal("expected secret from setup response")
	}
	if setup.OTPURL == "" {
		t.Fatal("expected otp_url from setup response")
	}

	verifyCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate verify code: %v", err)
	}

	verified := requestJSONAuth[twoFactorVerifyResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": verifyCode,
	}, http.StatusOK)
	if !verified.Verified {
		t.Fatal("expected setup verification to succeed")
	}
	if len(verified.RecoveryCodes) != services.TwoFactorRecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", services.TwoFactorRecoveryCodeCount, len(verified.RecoveryCodes))
	}

	challenge := requestJSON[twoFactorChallengeResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "twofactor@example.com",
		"password": "password123",
	}, http.StatusAccepted)
	if !challenge.TwoFactorRequired {
		t.Fatal("expected two_factor_required=true")
	}
	if challenge.TwoFactorToken == "" {
		t.Fatal("expected two_factor_token in challenge response")
	}

	loginCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate login code: %v", err)
	}

	login := requestJSON[authSessionResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"two_factor_token": challenge.TwoFactorToken,
		"totp_code":        loginCode,
	}, http.StatusOK)
	if login.AccessToken == "" {
		t.Fatal("expected access token after valid TOTP login")
	}

	disableCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate disable code: %v", err)
	}

	expectStatusAuth(t, handler, login.AccessToken, http.MethodPost, "/v1/auth/2fa/disable", map[string]any{
		"code": disableCode,
	}, http.StatusNoContent)

	plainLogin := requestJSON[authSessionResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "twofactor@example.com",
		"password": "password123",
	}, http.StatusOK)
	if plainLogin.AccessToken == "" {
		t.Fatal("expected plain login after disabling 2FA")
	}
}

func TestTwoFactorInvalidTOTPRejected(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "invalid-2fa@example.com", "Invalid 2FA", "password123")

	setup := requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)
	verifyCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate verify code: %v", err)
	}
	requestJSONAuth[twoFactorVerifyResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": verifyCode,
	}, http.StatusOK)

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":     "invalid-2fa@example.com",
		"password":  "password123",
		"totp_code": "000000",
	}, http.StatusUnauthorized)
	if errResp["error"] != models.ErrInvalidTOTP.Error() {
		t.Fatalf("expected invalid TOTP error, got %q", errResp["error"])
	}
}

func TestTwoFactorInvalidTOTPFormatRejected(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "invalid-format-2fa@example.com", "Invalid Format 2FA", "password123")

	setup := requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)
	verifyCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate verify code: %v", err)
	}
	requestJSONAuth[twoFactorVerifyResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": verifyCode,
	}, http.StatusOK)

	challenge := requestJSON[twoFactorChallengeResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "invalid-format-2fa@example.com",
		"password": "password123",
	}, http.StatusAccepted)

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"two_factor_token": challenge.TwoFactorToken,
		"totp_code":        "12345",
	}, http.StatusBadRequest)
	if errResp["error"] != models.ErrInvalidTOTPFormat.Error() {
		t.Fatalf("expected invalid TOTP format error, got %q", errResp["error"])
	}
}

func TestTwoFactorRecoveryCodeConsumption(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "recover-2fa@example.com", "Recover 2FA", "password123")

	setup := requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)
	verifyCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate verify code: %v", err)
	}
	verified := requestJSONAuth[twoFactorVerifyResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": verifyCode,
	}, http.StatusOK)

	challenge := requestJSON[twoFactorChallengeResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "recover-2fa@example.com",
		"password": "password123",
	}, http.StatusAccepted)

	recoveryLogin := requestJSON[authSessionResponse](t, handler, http.MethodPost, "/v1/auth/2fa/recover", map[string]any{
		"two_factor_token": challenge.TwoFactorToken,
		"recovery_code":    verified.RecoveryCodes[0],
	}, http.StatusOK)
	if recoveryLogin.AccessToken == "" {
		t.Fatal("expected access token from recovery-code login")
	}

	reusedChallenge := requestJSON[twoFactorChallengeResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "recover-2fa@example.com",
		"password": "password123",
	}, http.StatusAccepted)

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/2fa/recover", map[string]any{
		"two_factor_token": reusedChallenge.TwoFactorToken,
		"recovery_code":    verified.RecoveryCodes[0],
	}, http.StatusUnauthorized)
	if errResp["error"] != models.ErrInvalidRecoveryCode.Error() {
		t.Fatalf("expected invalid recovery code error, got %q", errResp["error"])
	}
}

func TestTwoFactorVerifyRateLimitedAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "verify-limit@example.com", "Verify Limit", "password123")

	requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)

	for i := 0; i < 5; i++ {
		errResp := requestErrorAuth(t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
			"code": "000000",
		}, http.StatusUnauthorized)
		if errResp["error"] != models.ErrInvalidTOTP.Error() {
			t.Fatalf("attempt %d: expected invalid TOTP error, got %q", i+1, errResp["error"])
		}
	}

	errResp := requestErrorAuth(t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": "000000",
	}, http.StatusTooManyRequests)
	if errResp["error"] != "too many 2FA attempts, try again later" {
		t.Fatalf("expected rate limit error, got %q", errResp["error"])
	}
}

func TestTwoFactorRecoveryRateLimitedAfterRepeatedFailures(t *testing.T) {
	t.Parallel()

	_, handler := newTestApp(t)
	auth := registerTestUser(t, handler, "recover-limit@example.com", "Recover Limit", "password123")

	setup := requestJSONAuth[twoFactorSetupResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/setup", nil, http.StatusCreated)
	verifyCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate verify code: %v", err)
	}
	requestJSONAuth[twoFactorVerifyResponse](t, handler, auth.AccessToken, http.MethodPost, "/v1/auth/2fa/verify", map[string]any{
		"code": verifyCode,
	}, http.StatusOK)

	challenge := requestJSON[twoFactorChallengeResponse](t, handler, http.MethodPost, "/v1/auth/login", map[string]any{
		"email":    "recover-limit@example.com",
		"password": "password123",
	}, http.StatusAccepted)

	for i := 0; i < 5; i++ {
		errResp := requestError(t, handler, http.MethodPost, "/v1/auth/2fa/recover", map[string]any{
			"two_factor_token": challenge.TwoFactorToken,
			"recovery_code":    "WRONG-CODE",
		}, http.StatusUnauthorized)
		if errResp["error"] != models.ErrInvalidRecoveryCode.Error() {
			t.Fatalf("attempt %d: expected invalid recovery code error, got %q", i+1, errResp["error"])
		}
	}

	errResp := requestError(t, handler, http.MethodPost, "/v1/auth/2fa/recover", map[string]any{
		"two_factor_token": challenge.TwoFactorToken,
		"recovery_code":    "WRONG-CODE",
	}, http.StatusTooManyRequests)
	if errResp["error"] != "too many 2FA attempts, try again later" {
		t.Fatalf("expected rate limit error, got %q", errResp["error"])
	}
}

type authSessionResponse struct {
	Token          string      `json:"token"`
	AccessToken    string      `json:"access_token"`
	RefreshToken   string      `json:"refresh_token"`
	ExpiresIn      int64       `json:"expires_in"`
	User           models.User `json:"user"`
	TwoFactorToken string      `json:"two_factor_token"`
}

type sessionResponse struct {
	ID        string `json:"id"`
	UserAgent string `json:"user_agent"`
	LastIP    string `json:"last_ip"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
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

type twoFactorChallengeResponse struct {
	TwoFactorRequired bool        `json:"two_factor_required"`
	TwoFactorToken    string      `json:"two_factor_token"`
	User              models.User `json:"user"`
}

func TestDeleteSessionRevokesRefreshToken(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "del-session-revoke@example.com",
		PasswordHash: passwordHash,
		Name:         "Delete Session Revoke User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-device", "192.168.1.100")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessions := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	sessionToDelete := sessions[0]

	expectStatusAuth(t, handler, accessToken, http.MethodDelete, "/v1/auth/sessions/"+sessionToDelete.ID, nil, http.StatusNoContent)

	errResp := requestErrorAuth(t, handler, "", http.MethodPost, "/v1/auth/refresh", map[string]any{
		"refresh_token": tokens.RefreshToken,
	}, http.StatusUnauthorized)

	if errResp["error"] != "token has been revoked" {
		t.Errorf("expected revoked token error after session delete, got %q", errResp["error"])
	}
}

func TestRefreshDoesNotCreateNewSession(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "no-dup-session@example.com",
		PasswordHash: passwordHash,
		Name:         "No Duplicate Session User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	_, err = authSvc.CreateSession(user.ID, "test-device", "192.168.1.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessionsBefore := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsBefore) != 1 {
		t.Fatalf("expected 1 session before refresh, got %d", len(sessionsBefore))
	}

	tokens, err := authSvc.CreateSession(user.ID, "test-device", "192.168.1.1")
	if err != nil {
		t.Fatalf("create session for refresh: %v", err)
	}

	for i := 0; i < 3; i++ {
		refreshResp := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", map[string]any{
			"refresh_token": tokens.RefreshToken,
		}, http.StatusOK)

		newRefreshToken, ok := refreshResp["refresh_token"].(string)
		if !ok {
			t.Fatal("expected refresh_token in response")
		}
		tokens.RefreshToken = newRefreshToken
	}

	sessionsAfter := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data

	// Should still have 2 sessions: the original one + the one created for testing
	// The refreshes should not create new sessions
	if len(sessionsAfter) != len(sessionsBefore)+1 {
		t.Errorf("after %d refreshes, expected %d sessions, got %d - refresh should not create new sessions", 3, len(sessionsBefore)+1, len(sessionsAfter))
	}
}

func TestAccessTokenTTL(t *testing.T) {
	t.Parallel()

	if services.AccessTokenTTL != 15*time.Minute {
		t.Errorf("Expected AccessTokenTTL to be 15 minutes for security, got %v", services.AccessTokenTTL)
	}
}

func TestLogoutCannotRevokeOtherUsersToken(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user1 := models.User{
		Email:        "user1@example.com",
		PasswordHash: passwordHash,
		Name:         "User One",
	}
	if err := db.Create(&user1).Error; err != nil {
		t.Fatalf("create user1: %v", err)
	}

	user2 := models.User{
		Email:        "user2@example.com",
		PasswordHash: passwordHash,
		Name:         "User Two",
	}
	if err := db.Create(&user2).Error; err != nil {
		t.Fatalf("create user2: %v", err)
	}

	authSvc := services.NewAuthService(db)

	user1Tokens, err := authSvc.CreateSession(user1.ID, "user1-device", "192.168.1.1")
	if err != nil {
		t.Fatalf("create user1 session: %v", err)
	}

	user2Tokens, err := authSvc.CreateSession(user2.ID, "user2-device", "192.168.1.2")
	if err != nil {
		t.Fatalf("create user2 session: %v", err)
	}
	_ = user2Tokens // Not used, just creating session for context

	user2AccessToken, err := services.GenerateJWT(user2.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate user2 jwt: %v", err)
	}

	logoutReq := map[string]any{
		"refresh_token": user1Tokens.RefreshToken,
	}

	errResp := requestErrorAuth(t, handler, user2AccessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusForbidden)
	if errResp["error"] != "cannot revoke another user's session" {
		t.Errorf("expected forbidden error when trying to revoke another user's token, got %q", errResp["error"])
	}

	refreshReq := map[string]any{
		"refresh_token": user1Tokens.RefreshToken,
	}
	refreshResp := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusOK)

	if refreshResp["access_token"] == nil || refreshResp["access_token"] == "" {
		t.Error("user1's refresh token should still work after user2 attempted to revoke it")
	}
}

func TestLegacyTokenWithoutSessionID(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "legacy@example.com",
		PasswordHash: passwordHash,
		Name:         "Legacy User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := services.GenerateSecureToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	tokenHash, err := services.HashToken(token)
	if err != nil {
		t.Fatalf("hash token: %v", err)
	}

	legacyToken := services.RefreshToken{
		UserID:    user.ID,
		SessionID: "",
		TokenHash: tokenHash,
		UserAgent: "legacy-device",
		IPAddress: "127.0.0.1",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&legacyToken).Error; err != nil {
		t.Fatalf("create legacy token: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": token,
	}

	refreshResp := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusOK)

	if refreshResp["access_token"] == nil || refreshResp["access_token"] == "" {
		t.Error("expected access_token in refresh response for legacy token")
	}
	if refreshResp["refresh_token"] == nil || refreshResp["refresh_token"] == "" {
		t.Error("expected new refresh_token in refresh response for legacy token")
	}

	// Verify the new token has a session_id
	var storedToken services.RefreshToken
	if err := db.Where("token_hash IS NOT NULL").Order("created_at desc").First(&storedToken).Error; err != nil {
		t.Fatalf("find new token: %v", err)
	}

	if storedToken.SessionID == "" {
		t.Error("expected new refresh token to have a session_id after refresh")
	}
}

func TestLogoutRequiresSessionIdentifier(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "logout-require@example.com",
		PasswordHash: passwordHash,
		Name:         "Logout Require User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	_, err = authSvc.CreateSession(user.ID, "test-device", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	logoutReq := map[string]any{}

	errResp := requestErrorAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusBadRequest)
	if errResp["error"] != "must provide either refresh_token or all_sessions" {
		t.Errorf("expected error about missing session identifier, got %q", errResp["error"])
	}

	sessionsBefore := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsBefore) != 1 {
		t.Errorf("expected 1 session before logout attempt, got %d", len(sessionsBefore))
	}
}

func TestRefreshResponseDoesNotIncludeUser(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "refresh-nouser@example.com",
		PasswordHash: passwordHash,
		Name:         "Refresh No User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-device", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	refreshReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	refreshResp := requestJSONAuth[map[string]any](t, handler, "", http.MethodPost, "/v1/auth/refresh", refreshReq, http.StatusOK)

	if _, exists := refreshResp["user"]; exists {
		t.Error("refresh response should not include user field")
	}
	if refreshResp["access_token"] == nil || refreshResp["access_token"] == "" {
		t.Error("expected access_token in refresh response")
	}
	if refreshResp["refresh_token"] == nil || refreshResp["refresh_token"] == "" {
		t.Error("expected refresh_token in refresh response")
	}
	if refreshResp["expires_in"] == nil {
		t.Error("expected expires_in in refresh response")
	}
}

func TestLogoutRemovesSessionFromList(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "logout-session-remove@example.com",
		PasswordHash: passwordHash,
		Name:         "Session Remove User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "device-to-remove", "192.168.1.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	sessionsBefore := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsBefore) != 1 {
		t.Fatalf("expected 1 session before logout, got %d", len(sessionsBefore))
	}

	logoutReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}
	expectStatusAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusNoContent)

	sessionsAfter := requestJSONAuth[api.PaginatedResponse[sessionResponse]](t, handler, accessToken, http.MethodGet, "/v1/auth/sessions", nil, http.StatusOK).Data
	if len(sessionsAfter) != 0 {
		t.Errorf("expected 0 sessions after logout, got %d - session should be removed from list", len(sessionsAfter))
	}
}

func TestLogoutWithAlreadyRevokedTokenReturnsNotFound(t *testing.T) {
	t.Parallel()

	db, handler := newTestApp(t)

	passwordHash, err := services.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        "already-revoked@example.com",
		PasswordHash: passwordHash,
		Name:         "Already Revoked User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	authSvc := services.NewAuthService(db)
	tokens, err := authSvc.CreateSession(user.ID, "test-device", "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	accessToken, err := services.GenerateJWT(user.ID, services.AccessTokenTTL)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	logoutReq := map[string]any{
		"refresh_token": tokens.RefreshToken,
	}

	expectStatusAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusNoContent)

	errResp := requestErrorAuth(t, handler, accessToken, http.MethodPost, "/v1/auth/logout", logoutReq, http.StatusNotFound)
	if errResp["error"] != "session not found" {
		t.Errorf("expected 'session not found' error for already revoked token, got %q", errResp["error"])
	}
}
