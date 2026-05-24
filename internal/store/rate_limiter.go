package store

import (
	"sync"
	"time"
)

type RateLimiterStore struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func NewRateLimiterStore() *RateLimiterStore {
	return &RateLimiterStore{buckets: map[string]*tokenBucket{}}
}

func (s *RateLimiterStore) Allow(key string, limitPerMinute int, now time.Time) (bool, time.Duration) {
	if limitPerMinute <= 0 {
		return true, 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	bucket := s.buckets[key]
	if bucket == nil {
		bucket = &tokenBucket{
			tokens:     float64(limitPerMinute),
			maxTokens:  float64(limitPerMinute),
			refillRate: float64(limitPerMinute) / 60,
			lastRefill: now,
		}
		s.buckets[key] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		bucket.tokens += elapsed * bucket.refillRate
		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}
		bucket.lastRefill = now
	}

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}

	waitSeconds := (1 - bucket.tokens) / bucket.refillRate
	return false, time.Duration(waitSeconds*float64(time.Second)) + time.Second
}
