package store

import (
	"sync"
	"time"
)

type TransactionGuardStore struct {
	mu    sync.Mutex
	users map[string]*transactionUserState
}

type transactionUserState struct {
	lastTransactionAt time.Time
	dailyDate         string
	dailyCount        int
}

func NewTransactionGuardStore() *TransactionGuardStore {
	return &TransactionGuardStore{users: map[string]*transactionUserState{}}
}

func (s *TransactionGuardStore) Allow(userID string, cooldown time.Duration, dailyLimit int, now time.Time) (bool, time.Duration, string) {
	if dailyLimit <= 0 {
		dailyLimit = 10
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.users[userID]
	if state == nil {
		state = &transactionUserState{}
		s.users[userID] = state
	}

	today := now.Format("2006-01-02")
	if state.dailyDate != today {
		state.dailyDate = today
		state.dailyCount = 0
	}

	if !state.lastTransactionAt.IsZero() {
		nextAllowed := state.lastTransactionAt.Add(cooldown)
		if now.Before(nextAllowed) {
			return false, nextAllowed.Sub(now), "transaction cooldown is active"
		}
	}

	if state.dailyCount >= dailyLimit {
		return false, time.Hour, "daily transaction limit exceeded"
	}

	state.lastTransactionAt = now
	state.dailyCount++
	return true, 0, ""
}
