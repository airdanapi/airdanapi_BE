package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/golang-jwt/jwt/v5"
)

// TestGW_T01_ReadRequestSuccess verifies successful transparent proxy for read requests
func TestGW_T01_ReadRequestSuccess(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"success"}`))
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: false,
			RouteClass:    "read",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:read"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:read"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/items", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"data":"success"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

// TestGW_T02_TransactionalPlusFee verifies fee calculation and transaction processing
func TestGW_T02_TransactionalPlusFee(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":200000}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005, // 0.5%
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	fee := body["fee"].(map[string]any)
	if fee["status"] != "success" || int64(fee["amount"].(float64)) != 1000 {
		t.Fatalf("expected fee amount 1000 and status success, got %+v", fee)
	}
}

// TestGW_T03_MissingJWT verifies rejection when no token is provided
func TestGW_T03_MissingJWT(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{
		route: domain.Route{RequiredScope: stringPointer("marketplace:write")},
	})
	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{"service":"marketplace","feature":"checkout","method":"POST"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusUnauthorized, "AUTH_TOKEN_MISSING")
}

// TestGW_T04_ExpiredJWT verifies rejection of expired token
func TestGW_T04_ExpiredJWT(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{})
	// Create an expired token manually
	_, _, privateKey := validationTestKeys(t)
	token := signTestToken(t, privateKey, map[string]any{
		"iss": "smartbank",
		"aud": "ecosystem",
		"sub": "user_123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assertEnvelopeError(t, rec, http.StatusUnauthorized, "AUTH_INVALID_TOKEN")
}

// TestGW_T05_RevokedToken verifies rejection of blacklisted token
func TestGW_T05_RevokedToken(t *testing.T) {
	// Need a custom handler with blacklist mock
	_, publicPEM, privateKey := validationTestKeys(t)
	validationPrivateKey = privateKey
	
	// Create auth service with a mocked blacklist that returns true for revoked
	authService, _ := service.NewAuthService(config.AuthConfig{
		Issuer:           "smartbank",
		Audience:         "ecosystem",
		PublicKeyPEM:     publicPEM,
		ClockSkewSeconds: 30,
	}, mockBlacklist{revoked: true})

	h := NewValidationHandler(fakeRouteRepository{})
	handler := middleware.RequestID(middleware.AuthRequired(authService)(http.HandlerFunc(h.ValidateRequest)))

	token := signTestToken(t, privateKey, map[string]any{
		"iss": "smartbank",
		"aud": "ecosystem",
		"sub": "user_123",
		"jti": "revoked-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assertEnvelopeError(t, rec, http.StatusUnauthorized, "AUTH_INVALID_TOKEN")
}

// TestGW_T06_UnknownRoute verifies routing rejects unknown paths
func TestGW_T06_UnknownRoute(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{err: repository.ErrNotFound})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unknown/route", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assertEnvelopeError(t, rec, http.StatusNotFound, "ROUTE_NOT_FOUND")
}

// TestGW_T07_RateLimitExceeded verifies rate limiter blocks excessive requests
func TestGW_T07_RateLimitExceeded(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer downstream.Close()

	handler := routingTestHandlerWithProtection(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			RouteClass:    "read",
		},
	}, nil, service.NewProtectionService(config.ProtectionConfig{
		ReadRateLimitPerMinute: 1, // Only 1 request allowed per minute
	}))
	token := validationTestToken(t, []string{})

	// 1st request should pass
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/items", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request failed: %d", rec1.Code)
	}

	// 2nd request should be rate limited
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/items", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	assertEnvelopeError(t, rec2, http.StatusTooManyRequests, "RATE_LIMITED")
}

// TestGW_T08_IdempotencyReplay verifies replay of same idempotency key
func TestGW_T08_IdempotencyReplay(t *testing.T) {
	downstreamCalls := 0
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithProtectionAndFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
		},
	}, service.NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      10,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
	}), service.NewFeeService(config.FeeConfig{Rate: 0.005}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"order":1}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Idempotency-Key", "idem-t08")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 on attempt %d, got %d", i+1, rec.Code)
		}
	}

	if downstreamCalls != 1 {
		t.Fatalf("expected 1 downstream call, got %d", downstreamCalls)
	}
}

// TestGW_T09_IdempotencyConflict verifies conflict for same key but different body
func TestGW_T09_IdempotencyConflict(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100}`))
	}))
	defer downstream.Close()
	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithProtectionAndFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
		},
	}, service.NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      0,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
	}), service.NewFeeService(config.FeeConfig{Rate: 0.005}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{})

	// 1st request
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"transaction_amount":100,"order":1}`))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("X-Idempotency-Key", "idem-t09")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// 2nd request with different body
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"order":2}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("X-Idempotency-Key", "idem-t09")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	
	assertEnvelopeError(t, rec2, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
}

// TestGW_T10_DownstreamTimeout verifies gateway timeouts slow downstreams
func TestGW_T10_DownstreamTimeout(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: downstream.URL,
			TimeoutMS:     1, // Extremely short timeout
		},
	})
	token := validationTestToken(t, []string{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/slow", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assertEnvelopeError(t, rec, http.StatusBadGateway, "UPSTREAM_TIMEOUT")
}

// TestGW_T11_CircuitOpen verifies circuit breaker opens after repeated failures
func TestGW_T11_CircuitOpen(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer downstream.Close()

	handler := routingTestHandlerWithProtection(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			RouteClass:    "read",
		},
	}, nil, service.NewProtectionService(config.ProtectionConfig{
		ReadRateLimitPerMinute: 100,
		CircuitOpenSeconds:     60,
	}))
	token := validationTestToken(t, []string{})

	// Fire 5 failing requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/fail", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	// 6th request should be blocked by circuit breaker
	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/fail", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	assertEnvelopeError(t, rec, http.StatusServiceUnavailable, "CIRCUIT_OPEN")
}

// TestGW_T12_FeeChargeFailed verifies fee is deferred on smartbank failure
func TestGW_T12_FeeChargeFailed(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":200000}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
		},
	}, feeRepo, service.NewFeeService(config.FeeConfig{
		Rate: 0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_failure"}))) // Mock fails
	
	token := validationTestToken(t, []string{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	fee := body["fee"].(map[string]any)
	if fee["status"] != "deferred" {
		t.Fatalf("expected fee deferred, got %+v", fee)
	}
}

// TestGW_T13_ScopeDenied verifies route authorization
func TestGW_T13_ScopeDenied(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			RequiredScope: stringPointer("marketplace:admin"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:read"}) // Different scope

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	assertEnvelopeError(t, rec, http.StatusForbidden, "AUTH_SCOPE_DENIED")
}

// TestGW_T14_LogQuery verifies the logging endpoint
func TestGW_T14_LogQuery(t *testing.T) {
	handler := loggingTestHandler(t, &loggingRepository{items: []domain.RequestLog{{
		RequestID: "req-123",
	}}})
	token := validationTestToken(t, []string{"admin:read"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/logging", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	items := body["data"].(map[string]any)["items"].([]any)
	if items[0].(map[string]any)["request_id"] != "req-123" {
		t.Fatalf("unexpected log item: %+v", items[0])
	}
}

// TestGW_T15_ConcurrentCalls verifies gateway handles concurrent requests without race conditions
func TestGW_T15_ConcurrentCalls(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"success"}`))
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			RouteClass:    "read",
		},
	})
	token := validationTestToken(t, []string{})

	var wg sync.WaitGroup
	concurrentReqs := 10

	for i := 0; i < concurrentReqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/items", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}
	
	wg.Wait()
}

// Helper mock for JWT blacklist
type mockBlacklist struct {
	revoked bool
}
func (m mockBlacklist) ExistsActiveJTI(ctx context.Context, jti string) (bool, error) {
	return m.revoked, nil
}

// signTestToken is a helper for manual token signing
func signTestToken(t *testing.T, privateKey interface{}, claims map[string]any) string {
	token, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims)).SignedString(privateKey)
	return token
}
