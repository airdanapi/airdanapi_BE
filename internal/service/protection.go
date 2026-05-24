package service

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/store"
)

type ProtectionService struct {
	cfg          config.ProtectionConfig
	rateLimiter  *store.RateLimiterStore
	transactions *store.TransactionGuardStore
	idempotency  *store.IdempotencyStore
	circuits     *store.CircuitBreakerStore
	stopCleanup  chan struct{}
}

type ProtectionDecision struct {
	Allowed        bool
	Status         int
	Code           string
	Message        string
	RetryAfter     time.Duration
	CachedResponse *store.CachedResponse
	IdempotencyKey string
	BodyHash       string
}

func NewProtectionService(cfg config.ProtectionConfig) *ProtectionService {
	service := &ProtectionService{
		cfg:          cfg,
		rateLimiter:  store.NewRateLimiterStore(),
		transactions: store.NewTransactionGuardStore(),
		idempotency:  store.NewIdempotencyStore(),
		circuits:     store.NewCircuitBreakerStore(),
		stopCleanup:  make(chan struct{}),
	}
	service.idempotency.StartCleanup(service.idempotencyTTL(), service.stopCleanup)
	return service
}

func (s *ProtectionService) Before(route domain.Route, principal Principal, method string, path string, body []byte, headers http.Header, now time.Time) ProtectionDecision {
	if s == nil {
		return ProtectionDecision{Allowed: true}
	}

	limit := s.cfg.ReadRateLimitPerMinute
	if route.RouteClass == "transactional" || route.Transactional {
		limit = s.cfg.TransactionalRateLimitPerMinute
	}
	if ok, retryAfter := s.rateLimiter.Allow(principal.UserID+":"+route.RouteClass, limit, now); !ok {
		return ProtectionDecision{
			Status:     http.StatusTooManyRequests,
			Code:       "RATE_LIMITED",
			Message:    "rate limit exceeded",
			RetryAfter: retryAfter,
		}
	}

	decision := ProtectionDecision{Allowed: true}
	if route.Transactional {
		idempotencyHeader := strings.TrimSpace(headers.Get("X-Idempotency-Key"))
		if idempotencyHeader == "" {
			return ProtectionDecision{
				Status:  http.StatusBadRequest,
				Code:    "VALIDATION_FAILED",
				Message: "X-Idempotency-Key is required for transactional routes",
			}
		}

		idempotencyKey := idempotencyCacheKey(method, path, principal.UserID, idempotencyHeader)
		bodyHash := hashBytes(body)
		if cached, ok := s.idempotency.Get(idempotencyKey, now); ok {
			if cached.BodyHash != bodyHash {
				return ProtectionDecision{
					Status:  http.StatusConflict,
					Code:    "IDEMPOTENCY_CONFLICT",
					Message: "idempotency key was used with a different request body",
				}
			}
			return ProtectionDecision{Allowed: true, CachedResponse: cached}
		}

		if ok, retryAfter, message := s.transactions.Allow(principal.UserID, s.cooldown(), s.cfg.TransactionDailyLimit, now); !ok {
			return ProtectionDecision{
				Status:     http.StatusTooManyRequests,
				Code:       "RATE_LIMITED",
				Message:    message,
				RetryAfter: retryAfter,
			}
		}

		decision.IdempotencyKey = idempotencyKey
		decision.BodyHash = bodyHash
	}

	if !s.circuits.Allow(route.ServiceName, s.openWindow(), now) {
		return ProtectionDecision{
			Status:  http.StatusServiceUnavailable,
			Code:    "CIRCUIT_OPEN",
			Message: "downstream circuit is open",
		}
	}

	return decision
}

func (s *ProtectionService) After(route domain.Route, decision ProtectionDecision, response store.CachedResponse, timeout bool, transportFailure bool, now time.Time) {
	if s == nil || decision.CachedResponse != nil {
		return
	}

	if timeout || transportFailure || response.StatusCode >= 500 {
		s.circuits.RecordFailure(route.ServiceName, timeout, now)
	} else {
		s.circuits.RecordSuccess(route.ServiceName)
	}

	if route.Transactional && decision.IdempotencyKey != "" && response.StatusCode >= 200 && response.StatusCode < 300 {
		response.BodyHash = decision.BodyHash
		response.ExpiresAt = now.Add(s.idempotencyTTL())
		s.idempotency.Set(decision.IdempotencyKey, response)
	}
}

func (s *ProtectionService) cooldown() time.Duration {
	seconds := s.cfg.TransactionCooldownSeconds
	if seconds <= 0 {
		seconds = 10
	}
	return time.Duration(seconds) * time.Second
}

func (s *ProtectionService) idempotencyTTL() time.Duration {
	hours := s.cfg.IdempotencyTTLHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func (s *ProtectionService) openWindow() time.Duration {
	seconds := s.cfg.CircuitOpenSeconds
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func RetryAfterSeconds(duration time.Duration) string {
	seconds := int(duration.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func idempotencyCacheKey(method, path, userID, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(method + ":" + path + ":" + userID + ":" + idempotencyKey))
	return hex.EncodeToString(sum[:])
}

func hashBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
