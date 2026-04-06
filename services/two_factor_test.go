package services

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fitness-tracker/models"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTwoFactorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("PRAGMA busy_timeout = 1000").Error; err != nil {
		t.Fatalf("set sqlite busy timeout: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.TwoFactorSecret{}, &models.RecoveryCode{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	return db
}

func createTwoFactorTestUser(t *testing.T, db *gorm.DB) models.User {
	t.Helper()

	passwordHash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := models.User{
		Email:        uuid.NewString() + "@example.com",
		PasswordHash: passwordHash,
		Name:         "Two Factor User",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

func TestTwoFactorSetupVerifyAndDisable(t *testing.T) {
	db := newTwoFactorTestDB(t)
	user := createTwoFactorTestUser(t, db)
	svc := NewTwoFactorService(db)

	secret, _, err := svc.BeginSetup(user.ID, user.Email)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}

	code, err := totp.GenerateCode(secret.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}

	recoveryCodes, err := svc.VerifySetup(user.ID, code)
	if err != nil {
		t.Fatalf("verify setup: %v", err)
	}
	if len(recoveryCodes) != TwoFactorRecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", TwoFactorRecoveryCodeCount, len(recoveryCodes))
	}

	enabled, err := svc.IsEnabled(user.ID)
	if err != nil {
		t.Fatalf("check enabled: %v", err)
	}
	if !enabled {
		t.Fatal("expected 2FA to be enabled")
	}

	disableCode, err := totp.GenerateCode(secret.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate disable TOTP code: %v", err)
	}
	if err := svc.Disable(user.ID, disableCode); err != nil {
		t.Fatalf("disable 2FA: %v", err)
	}

	enabled, err = svc.IsEnabled(user.ID)
	if err != nil {
		t.Fatalf("check enabled after disable: %v", err)
	}
	if enabled {
		t.Fatal("expected 2FA to be disabled")
	}
}

func TestTwoFactorRecoveryCodeConsumption(t *testing.T) {
	db := newTwoFactorTestDB(t)
	user := createTwoFactorTestUser(t, db)
	svc := NewTwoFactorService(db)

	secret, _, err := svc.BeginSetup(user.ID, user.Email)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}

	code, err := totp.GenerateCode(secret.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}

	recoveryCodes, err := svc.VerifySetup(user.ID, code)
	if err != nil {
		t.Fatalf("verify setup: %v", err)
	}

	if err := svc.ConsumeRecoveryCode(user.ID, recoveryCodes[0]); err != nil {
		t.Fatalf("consume recovery code: %v", err)
	}

	if err := svc.ConsumeRecoveryCode(user.ID, recoveryCodes[0]); err == nil {
		t.Fatal("expected reused recovery code to fail")
	}
}

func TestTwoFactorRecoveryCodeConsumptionIsAtomic(t *testing.T) {
	db := newTwoFactorTestDB(t)
	user := createTwoFactorTestUser(t, db)
	svc := NewTwoFactorService(db)

	secret, _, err := svc.BeginSetup(user.ID, user.Email)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}

	code, err := totp.GenerateCode(secret.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}

	recoveryCodes, err := svc.VerifySetup(user.ID, code)
	if err != nil {
		t.Fatalf("verify setup: %v", err)
	}

	const attempts = 10
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successCount atomic.Int32
	errCh := make(chan error, attempts)

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := svc.ConsumeRecoveryCode(user.ID, recoveryCodes[0])
			if err == nil {
				successCount.Add(1)
				return
			}
			if errors.Is(err, models.ErrInvalidRecoveryCode) {
				return
			}
			if strings.Contains(err.Error(), "database table is locked") {
				return
			}
			if !errors.Is(err, models.ErrInvalidRecoveryCode) {
				errCh <- err
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("expected invalid recovery code error for losing attempts, got: %v", err)
	}

	if got := successCount.Load(); got != 1 {
		t.Fatalf("expected exactly one successful recovery code consumption, got %d", got)
	}
}

func TestTwoFactorRecoveryCodeTransactionRollbackPreservesCode(t *testing.T) {
	db := newTwoFactorTestDB(t)
	user := createTwoFactorTestUser(t, db)
	svc := NewTwoFactorService(db)

	secret, _, err := svc.BeginSetup(user.ID, user.Email)
	if err != nil {
		t.Fatalf("begin setup: %v", err)
	}

	code, err := totp.GenerateCode(secret.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate TOTP code: %v", err)
	}

	recoveryCodes, err := svc.VerifySetup(user.ID, code)
	if err != nil {
		t.Fatalf("verify setup: %v", err)
	}

	txErr := db.Transaction(func(tx *gorm.DB) error {
		if err := svc.ConsumeRecoveryCodeTx(tx, user.ID, recoveryCodes[0]); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if txErr == nil || txErr.Error() != "force rollback" {
		t.Fatalf("expected forced rollback error, got %v", txErr)
	}

	if err := svc.ConsumeRecoveryCode(user.ID, recoveryCodes[0]); err != nil {
		t.Fatalf("expected recovery code to remain usable after rollback, got %v", err)
	}
}
