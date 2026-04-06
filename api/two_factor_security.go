package api

import (
	"errors"
	"sync"
	"time"

	"fitness-tracker/services"

	"github.com/google/uuid"
)

const (
	twoFactorAttemptLimit  = 5
	twoFactorAttemptWindow = time.Minute
	twoFactorChallengeTTL  = 5 * time.Minute
)

var (
	errTooManyTwoFactorAttempts = errors.New("too many 2FA attempts, try again later")
	errInvalidTwoFactorToken    = errors.New("invalid or expired two_factor_token")
)

type twoFactorAttemptState struct {
	failures    int
	windowStart time.Time
}

type twoFactorAttemptLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	attempts map[string]twoFactorAttemptState
}

func newTwoFactorAttemptLimiter() *twoFactorAttemptLimiter {
	return &twoFactorAttemptLimiter{
		now:      time.Now,
		attempts: make(map[string]twoFactorAttemptState),
	}
}

func (l *twoFactorAttemptLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	state, ok := l.attempts[key]
	if !ok {
		return true
	}
	if now.Sub(state.windowStart) >= twoFactorAttemptWindow {
		delete(l.attempts, key)
		return true
	}
	return state.failures < twoFactorAttemptLimit
}

func (l *twoFactorAttemptLimiter) RegisterFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	state, ok := l.attempts[key]
	if !ok || now.Sub(state.windowStart) >= twoFactorAttemptWindow {
		l.attempts[key] = twoFactorAttemptState{
			failures:    1,
			windowStart: now,
		}
		return
	}

	state.failures++
	l.attempts[key] = state
}

func (l *twoFactorAttemptLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

type twoFactorChallenge struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type twoFactorChallengeStore struct {
	mu         sync.Mutex
	now        func() time.Time
	challenges map[string]twoFactorChallenge
}

func newTwoFactorChallengeStore() *twoFactorChallengeStore {
	return &twoFactorChallengeStore{
		now:        time.Now,
		challenges: make(map[string]twoFactorChallenge),
	}
}

func (s *twoFactorChallengeStore) Issue(userID uuid.UUID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()

	token, err := services.GenerateSecureToken()
	if err != nil {
		return "", err
	}

	s.challenges[token] = twoFactorChallenge{
		UserID:    userID,
		ExpiresAt: s.now().UTC().Add(twoFactorChallengeTTL),
	}
	return token, nil
}

func (s *twoFactorChallengeStore) Resolve(token string) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()

	challenge, ok := s.challenges[token]
	if !ok || challenge.UserID == uuid.Nil {
		return uuid.Nil, errInvalidTwoFactorToken
	}

	return challenge.UserID, nil
}

func (s *twoFactorChallengeStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.challenges, token)
}

func (s *twoFactorChallengeStore) pruneExpiredLocked() {
	now := s.now().UTC()
	for token, challenge := range s.challenges {
		if !challenge.ExpiresAt.After(now) {
			delete(s.challenges, token)
		}
	}
}
