package service

import (
	"net/http"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/store"
)

func TestProtectionCooldown(t *testing.T) {
	protection := NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      10,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
		CircuitOpenSeconds:              60,
	})
	route := transactionalProtectionRoute()
	principal := Principal{UserID: "user_123"}
	now := time.Now()

	first := protection.Before(route, principal, http.MethodPost, "/api/v1/marketplace/checkout", []byte(`{"id":1}`), headersWithIdempotency("key-1"), now)
	if !first.Allowed {
		t.Fatalf("expected first request allowed, got %+v", first)
	}

	second := protection.Before(route, principal, http.MethodPost, "/api/v1/marketplace/checkout", []byte(`{"id":2}`), headersWithIdempotency("key-2"), now.Add(time.Second))
	if second.Status != http.StatusTooManyRequests || second.Code != "RATE_LIMITED" {
		t.Fatalf("expected cooldown rate limit, got %+v", second)
	}
}

func TestProtectionDailyLimit(t *testing.T) {
	protection := NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 100,
		TransactionCooldownSeconds:      0,
		TransactionDailyLimit:           1,
		IdempotencyTTLHours:             24,
		CircuitOpenSeconds:              60,
	})
	route := transactionalProtectionRoute()
	principal := Principal{UserID: "user_123"}
	now := time.Now()

	first := protection.Before(route, principal, http.MethodPost, "/api/v1/marketplace/checkout", []byte(`{"id":1}`), headersWithIdempotency("key-1"), now)
	if !first.Allowed {
		t.Fatalf("expected first request allowed, got %+v", first)
	}

	second := protection.Before(route, principal, http.MethodPost, "/api/v1/marketplace/checkout", []byte(`{"id":2}`), headersWithIdempotency("key-2"), now.Add(11*time.Second))
	if second.Status != http.StatusTooManyRequests || second.Code != "RATE_LIMITED" {
		t.Fatalf("expected daily rate limit, got %+v", second)
	}
}

func TestProtectionCircuitOpensAfterTimeouts(t *testing.T) {
	protection := NewProtectionService(config.ProtectionConfig{
		ReadRateLimitPerMinute: 100,
		CircuitOpenSeconds:     60,
	})
	route := domain.Route{ServiceName: "marketplace", RouteClass: "read"}
	principal := Principal{UserID: "user_123"}
	now := time.Now()

	for i := 0; i < 3; i++ {
		decision := protection.Before(route, principal, http.MethodGet, "/api/v1/marketplace/status_order", nil, http.Header{}, now.Add(time.Duration(i)*time.Second))
		if !decision.Allowed {
			t.Fatalf("expected request %d allowed, got %+v", i, decision)
		}
		protection.After(route, decision, store.CachedResponse{StatusCode: http.StatusBadGateway}, true, false, now.Add(time.Duration(i)*time.Second))
	}

	blocked := protection.Before(route, principal, http.MethodGet, "/api/v1/marketplace/status_order", nil, http.Header{}, now.Add(4*time.Second))
	if blocked.Status != http.StatusServiceUnavailable || blocked.Code != "CIRCUIT_OPEN" {
		t.Fatalf("expected circuit open, got %+v", blocked)
	}
}

func transactionalProtectionRoute() domain.Route {
	return domain.Route{
		ServiceName:   "marketplace",
		Transactional: true,
		RouteClass:    "transactional",
	}
}

func headersWithIdempotency(key string) http.Header {
	headers := http.Header{}
	headers.Set("X-Idempotency-Key", key)
	return headers
}
