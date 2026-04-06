package models

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type TwoFactorSecret struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	Secret    string    `gorm:"type:varchar(255);not null" json:"-"` // Base32 encoded secret, hidden from JSON
	Verified  bool      `gorm:"default:false;not null" json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

type RecoveryCode struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	CodeHash  string     `gorm:"type:varchar(255);not null" json:"-"` // Bcrypt hash of recovery code
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

var (
	Err2FAAlreadyEnabled   = errors.New("2FA already enabled for this user")
	Err2FANotEnabled       = errors.New("2FA not enabled for this user")
	Err2FANotVerified      = errors.New("2FA secret not verified")
	ErrInvalidTOTP         = errors.New("invalid TOTP code")
	ErrInvalidTOTPFormat   = errors.New("TOTP code must be 6 digits")
	ErrInvalidRecoveryCode = errors.New("invalid or already used recovery code")
	ErrRecoveryCodeUsed    = errors.New("recovery code already used")
)

func (t *TwoFactorSecret) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	if t.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}

	if t.Secret == "" {
		return errors.New("secret is required")
	}

	return nil
}

func (r *RecoveryCode) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}

	if r.UserID == uuid.Nil {
		return errors.New("user_id is required")
	}

	if r.CodeHash == "" {
		return errors.New("code_hash is required")
	}

	return nil
}

func GenerateTwoFactorSecret(email string, issuer string) (*TwoFactorSecret, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	secretBase32 := key.Secret()

	twoFactor := &TwoFactorSecret{
		Secret:   secretBase32,
		Verified: false,
	}

	return twoFactor, key.String(), nil
}

func ValidateTOTP(secret string, code string) (bool, error) {
	if len(code) != 6 {
		return false, ErrInvalidTOTPFormat
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			return false, ErrInvalidTOTPFormat
		}
	}

	valid := totp.Validate(code, secret)
	return valid, nil
}

func GenerateRecoveryCodes(count int) ([]string, []*RecoveryCode, error) {
	codes := make([]string, count)
	recoveryCodes := make([]*RecoveryCode, count)

	for i := 0; i < count; i++ {
		code := generateRecoveryCode()
		codes[i] = code

		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to hash recovery code: %w", err)
		}

		recoveryCodes[i] = &RecoveryCode{
			CodeHash: string(hash),
		}
	}

	return codes, recoveryCodes, nil
}

func generateRecoveryCode() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return strings.ToUpper(base32.StdEncoding.EncodeToString(bytes)[:16])
}

func ValidateRecoveryCode(storedCodes []*RecoveryCode, inputCode string) (*RecoveryCode, bool, error) {
	for _, code := range storedCodes {
		if code.UsedAt != nil {
			continue
		}

		err := bcrypt.CompareHashAndPassword([]byte(code.CodeHash), []byte(inputCode))
		if err == nil {
			return code, true, nil
		}
	}

	return nil, false, nil
}
