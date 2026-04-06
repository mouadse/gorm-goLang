// Package services contains business logic extracted from handlers.
// This layer provides unit-testable business rules separate from HTTP handling.
package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// AccessTokenTTL is the lifetime of access tokens.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is the lifetime of refresh tokens.
	RefreshTokenTTL = 7 * 24 * time.Hour
	// MinPasswordLength is the minimum required password length.
	MinPasswordLength = 8
)

var (
	ErrMissingJWTSecret   = errors.New("JWT_SECRET must be set")
	ErrLegacyPasswordHash = errors.New("account requires password reset before login")
	ErrDuplicateLoginUser = errors.New("duplicate email records require migration before login")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTokenRevoked       = errors.New("token has been revoked")
	ErrSessionNotFound    = errors.New("session not found")
	ErrUnauthorizedAccess = errors.New("unauthorized access")
)

const (
	legacyPendingPasswordHash  = "pending-auth"
	legacyDisabledPasswordHash = "auth-disabled"
)

// TokenType represents the type of authentication token.
type TokenType string

const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

// RefreshToken represents a stored refresh token.
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	SessionID string     `gorm:"type:varchar(255);index" json:"session_id"`
	TokenHash string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"-"`
	UserAgent string     `gorm:"type:varchar(255)" json:"user_agent"`
	IPAddress string     `gorm:"type:varchar(45)" json:"ip_address"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// BeforeCreate sets a new UUID before inserting.
func (r *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// UserSession represents a user session.
type UserSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	SessionID string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"session_id"`
	UserAgent string    `gorm:"type:varchar(255)" json:"user_agent"`
	LastIP    string    `gorm:"type:varchar(45)" json:"last_ip"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// BeforeCreate sets a new UUID before inserting.
func (u *UserSession) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// AuthTokens represents the response from a successful authentication.
type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds until access token expires
}

// AccessTokenClaims represents the signed JWT claims for access tokens.
type AccessTokenClaims struct {
	AuthVersion uint `json:"auth_version,omitempty"`
	jwt.RegisteredClaims
}

// LoginRequest contains credentials for authentication.
type LoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TOTPCode       string `json:"totp_code,omitempty"`
	RecoveryCode   string `json:"recovery_code,omitempty"`
	TwoFactorToken string `json:"two_factor_token,omitempty"`
}

func (r LoginRequest) String() string {
	return fmt.Sprintf("LoginRequest{Email:%q, Password:[redacted], TOTPCode:[redacted], RecoveryCode:[redacted], TwoFactorToken:[redacted]}", r.Email)
}

func (r LoginRequest) GoString() string {
	return r.String()
}

// CreateSessionTx creates a session using the provided transaction.
func (s *AuthService) CreateSessionTx(tx *gorm.DB, userID uuid.UUID, userAgent, ipAddress string) (*AuthTokens, error) {
	return s.createSession(tx, userID, userAgent, ipAddress)
}

// RegisterRequest contains data for user registration.
type RegisterRequest struct {
	Email         string  `json:"email"`
	Password      string  `json:"password"`
	Name          string  `json:"name"`
	Avatar        string  `json:"avatar"`
	Age           int     `json:"age"`
	DateOfBirth   string  `json:"date_of_birth"`
	Weight        float64 `json:"weight"`
	Height        float64 `json:"height"`
	Goal          string  `json:"goal"`
	ActivityLevel string  `json:"activity_level"`
	TDEE          int     `json:"tdee"`
}

// AuthService provides business logic for authentication and session management.
type AuthService struct {
	db *gorm.DB
}

// NewAuthService creates a new auth service.
func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

// GenerateSecureToken generates a cryptographically secure random token.
func GenerateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashToken creates a hash of a token for storage.
func HashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CompareTokenHash compares a token with its stored hash.
func CompareTokenHash(hash, token string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
}

// GenerateJWT creates a new JWT access token for a user.
func GenerateJWT(userID uuid.UUID, ttl time.Duration) (string, error) {
	return GenerateJWTWithVersion(userID, 0, ttl)
}

// GenerateJWTWithVersion creates a new JWT access token for a user and auth version.
func GenerateJWTWithVersion(userID uuid.UUID, authVersion uint, ttl time.Duration) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, AccessTokenClaims{
		AuthVersion: authVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})

	return token.SignedString(secret)
}

// ValidateJWT validates a JWT token and returns the user ID.
func ValidateJWT(tokenString string) (uuid.UUID, error) {
	userID, _, err := ParseAccessToken(tokenString)
	return userID, err
}

// ParseAccessToken validates a JWT token and returns the user ID and auth version.
func ParseAccessToken(tokenString string) (uuid.UUID, uint, error) {
	secret, err := jwtSecret()
	if err != nil {
		return uuid.Nil, 0, err
	}

	claims := &AccessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, 0, ErrInvalidToken
	}

	if claims.Subject == "" {
		return uuid.Nil, 0, errors.New("missing subject claim")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, 0, errors.New("invalid subject claim")
	}

	return userID, claims.AuthVersion, nil
}

// ValidateJWTConfig checks if JWT configuration is valid.
func ValidateJWTConfig() error {
	_, err := jwtSecret()
	return err
}

func jwtSecret() ([]byte, error) {
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		return []byte(secret), nil
	}
	return nil, ErrMissingJWTSecret
}

// NormalizeEmail normalizes an email address.
func NormalizeEmail(raw string) (string, error) {
	email, err := requireNonBlank("email", raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(email), nil
}

// RequirePassword validates password requirements.
func RequirePassword(raw string) (string, error) {
	password, err := requireNonBlank("password", raw)
	if err != nil {
		return "", err
	}
	if len(password) < MinPasswordLength {
		return "", errors.New("password must be at least 8 characters")
	}
	return password, nil
}

// HashPassword creates a bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword compares a password hash with a plain password.
func ComparePassword(passwordHash, password string) error {
	if isLegacyPasswordHash(passwordHash) {
		return ErrLegacyPasswordHash
	}
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

func isLegacyPasswordHash(passwordHash string) bool {
	switch strings.TrimSpace(passwordHash) {
	case "", legacyPendingPasswordHash, legacyDisabledPasswordHash:
		return true
	default:
		return false
	}
}

func requireNonBlank(field, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New(field + " is required")
	}
	return value, nil
}

// CreateSession creates a new user session with refresh token.
func (s *AuthService) CreateSession(userID uuid.UUID, userAgent, ipAddress string) (*AuthTokens, error) {
	return s.createSession(s.db, userID, userAgent, ipAddress)
}

func (s *AuthService) createSession(db *gorm.DB, userID uuid.UUID, userAgent, ipAddress string) (*AuthTokens, error) {
	refreshToken, err := GenerateSecureToken()
	if err != nil {
		return nil, err
	}

	// Hash refresh token for storage
	tokenHash, err := HashToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Generate session ID
	sessionID, err := GenerateSecureToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(RefreshTokenTTL)

	// Create session first
	session := UserSession{
		UserID:    userID,
		SessionID: sessionID,
		UserAgent: userAgent,
		LastIP:    ipAddress,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	if err := db.Create(&session).Error; err != nil {
		return nil, err
	}

	// Store refresh token linked to session
	rt := RefreshToken{
		UserID:    userID,
		SessionID: sessionID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IPAddress: ipAddress,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	if err := db.Create(&rt).Error; err != nil {
		// Cleanup session if refresh token creation fails outside an explicit transaction.
		db.Delete(&session)
		return nil, err
	}

	authVersion, err := s.userAuthVersionWithDB(db, userID)
	if err != nil {
		return nil, err
	}

	// Generate access token
	accessToken, err := GenerateJWTWithVersion(userID, authVersion, AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

func (s *AuthService) findRefreshToken(rawToken string) (*RefreshToken, error) {
	var candidates []RefreshToken
	now := time.Now().UTC()
	if err := s.db.
		Where("expires_at > ?", now).
		Order("created_at desc").
		Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("find refresh token candidates: %w", err)
	}

	for i := range candidates {
		if err := CompareTokenHash(candidates[i].TokenHash, rawToken); err == nil {
			if candidates[i].RevokedAt != nil {
				return nil, ErrTokenRevoked
			}
			return &candidates[i], nil
		}
	}

	return nil, ErrInvalidToken
}

// RefreshSession validates a refresh token and rotates it within the same session.
func (s *AuthService) RefreshSession(refreshToken, userAgent, ipAddress string) (*AuthTokens, error) {
	storedToken, err := s.findRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user
	var user models.User
	if err := s.db.First(&user, "id = ?", storedToken.UserID).Error; err != nil {
		return nil, err
	}

	// Revoke old token
	now := time.Now().UTC()
	if err := s.db.Model(storedToken).Update("revoked_at", now).Error; err != nil {
		return nil, err
	}

	// Find or create session for this token
	var session UserSession
	var sessionID string

	if storedToken.SessionID != "" {
		// Try to find existing session
		if err := s.db.Where("session_id = ?", storedToken.SessionID).First(&session).Error; err == nil {
			sessionID = storedToken.SessionID
		}
	}

	// If no session found (legacy token or session deleted), create a new one
	if sessionID == "" {
		sessionID, err = GenerateSecureToken()
		if err != nil {
			return nil, err
		}
	}

	expiresAt := now.Add(RefreshTokenTTL)

	// Generate new refresh token
	newRefreshToken, err := GenerateSecureToken()
	if err != nil {
		return nil, err
	}

	tokenHash, err := HashToken(newRefreshToken)
	if err != nil {
		return nil, err
	}

	// Store new refresh token with session ID
	newRt := RefreshToken{
		UserID:    user.ID,
		SessionID: sessionID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IPAddress: ipAddress,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	if err := s.db.Create(&newRt).Error; err != nil {
		return nil, err
	}

	// Create or update session
	if session.ID != uuid.Nil {
		// Update existing session
		if err := s.db.Model(&session).Updates(map[string]any{
			"last_ip":    ipAddress,
			"expires_at": expiresAt,
		}).Error; err != nil {
			// Non-fatal - session still valid
		}
	} else {
		// Create new session for legacy tokens
		session := UserSession{
			UserID:    user.ID,
			SessionID: sessionID,
			UserAgent: userAgent,
			LastIP:    ipAddress,
			CreatedAt: now,
			ExpiresAt: expiresAt,
		}
		if err := s.db.Create(&session).Error; err != nil {
			// Non-fatal - token is still valid
		}
	}

	// Generate access token
	accessToken, err := GenerateJWTWithVersion(user.ID, user.AuthVersion, AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

// RevokeSession revokes a refresh token.
func (s *AuthService) RevokeSession(refreshToken string) error {
	storedToken, err := s.findRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return ErrSessionNotFound
		}
		return err
	}

	now := time.Now().UTC()
	return s.db.Model(storedToken).Update("revoked_at", now).Error
}

// RevokeUserSession revokes a refresh token after verifying it belongs to the user.
func (s *AuthService) RevokeUserSession(userID uuid.UUID, refreshToken string) error {
	storedToken, err := s.findRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrTokenRevoked) {
			return ErrSessionNotFound
		}
		return err
	}

	if storedToken.UserID != userID {
		return ErrUnauthorizedAccess
	}

	now := time.Now().UTC()
	if storedToken.SessionID != "" {
		if err := s.db.Model(&RefreshToken{}).
			Where("session_id = ? AND revoked_at IS NULL", storedToken.SessionID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := s.db.Where("session_id = ?", storedToken.SessionID).Delete(&UserSession{}).Error; err != nil {
			return err
		}
		return nil
	}

	if err := s.db.Model(storedToken).Update("revoked_at", now).Error; err != nil {
		return err
	}

	return nil
}

func (s *AuthService) userAuthVersion(userID uuid.UUID) (uint, error) {
	return s.userAuthVersionWithDB(s.db, userID)
}

func (s *AuthService) userAuthVersionWithDB(db *gorm.DB, userID uuid.UUID) (uint, error) {
	var user models.User
	if err := db.Select("auth_version").First(&user, "id = ?", userID).Error; err != nil {
		return 0, err
	}
	return user.AuthVersion, nil
}

// GetUserSessions returns all active sessions for a user.
func (s *AuthService) GetUserSessions(userID uuid.UUID) ([]UserSession, error) {
	var sessions []UserSession
	err := s.db.Where("user_id = ? AND expires_at > ?", userID, time.Now().UTC()).
		Order("created_at desc").
		Find(&sessions).Error
	return sessions, err
}

// DeleteSession deletes a specific session and revokes its refresh tokens.
func (s *AuthService) DeleteSession(userID, sessionID uuid.UUID) error {
	// Get the session to find its session_id string
	var session UserSession
	if err := s.db.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSessionNotFound
		}
		return err
	}

	// Revoke all refresh tokens for this session
	now := time.Now().UTC()
	if err := s.db.Model(&RefreshToken{}).
		Where("session_id = ? AND revoked_at IS NULL", session.SessionID).
		Update("revoked_at", now).Error; err != nil {
		return err
	}

	// Delete the session
	result := s.db.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&UserSession{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// DeleteAllUserSessions revokes all sessions for a user (logout from all devices).
func (s *AuthService) DeleteAllUserSessions(userID uuid.UUID) error {
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}

		if err := tx.Where("user_id = ?", userID).Delete(&UserSession{}).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			Update("auth_version", gorm.Expr("COALESCE(auth_version, 0) + 1")).Error; err != nil {
			return err
		}

		return nil
	})
}

// ExtractSessionInfo extracts user agent and IP from an HTTP request.
func ExtractSessionInfo(r *http.Request) (userAgent, ipAddress string) {
	userAgent = r.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = "Unknown"
	}

	// Try various headers for real IP
	ipAddress = r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.Header.Get("X-Real-IP")
	}
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	// Extract just the IP if it includes port
	if idx := strings.LastIndex(ipAddress, ":"); idx != -1 {
		ipAddress = ipAddress[:idx]
	}

	return userAgent, ipAddress
}

// LookupLoginUser finds a user by email for login.
func (s *AuthService) LookupLoginUser(email string) (models.User, error) {
	var users []models.User
	if err := s.db.Where("LOWER(email) = ?", email).Order("created_at asc").Limit(2).Find(&users).Error; err != nil {
		return models.User{}, err
	}
	if len(users) == 0 {
		return models.User{}, errors.New("user not found")
	}
	if len(users) > 1 {
		return models.User{}, ErrDuplicateLoginUser
	}
	return users[0], nil
}

// BackfillNormalizedEmail updates a user's email to normalized form.
func (s *AuthService) BackfillNormalizedEmail(user *models.User, email string) error {
	if user.Email == email {
		return nil
	}
	if err := s.db.Model(user).Update("email", email).Error; err != nil {
		return err
	}
	user.Email = email
	return nil
}

// CreateLocalUser creates a new user with the provided registration data.
func (s *AuthService) CreateLocalUser(req RegisterRequest) (models.User, error) {
	email, err := NormalizeEmail(req.Email)
	if err != nil {
		return models.User{}, err
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		return models.User{}, err
	}

	password, err := RequirePassword(req.Password)
	if err != nil {
		return models.User{}, err
	}

	dateOfBirth, err := parseOptionalBirthDate(req.DateOfBirth)
	if err != nil {
		return models.User{}, err
	}

	if req.Age < 0 || req.TDEE < 0 || req.Weight < 0 || req.Height < 0 {
		return models.User{}, errors.New("numeric profile fields cannot be negative")
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return models.User{}, err
	}

	user := models.User{
		Email:         email,
		PasswordHash:  passwordHash,
		Name:          name,
		Avatar:        strings.TrimSpace(req.Avatar),
		Age:           req.Age,
		DateOfBirth:   dateOfBirth,
		Weight:        req.Weight,
		Height:        req.Height,
		Goal:          strings.TrimSpace(req.Goal),
		ActivityLevel: strings.TrimSpace(req.ActivityLevel),
		TDEE:          req.TDEE,
	}

	if user.TDEE == 0 {
		user.TDEE = user.CalculateTDEE()
	}

	if err := s.db.Create(&user).Error; err != nil {
		return models.User{}, err
	}

	return user, nil
}

func parseOptionalBirthDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("date_of_birth must be YYYY-MM-DD")
	}

	return &parsed, nil
}
