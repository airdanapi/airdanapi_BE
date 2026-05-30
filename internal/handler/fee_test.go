package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/service"

	"github.com/go-chi/chi/v5"
)

func TestFeeListRequiresAdminRead(t *testing.T) {
	handler := feeTestHandler(t, &routingFeeRepository{}, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, nil, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/biaya_layanan_integrasi", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusForbidden, "AUTH_SCOPE_DENIED")
}

func TestFeeListSuccess(t *testing.T) {
	feeRepo := &routingFeeRepository{fees: []domain.GatewayFee{{
		ID:                1,
		RequestID:         "req-1",
		UserID:            "user_123",
		SourceApp:         "marketplace",
		TransactionAmount: 100000,
		FeeAmount:         500,
		FeeRate:           0.005,
		Status:            "SUCCESS",
		MaxRetries:        5,
	}}}
	handler := feeTestHandler(t, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"admin:read"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/biaya_layanan_integrasi", nil)
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
	data := body["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one fee item, got %+v", items)
	}
	item := items[0].(map[string]any)
	if item["request_id"] != "req-1" {
		t.Fatalf("expected snake_case request_id, got %+v", item)
	}
	if item["transaction_amount"] != float64(100000) {
		t.Fatalf("expected snake_case transaction_amount, got %+v", item)
	}
	if _, exists := item["FeeAmount"]; exists {
		t.Fatalf("response must not expose Go field FeeAmount: %+v", item)
	}
}

func TestFeeManualRetrySuccess(t *testing.T) {
	feeRepo := &routingFeeRepository{fees: []domain.GatewayFee{{
		ID:                1,
		RequestID:         "req-1",
		UserID:            "user_123",
		SourceApp:         "marketplace",
		TransactionAmount: 100000,
		FeeAmount:         500,
		FeeRate:           0.005,
		Status:            "PENDING",
		RetryCount:        1,
		MaxRetries:        5,
	}}}
	handler := feeTestHandler(t, feeRepo, service.NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, feeRepo, service.NewSmartBankClient(config.SmartBankConfig{Mode: "mock_success"})))
	token := validationTestToken(t, []string{"admin:write"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/biaya_layanan_integrasi/retry/1", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if feeRepo.fees[0].Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS after retry, got %+v", feeRepo.fees[0])
	}
}

func TestFeeCalculationRounding(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		rate   float64
		want   int64
	}{
		{name: "below_one_rounds_to_zero", amount: 99, rate: 0.005, want: 0},
		{name: "half_rounds_to_one", amount: 200, rate: 0.005, want: 1},
		{name: "zero_amount", amount: 0, rate: 0.005, want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.CalculateFee(tc.amount, tc.rate); got != tc.want {
				t.Fatalf("CalculateFee(%d, %f) = %d, want %d", tc.amount, tc.rate, got, tc.want)
			}
		})
	}
}

func feeTestHandler(t *testing.T, feeRepo *routingFeeRepository, feeService service.FeeService) http.Handler {
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

	h := NewFeeHandler(feeRepo, feeService)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.AuthRequired(authService))
	r.Get("/integrator/biaya_layanan_integrasi", h.List)
	r.Post("/integrator/biaya_layanan_integrasi/retry/{id}", h.Retry)
	return r
}
