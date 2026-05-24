package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"
)

var ErrConsoleUnauthorized = errors.New("console credentials are invalid")

type ConsoleOperatorRepository interface {
	FindByEmail(ctx context.Context, email string) (domain.Operator, error)
	UpdateLastLogin(ctx context.Context, id int64) error
}

type ConsoleSession struct {
	Token     string
	Operator  domain.Operator
	ExpiresAt time.Time
}

type ConsoleSessionStore struct {
	mu       sync.RWMutex
	ttl      time.Duration
	sessions map[string]ConsoleSession
}

func NewConsoleSessionStore(ttl time.Duration) *ConsoleSessionStore {
	return &ConsoleSessionStore{
		ttl:      ttl,
		sessions: make(map[string]ConsoleSession),
	}
}

func (s *ConsoleSessionStore) Login(ctx context.Context, repo ConsoleOperatorRepository, email, password string) (ConsoleSession, error) {
	if repo == nil {
		return ConsoleSession{}, repository.ErrNotFound
	}

	operator, err := repo.FindByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ConsoleSession{}, ErrConsoleUnauthorized
		}
		return ConsoleSession{}, err
	}
	if !operator.IsActive || !VerifyPassword(password, operator.PasswordHash) {
		return ConsoleSession{}, ErrConsoleUnauthorized
	}

	token, err := randomToken()
	if err != nil {
		return ConsoleSession{}, err
	}

	session := ConsoleSession{
		Token:     token,
		Operator:  operator,
		ExpiresAt: time.Now().Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[token] = session
	s.mu.Unlock()

	_ = repo.UpdateLastLogin(ctx, operator.ID)
	return session, nil
}

func (s *ConsoleSessionStore) Find(token string) (ConsoleSession, bool) {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return ConsoleSession{}, false
	}
	if time.Now().After(session.ExpiresAt) {
		s.Logout(token)
		return ConsoleSession{}, false
	}
	return session, true
}

func (s *ConsoleSessionStore) Logout(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
