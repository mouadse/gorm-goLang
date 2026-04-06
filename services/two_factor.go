package services

import (
	"errors"
	"strings"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const TwoFactorRecoveryCodeCount = 8

type TwoFactorService struct {
	db *gorm.DB
}

func NewTwoFactorService(db *gorm.DB) *TwoFactorService {
	return &TwoFactorService{db: db}
}

func (s *TwoFactorService) IsEnabled(userID uuid.UUID) (bool, error) {
	var count int64
	err := s.db.Model(&models.TwoFactorSecret{}).
		Where("user_id = ? AND verified = ?", userID, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *TwoFactorService) BeginSetup(userID uuid.UUID, email string) (*models.TwoFactorSecret, string, error) {
	enabled, err := s.IsEnabled(userID)
	if err != nil {
		return nil, "", err
	}
	if enabled {
		return nil, "", models.Err2FAAlreadyEnabled
	}

	secret, otpURL, err := models.GenerateTwoFactorSecret(email, "Fitness Tracker")
	if err != nil {
		return nil, "", err
	}
	secret.UserID = userID

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.TwoFactorSecret{}).Error; err != nil {
			return err
		}
		return tx.Create(secret).Error
	})
	if err != nil {
		return nil, "", err
	}

	return secret, otpURL, nil
}

func (s *TwoFactorService) VerifySetup(userID uuid.UUID, code string) ([]string, error) {
	var secret models.TwoFactorSecret
	if err := s.db.Where("user_id = ?", userID).First(&secret).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.Err2FANotEnabled
		}
		return nil, err
	}

	valid, err := models.ValidateTOTP(secret.Secret, normalizeTOTPCode(code))
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, models.ErrInvalidTOTP
	}

	if secret.Verified {
		return []string{}, nil
	}

	plainCodes, recoveryCodes, err := models.GenerateRecoveryCodes(TwoFactorRecoveryCodeCount)
	if err != nil {
		return nil, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&secret).Update("verified", true).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
			return err
		}
		for _, recoveryCode := range recoveryCodes {
			recoveryCode.UserID = userID
			if err := tx.Create(recoveryCode).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return plainCodes, nil
}

func (s *TwoFactorService) Disable(userID uuid.UUID, code string) error {
	var secret models.TwoFactorSecret
	if err := s.db.Where("user_id = ? AND verified = ?", userID, true).First(&secret).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Err2FANotEnabled
		}
		return err
	}

	valid, err := models.ValidateTOTP(secret.Secret, normalizeTOTPCode(code))
	if err != nil {
		return err
	}
	if !valid {
		return models.ErrInvalidTOTP
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.RecoveryCode{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", userID).Delete(&models.TwoFactorSecret{}).Error
	})
}

func (s *TwoFactorService) VerifyLoginTOTP(userID uuid.UUID, code string) error {
	var secret models.TwoFactorSecret
	if err := s.db.Where("user_id = ? AND verified = ?", userID, true).First(&secret).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Err2FANotEnabled
		}
		return err
	}

	valid, err := models.ValidateTOTP(secret.Secret, normalizeTOTPCode(code))
	if err != nil {
		return err
	}
	if !valid {
		return models.ErrInvalidTOTP
	}

	return nil
}

func (s *TwoFactorService) ConsumeRecoveryCode(userID uuid.UUID, code string) error {
	return s.consumeRecoveryCode(s.db, userID, code)
}

func (s *TwoFactorService) ConsumeRecoveryCodeTx(tx *gorm.DB, userID uuid.UUID, code string) error {
	return s.consumeRecoveryCode(tx, userID, code)
}

func (s *TwoFactorService) consumeRecoveryCode(db *gorm.DB, userID uuid.UUID, code string) error {
	var recoveryCodes []*models.RecoveryCode
	if err := db.Where("user_id = ? AND used_at IS NULL", userID).Find(&recoveryCodes).Error; err != nil {
		return err
	}

	matched, valid, err := models.ValidateRecoveryCode(recoveryCodes, normalizeRecoveryCode(code))
	if err != nil {
		return err
	}
	if !valid {
		return models.ErrInvalidRecoveryCode
	}

	now := time.Now().UTC()
	result := db.Model(&models.RecoveryCode{}).
		Where("id = ? AND user_id = ? AND used_at IS NULL", matched.ID, userID).
		Update("used_at", &now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return models.ErrInvalidRecoveryCode
	}

	return nil
}

func normalizeTOTPCode(code string) string {
	return strings.ReplaceAll(strings.TrimSpace(code), " ", "")
}

func normalizeRecoveryCode(code string) string {
	trimmed := strings.TrimSpace(code)
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	return strings.ToUpper(trimmed)
}
