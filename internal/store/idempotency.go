package store

import (
	"net/http"
	"sync"
	"time"
)

type IdempotencyStore struct {
	mu    sync.RWMutex
	cache map[string]*CachedResponse
}

type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	BodyHash   string
	ExpiresAt  time.Time
}

func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{cache: map[string]*CachedResponse{}}
}

func (s *IdempotencyStore) Get(key string, now time.Time) (*CachedResponse, bool) {
	s.mu.RLock()
	entry := s.cache[key]
	s.mu.RUnlock()
	if entry == nil {
		return nil, false
	}
	if now.After(entry.ExpiresAt) {
		s.mu.Lock()
		delete(s.cache, key)
		s.mu.Unlock()
		return nil, false
	}
	return entry, true
}

func (s *IdempotencyStore) Set(key string, response CachedResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = &response
}

func (s *IdempotencyStore) StartCleanup(ttl time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				s.cleanup(now)
			}
		}
	}()
}

func (s *IdempotencyStore) cleanup(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.cache {
		if now.After(entry.ExpiresAt) {
			delete(s.cache, key)
		}
	}
}
