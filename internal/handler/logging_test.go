package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/go-chi/chi/v5"
)

type loggingRepository struct {
	items []domain.RequestLog
}

func (r *loggingRepository) Create(ctx context.Context, entry domain.RequestLog) (int64, error) {
	entry.ID = int64(len(r.items) + 1)
	r.items = append(r.items, entry)
	return entry.ID, nil
}

func (r *loggingRepository) List(ctx context.Context, filter repository.RequestLogFilter) ([]domain.RequestLog, error) {
	return r.items, nil
}

func TestLoggingListRequiresAdminRead(t *testing.T) {
	handler := loggingTestHandler(t, &loggingRepository{})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/logging", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusForbidden, "AUTH_SCOPE_DENIED")
}

func TestLoggingListSuccess(t *testing.T) {
	handler := loggingTestHandler(t, &loggingRepository{items: []domain.RequestLog{{
		ID:        1,
		RequestID: "req-1",
		TargetApp: "marketplace",
		Endpoint:  "/marketplace/checkout",
		Method:    http.MethodPost,
		IPAddress: "127.0.0.1",
		Lifecycle: "COMPLETED",
	}}})
	token := validationTestToken(t, []string{"admin:read"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/logging?page=1&per_page=20", nil)
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
		t.Fatalf("expected one log item, got %+v", items)
	}
}

func TestLoggingListInvalidDate(t *testing.T) {
	handler := loggingTestHandler(t, &loggingRepository{})
	token := validationTestToken(t, []string{"admin:read"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/logging?from=not-a-date", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusBadRequest, "VALIDATION_FAILED")
}

func TestLoggingListRepositoryUnavailable(t *testing.T) {
	handler := loggingTestHandler(t, nil)
	token := validationTestToken(t, []string{"admin:read"})

	req := httptest.NewRequest(http.MethodGet, "/integrator/logging", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertEnvelopeError(t, rec, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE")
}

func loggingTestHandler(t *testing.T, repo repository.RequestLogRepository) http.Handler {
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

	h := NewLoggingHandler(repo)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.AuthRequired(authService))
	r.Get("/integrator/logging", h.List)
	return r
}
