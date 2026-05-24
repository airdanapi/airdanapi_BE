package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/go-chi/chi/v5"
)

type routingRouteRepository struct {
	route domain.Route
	err   error
}

func (r routingRouteRepository) FindActiveByServiceFeatureMethod(ctx context.Context, service, feature, method string) (domain.Route, error) {
	return r.route, r.err
}

func (r routingRouteRepository) ListActive(ctx context.Context) ([]domain.Route, error) {
	return []domain.Route{r.route}, nil
}

func TestTransparentRoutingSuccess(t *testing.T) {
	var receivedBody string
	var receivedQuery string
	var receivedHop string
	var receivedUser string
	var receivedParent string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		receivedBody = body.String()
		receivedQuery = r.URL.RawQuery
		receivedHop = r.Header.Get("X-Hop-Count")
		receivedUser = r.Header.Get("X-User-Id")
		receivedParent = r.Header.Get("X-Parent-Request-Id")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"forwarded":true}`))
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: downstream.URL + "/checkout",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout?order=123", strings.NewReader(`{"amount":10000}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hop-Count", "1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"forwarded":true}` {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if receivedBody != `{"amount":10000}` || receivedQuery != "order=123" {
		t.Fatalf("request was not forwarded intact: body=%q query=%q", receivedBody, receivedQuery)
	}
	if receivedHop != "2" {
		t.Fatalf("expected hop 2, got %q", receivedHop)
	}
	if receivedUser != "user_123" || receivedParent == "" {
		t.Fatalf("expected forwarded user and parent request id, got user=%q parent=%q", receivedUser, receivedParent)
	}
}

func TestEnvelopeRoutingSuccess(t *testing.T) {
	var receivedBody string

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		receivedBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: downstream.URL,
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/routing_api", strings.NewReader(`{"target_service":"marketplace","target_feature":"checkout","method":"POST","payload":{"amount":10000}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["success"] != true {
		t.Fatalf("expected envelope success=true, got %+v", body)
	}
	data := body["data"].(map[string]any)
	if data["ok"] != true {
		t.Fatalf("unexpected envelope data: %+v", data)
	}
	if receivedBody != `{"amount":10000}` {
		t.Fatalf("unexpected forwarded payload: %s", receivedBody)
	}
}

func TestTransactionalRoutingChargesFee(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100000,"status":"paid"}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	fee := body["fee"].(map[string]any)
	if fee["status"] != "success" || int64(fee["amount"].(float64)) != 500 {
		t.Fatalf("unexpected fee body: %+v", fee)
	}
	if len(feeRepo.fees) != 1 || feeRepo.fees[0].Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS fee record, got %+v", feeRepo.fees)
	}
}

func TestTransactionalRoutingDefersFee(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100000}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_failure"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	fee := body["fee"].(map[string]any)
	if fee["status"] != "deferred" {
		t.Fatalf("expected deferred fee, got %+v", fee)
	}
	if len(feeRepo.fees) != 1 || feeRepo.fees[0].Status != "PENDING" {
		t.Fatalf("expected PENDING fee record, got %+v", feeRepo.fees)
	}
}

func TestTransactionalRoutingMissingAmountFails(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"paid"}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusBadGateway, "UPSTREAM_FAILED")
	if len(feeRepo.fees) != 0 {
		t.Fatalf("expected no fee record, got %+v", feeRepo.fees)
	}
}

func TestTransactionalRequiresIdempotencyKey(t *testing.T) {
	handler := routingTestHandlerWithProtection(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: "http://example.test",
			Transactional: true,
			RouteClass:    "transactional",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, nil, service.NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      10,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
		CircuitOpenSeconds:              60,
	}))
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestIdempotencyReplayUsesCachedResponse(t *testing.T) {
	downstreamCalls := 0
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100000,"status":"paid"}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithProtectionAndFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, service.NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      10,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
		CircuitOpenSeconds:              60,
	}), service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"order_id":"order-1"}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Idempotency-Key", "idem-1")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	if downstreamCalls != 1 {
		t.Fatalf("expected one downstream call, got %d", downstreamCalls)
	}
	if len(feeRepo.fees) != 1 {
		t.Fatalf("expected one fee record, got %d", len(feeRepo.fees))
	}
}

func TestIdempotencyConflict(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transaction_amount":100000}`))
	}))
	defer downstream.Close()

	feeRepo := &routingFeeRepository{}
	handler := routingTestHandlerWithProtectionAndFee(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "marketplace",
			DownstreamURL: downstream.URL,
			Transactional: true,
			RouteClass:    "transactional",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	}, service.NewProtectionService(config.ProtectionConfig{
		TransactionalRateLimitPerMinute: 10,
		TransactionCooldownSeconds:      10,
		TransactionDailyLimit:           10,
		IdempotencyTTLHours:             24,
		CircuitOpenSeconds:              60,
	}), service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"order_id":"order-1"}`))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("X-Idempotency-Key", "idem-conflict")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{"order_id":"order-2"}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("X-Idempotency-Key", "idem-conflict")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assertEnvelopeError(t, rec2, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
}

func TestRateLimitExceeded(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer downstream.Close()

	handler := routingTestHandlerWithProtection(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "umkm_insight",
			DownstreamURL: downstream.URL,
			Transactional: false,
			RouteClass:    "read",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("analytics:read"),
		},
	}, nil, service.NewProtectionService(config.ProtectionConfig{
		ReadRateLimitPerMinute: 1,
		CircuitOpenSeconds:     60,
	}))
	token := validationTestToken(t, []string{"analytics:read"})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/umkm_insight/dashboard", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if i == 1 {
			assertEnvelopeError(t, rec, http.StatusTooManyRequests, "RATE_LIMITED")
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header")
			}
		}
	}
}

func TestCircuitOpenAfterFailures(t *testing.T) {
	downstreamCalls := 0
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls++
		http.Error(w, "failed", http.StatusInternalServerError)
	}))
	defer downstream.Close()

	handler := routingTestHandlerWithProtection(t, routingRouteRepository{
		route: domain.Route{
			ServiceName:   "umkm_insight",
			DownstreamURL: downstream.URL,
			Transactional: false,
			RouteClass:    "read",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("analytics:read"),
		},
	}, nil, service.NewProtectionService(config.ProtectionConfig{
		ReadRateLimitPerMinute: 100,
		CircuitOpenSeconds:     60,
	}))
	token := validationTestToken(t, []string{"analytics:read"})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/umkm_insight/dashboard", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected downstream 500, got %d", rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/umkm_insight/dashboard", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusServiceUnavailable, "CIRCUIT_OPEN")
	if downstreamCalls != 5 {
		t.Fatalf("expected circuit to block sixth downstream call, got %d calls", downstreamCalls)
	}
}

func TestRoutingRouteNotFound(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{err: repository.ErrNotFound})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusNotFound, "ROUTE_NOT_FOUND")
}

func TestRoutingScopeDenied(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: "http://example.test",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"analytics:read"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusForbidden, "AUTH_SCOPE_DENIED")
}

func TestRoutingLoopDetected(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: "http://example.test",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Hop-Count", "3")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusLoopDetected, "LOOP_DETECTED")
}

func TestRoutingTimeout(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"late":true}`))
	}))
	defer downstream.Close()

	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: downstream.URL,
			TimeoutMS:     1,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusBadGateway, "UPSTREAM_TIMEOUT")
}

func TestRoutingTransportFailure(t *testing.T) {
	handler := routingTestHandler(t, routingRouteRepository{
		route: domain.Route{
			DownstreamURL: "http://127.0.0.1:1/unreachable",
			TimeoutMS:     5000,
			RequiredScope: stringPointer("marketplace:write"),
		},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusBadGateway, "UPSTREAM_FAILED")
}

func TestRoutingRegistryUnavailable(t *testing.T) {
	handler := routingTestHandler(t, nil)
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/checkout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE")
}

func routingTestHandler(t *testing.T, routes repository.RouteRepository) http.Handler {
	t.Helper()

	_, publicPEM, privateKey := validationTestKeys(t)
	validationPrivateKey = privateKey
	authService, err := service.NewAuthService(config.AuthConfig{
		Issuer:           "smartbank",
		Audience:         "ecosystem",
		PublicKeyPEM:     publicPEM,
		ClockSkewSeconds: 30,
	}, nil)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	h := NewRoutingHandler(routes, nil, nil)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.AuthRequired(authService))
	r.Post("/integrator/routing_api", h.Envelope)
	r.MethodFunc(http.MethodPost, "/api/v1/{service}/{feature}", h.Transparent)
	return r
}

func routingTestHandlerWithFee(t *testing.T, routes repository.RouteRepository, feeRepo repository.GatewayFeeRepository, feeService service.FeeService) http.Handler {
	t.Helper()
	_ = feeRepo

	_, publicPEM, privateKey := validationTestKeys(t)
	validationPrivateKey = privateKey
	authService, err := service.NewAuthService(config.AuthConfig{
		Issuer:           "smartbank",
		Audience:         "ecosystem",
		PublicKeyPEM:     publicPEM,
		ClockSkewSeconds: 30,
	}, nil)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	h := NewRoutingHandler(routes, &feeService, nil)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.AuthRequired(authService))
	r.MethodFunc(http.MethodPost, "/api/v1/{service}/{feature}", h.Transparent)
	return r
}

func routingTestHandlerWithProtection(t *testing.T, routes repository.RouteRepository, feeService *service.FeeService, protection *service.ProtectionService) http.Handler {
	t.Helper()

	_, publicPEM, privateKey := validationTestKeys(t)
	validationPrivateKey = privateKey
	authService, err := service.NewAuthService(config.AuthConfig{
		Issuer:           "smartbank",
		Audience:         "ecosystem",
		PublicKeyPEM:     publicPEM,
		ClockSkewSeconds: 30,
	}, nil)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	h := NewRoutingHandler(routes, feeService, protection)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.AuthRequired(authService))
	r.MethodFunc(http.MethodPost, "/api/v1/{service}/{feature}", h.Transparent)
	return r
}

func routingTestHandlerWithProtectionAndFee(t *testing.T, routes repository.RouteRepository, protection *service.ProtectionService, feeService service.FeeService) http.Handler {
	t.Helper()
	return routingTestHandlerWithProtection(t, routes, &feeService, protection)
}

type routingFeeRepository struct {
	fees []domain.GatewayFee
}

func (r *routingFeeRepository) Create(ctx context.Context, fee domain.GatewayFee) (int64, error) {
	fee.ID = int64(len(r.fees) + 1)
	r.fees = append(r.fees, fee)
	return fee.ID, nil
}

func (r *routingFeeRepository) FindByRequestID(ctx context.Context, requestID string) (domain.GatewayFee, error) {
	for _, fee := range r.fees {
		if fee.RequestID == requestID {
			return fee, nil
		}
	}
	return domain.GatewayFee{}, repository.ErrNotFound
}

func (r *routingFeeRepository) FindByID(ctx context.Context, id int64) (domain.GatewayFee, error) {
	for _, fee := range r.fees {
		if fee.ID == id {
			return fee, nil
		}
	}
	return domain.GatewayFee{}, repository.ErrNotFound
}

func (r *routingFeeRepository) List(ctx context.Context, filter repository.GatewayFeeFilter) ([]domain.GatewayFee, error) {
	return r.fees, nil
}

func (r *routingFeeRepository) UpdateRetryState(ctx context.Context, fee domain.GatewayFee) error {
	for i := range r.fees {
		if r.fees[i].ID == fee.ID {
			r.fees[i] = fee
			return nil
		}
	}
	return nil
}

func (r *routingFeeRepository) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]domain.GatewayFee, error) {
	return r.fees, nil
}

func assertEnvelopeError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	errorBody := body["error"].(map[string]any)
	if errorBody["code"] != code {
		t.Fatalf("expected error code %s, got %v", code, errorBody["code"])
	}
}
