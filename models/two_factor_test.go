package models

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTwoFactorSecretBeforeCreate(t *testing.T) {
	t.Run("generates UUID when ID is nil", func(t *testing.T) {
		secret := &TwoFactorSecret{
			UserID: uuid.New(),
			Secret: "JBSWY3DPEHPK3PXP",
		}

		if err := secret.BeforeCreate(nil); err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if secret.ID == uuid.Nil {
			t.Error("expected ID to be generated, got nil UUID")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := uuid.New()
		secret := &TwoFactorSecret{
			ID:     existingID,
			UserID: uuid.New(),
			Secret: "JBSWY3DPEHPK3PXP",
		}

		if err := secret.BeforeCreate(nil); err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if secret.ID != existingID {
			t.Errorf("expected ID to remain %v, got %v", existingID, secret.ID)
		}
	})

	t.Run("requires user_id", func(t *testing.T) {
		secret := &TwoFactorSecret{
			Secret: "JBSWY3DPEHPK3PXP",
		}

		err := secret.BeforeCreate(nil)
		if err == nil {
			t.Error("expected error for missing user_id, got nil")
		}
		if err.Error() != "user_id is required" {
			t.Errorf("expected 'user_id is required' error, got %v", err)
		}
	})

	t.Run("requires secret", func(t *testing.T) {
		secret := &TwoFactorSecret{
			UserID: uuid.New(),
		}

		err := secret.BeforeCreate(nil)
		if err == nil {
			t.Error("expected error for missing secret, got nil")
		}
		if err.Error() != "secret is required" {
			t.Errorf("expected 'secret is required' error, got %v", err)
		}
	})
}

func TestRecoveryCodeBeforeCreate(t *testing.T) {
	t.Run("generates UUID when ID is nil", func(t *testing.T) {
		code := &RecoveryCode{
			UserID:   uuid.New(),
			CodeHash: "hashed_code",
		}

		if err := code.BeforeCreate(nil); err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if code.ID == uuid.Nil {
			t.Error("expected ID to be generated, got nil UUID")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		existingID := uuid.New()
		code := &RecoveryCode{
			ID:       existingID,
			UserID:   uuid.New(),
			CodeHash: "hashed_code",
		}

		if err := code.BeforeCreate(nil); err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if code.ID != existingID {
			t.Errorf("expected ID to remain %v, got %v", existingID, code.ID)
		}
	})

	t.Run("requires user_id", func(t *testing.T) {
		code := &RecoveryCode{
			CodeHash: "hashed_code",
		}

		err := code.BeforeCreate(nil)
		if err == nil {
			t.Error("expected error for missing user_id, got nil")
		}
		if err.Error() != "user_id is required" {
			t.Errorf("expected 'user_id is required' error, got %v", err)
		}
	})

	t.Run("requires code_hash", func(t *testing.T) {
		code := &RecoveryCode{
			UserID: uuid.New(),
		}

		err := code.BeforeCreate(nil)
		if err == nil {
			t.Error("expected error for missing code_hash, got nil")
		}
		if err.Error() != "code_hash is required" {
			t.Errorf("expected 'code_hash is required' error, got %v", err)
		}
	})
}

func TestGenerateTwoFactorSecret(t *testing.T) {
	t.Run("generates valid secret and key URI", func(t *testing.T) {
		email := "test@example.com"
		issuer := "TestApp"

		secret, keyURI, err := GenerateTwoFactorSecret(email, issuer)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if secret == nil {
			t.Fatal("expected secret to be generated, got nil")
		}

		if secret.Secret == "" {
			t.Error("expected secret to have a value, got empty string")
		}

		if keyURI == "" {
			t.Error("expected key URI to be generated, got empty string")
		}

		if secret.Verified {
			t.Error("expected secret to be unverified by default")
		}
	})
}

func TestGenerateRecoveryCodes(t *testing.T) {
	t.Run("generates specified number of codes", func(t *testing.T) {
		count := 8
		codes, recoveryCodes, err := GenerateRecoveryCodes(count)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if len(codes) != count {
			t.Errorf("expected %d codes, got %d", count, len(codes))
		}

		if len(recoveryCodes) != count {
			t.Errorf("expected %d recovery code structs, got %d", count, len(recoveryCodes))
		}
	})

	t.Run("generates unique codes", func(t *testing.T) {
		codes, _, err := GenerateRecoveryCodes(10)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		seen := make(map[string]bool)
		for _, code := range codes {
			if seen[code] {
				t.Errorf("duplicate recovery code found: %s", code)
			}
			seen[code] = true
		}
	})

	t.Run("codes are 16 characters", func(t *testing.T) {
		codes, _, err := GenerateRecoveryCodes(5)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		for _, code := range codes {
			if len(code) != 16 {
				t.Errorf("expected code length 16, got %d for code %s", len(code), code)
			}
		}
	})

	t.Run("recovery codes have hashed values", func(t *testing.T) {
		_, recoveryCodes, err := GenerateRecoveryCodes(5)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		for _, rc := range recoveryCodes {
			if rc.CodeHash == "" {
				t.Error("expected recovery code to have hashed value")
			}
		}
	})
}

func TestValidateTOTP(t *testing.T) {
	t.Run("rejects invalid length code", func(t *testing.T) {
		secret := "JBSWY3DPEHPK3PXP"
		code := "12345"

		valid, err := ValidateTOTP(secret, code)
		if !errors.Is(err, ErrInvalidTOTPFormat) {
			t.Fatalf("expected invalid format error, got %v", err)
		}

		if valid {
			t.Error("expected invalid code to be rejected")
		}
	})

	t.Run("rejects non-digit code", func(t *testing.T) {
		secret := "JBSWY3DPEHPK3PXP"
		code := "12A456"

		valid, err := ValidateTOTP(secret, code)
		if !errors.Is(err, ErrInvalidTOTPFormat) {
			t.Fatalf("expected invalid format error, got %v", err)
		}

		if valid {
			t.Error("expected invalid code to be rejected")
		}
	})

	t.Run("rejects invalid code", func(t *testing.T) {
		secret := "JBSWY3DPEHPK3PXP"
		code := "123456"

		valid, err := ValidateTOTP(secret, code)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if valid {
			t.Error("expected invalid code to be rejected")
		}
	})
}
