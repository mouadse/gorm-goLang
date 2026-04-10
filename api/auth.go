package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL    = 24 * time.Hour
	minPasswordLength = 8
)

const (
	legacyPendingPasswordHash  = "pending-auth"
	legacyDisabledPasswordHash = "auth-disabled"
)

var (
	errMissingJWTSecret   = errors.New("JWT_SECRET must be set")
	errLegacyPasswordHash = errors.New("account requires password reset before login")
	errDuplicateLoginUser = errors.New("duplicate email records require migration before login")
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateRegistrationInput(registrationValidationInput{
		Email:       req.Email,
		Password:    req.Password,
		Name:        req.Name,
		DateOfBirth: req.DateOfBirth,
		Age:         req.Age,
		Weight:      req.Weight,
		Height:      req.Height,
		TDEE:        req.TDEE,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.createLocalUser(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeAuthResponse(w, http.StatusCreated, user)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validateCredentialFields(req.Email, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	email, err := normalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	password, err := requirePassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := s.lookupLoginUser(email)
	if err != nil {
		if errors.Is(err, errDuplicateLoginUser) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		return
	}

	if err := comparePassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, errLegacyPasswordHash) {
			writeError(w, http.StatusConflict, err)
			return
		}
		writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
		return
	}

	if err := s.backfillNormalizedEmail(&user, email); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeAuthResponse(w, http.StatusOK, user)
}

func (s *Server) createLocalUser(req createUserRequest) (models.User, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return models.User{}, err
	}

	name, err := requireNonBlank("name", req.Name)
	if err != nil {
		return models.User{}, err
	}

	password, err := requirePassword(req.Password)
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

	passwordHash, err := hashPassword(password)
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

func writeAuthResponse(w http.ResponseWriter, status int, user models.User) {
	token, err := generateJWT(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, status, authResponse{
		Token: token,
		User:  user,
	})
}

func normalizeEmail(raw string) (string, error) {
	email, err := requireNonBlank("email", raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(email), nil
}

func requirePassword(raw string) (string, error) {
	password, err := requireNonBlank("password", raw)
	if err != nil {
		return "", err
	}
	if len(password) < minPasswordLength {
		return "", errors.New("password must be at least 8 characters")
	}
	return password, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func comparePassword(passwordHash, password string) error {
	if isLegacyPasswordHash(passwordHash) {
		return errLegacyPasswordHash
	}
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

func generateJWT(userID uuid.UUID) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
	})

	return token.SignedString(secret)
}

func validateJWT(tokenString string) (uuid.UUID, error) {
	secret, err := jwtSecret()
	if err != nil {
		return uuid.Nil, err
	}

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, errors.New("invalid token")
	}

	if claims.Subject == "" {
		return uuid.Nil, errors.New("missing subject claim")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("invalid subject claim")
	}

	return userID, nil
}

func ValidateJWTConfig() error {
	_, err := jwtSecret()
	return err
}

func (s *Server) lookupLoginUser(email string) (models.User, error) {
	var users []models.User
	if err := s.db.Where("LOWER(email) = ?", email).Order("created_at asc").Limit(2).Find(&users).Error; err != nil {
		return models.User{}, err
	}
	if len(users) == 0 {
		return models.User{}, errors.New("user not found")
	}
	if len(users) > 1 {
		return models.User{}, errDuplicateLoginUser
	}
	return users[0], nil
}

func (s *Server) backfillNormalizedEmail(user *models.User, email string) error {
	if user.Email == email {
		return nil
	}

	if err := s.db.Model(user).Update("email", email).Error; err != nil {
		return err
	}
	user.Email = email
	return nil
}

func jwtSecret() ([]byte, error) {
	if secret := strings.TrimSpace(os.Getenv("JWT_SECRET")); secret != "" {
		return []byte(secret), nil
	}
	return nil, errMissingJWTSecret
}

func isLegacyPasswordHash(passwordHash string) bool {
	switch strings.TrimSpace(passwordHash) {
	case "", legacyPendingPasswordHash, legacyDisabledPasswordHash:
		return true
	default:
		return false
	}
}
