package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/golang-jwt/jwt/v5"
)

type fakeRouteRepository struct {
	route domain.Route
	err   error
}

func (f fakeRouteRepository) FindActiveByServiceFeatureMethod(ctx context.Context, service, feature, method string) (domain.Route, error) {
	return f.route, f.err
}

func (f fakeRouteRepository) ListActive(ctx context.Context) ([]domain.Route, error) {
	return []domain.Route{f.route}, nil
}

func TestValidateRequestSuccess(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{
		route: domain.Route{RequiredScope: stringPointer("marketplace:write")},
	})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{"service":"marketplace","feature":"checkout","method":"POST"}`))
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
	if data["valid"] != true || data["user_id"] != "user_123" {
		t.Fatalf("unexpected response data: %+v", data)
	}
}

func TestValidateRequestScopeDenied(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{
		route: domain.Route{RequiredScope: stringPointer("marketplace:write")},
	})
	token := validationTestToken(t, []string{"analytics:read"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{"service":"marketplace","feature":"checkout","method":"POST"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateRequestRouteNotFound(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{err: repository.ErrNotFound})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{"service":"missing","feature":"route","method":"POST"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestValidateRequestRouteLookupFailure(t *testing.T) {
	handler := validationTestHandler(t, fakeRouteRepository{err: errors.New("db failed")})
	token := validationTestToken(t, []string{"marketplace:write"})

	req := httptest.NewRequest(http.MethodPost, "/integrator/validasi_request", bytes.NewBufferString(`{"service":"marketplace","feature":"checkout","method":"POST"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func validationTestHandler(t *testing.T, routes fakeRouteRepository) http.Handler {
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

	h := NewValidationHandler(routes)
	return middleware.RequestID(middleware.AuthRequired(authService)(http.HandlerFunc(h.ValidateRequest)))
}

var validationPrivateKey *rsa.PrivateKey

func validationTestToken(t *testing.T, scopes []string) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":        "smartbank",
		"aud":        "ecosystem",
		"sub":        "user_123",
		"source_app": "marketplace",
		"roles":      []string{"umkm_owner"},
		"scopes":     scopes,
		"exp":        time.Now().Add(time.Hour).Unix(),
		"iat":        time.Now().Unix(),
		"nbf":        time.Now().Add(-time.Minute).Unix(),
	}).SignedString(validationPrivateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func validationTestKeys(t *testing.T) (string, string, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return "", string(publicPEM), privateKey
}

func stringPointer(value string) *string {
	return &value
}
