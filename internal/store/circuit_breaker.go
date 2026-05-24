package store

import (
	"sync"
	"time"
)

const (
	CircuitClosed   = "CLOSED"
	CircuitOpen     = "OPEN"
	CircuitHalfOpen = "HALF_OPEN"
)

type CircuitBreakerStore struct {
	mu       sync.Mutex
	circuits map[string]*CircuitBreaker
}

type CircuitBreaker struct {
	State              string
	FailureCount       int
	Timeouts           []time.Time
	OpenedAt           time.Time
	HalfOpenProbeInUse bool
}

func NewCircuitBreakerStore() *CircuitBreakerStore {
	return &CircuitBreakerStore{circuits: map[string]*CircuitBreaker{}}
}

func (s *CircuitBreakerStore) Allow(key string, openWindow time.Duration, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb := s.getLocked(key)
	if cb.State == CircuitOpen {
		if now.Sub(cb.OpenedAt) >= openWindow {
			cb.State = CircuitHalfOpen
			cb.HalfOpenProbeInUse = true
			return true
		}
		return false
	}
	if cb.State == CircuitHalfOpen {
		if cb.HalfOpenProbeInUse {
			return false
		}
		cb.HalfOpenProbeInUse = true
	}
	return true
}

func (s *CircuitBreakerStore) RecordSuccess(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb := s.getLocked(key)
	cb.State = CircuitClosed
	cb.FailureCount = 0
	cb.Timeouts = nil
	cb.OpenedAt = time.Time{}
	cb.HalfOpenProbeInUse = false
}

func (s *CircuitBreakerStore) RecordFailure(key string, timedOut bool, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb := s.getLocked(key)
	if cb.State == CircuitHalfOpen {
		cb.State = CircuitOpen
		cb.OpenedAt = now
		cb.HalfOpenProbeInUse = false
		return
	}

	cb.FailureCount++
	if timedOut {
		cutoff := now.Add(-30 * time.Second)
		timeouts := cb.Timeouts[:0]
		for _, timeoutAt := range cb.Timeouts {
			if timeoutAt.After(cutoff) {
				timeouts = append(timeouts, timeoutAt)
			}
		}
		cb.Timeouts = append(timeouts, now)
	}

	if cb.FailureCount >= 5 || len(cb.Timeouts) >= 3 {
		cb.State = CircuitOpen
		cb.OpenedAt = now
		cb.HalfOpenProbeInUse = false
	}
}

func (s *CircuitBreakerStore) getLocked(key string) *CircuitBreaker {
	cb := s.circuits[key]
	if cb == nil {
		cb = &CircuitBreaker{State: CircuitClosed}
		s.circuits[key] = cb
	}
	return cb
}
