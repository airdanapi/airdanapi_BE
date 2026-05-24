package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/repository"

	"github.com/golang-jwt/jwt/v5"
)

type fakeBlacklist struct {
	revoked bool
}

func (f fakeBlacklist) ExistsActiveJTI(ctx context.Context, jti string) (bool, error) {
	return f.revoked, nil
}

func TestAuthServiceValidateBearer(t *testing.T) {
	privateKey, publicPEM := testRSAKey(t)
	authService := newTestAuthService(t, publicPEM, nil)

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"iss":        "smartbank",
		"aud":        "ecosystem",
		"sub":        "user_123",
		"jti":        "jti-123",
		"source_app": "marketplace",
		"roles":      []string{"umkm_owner"},
		"scopes":     []string{"marketplace:write"},
		"exp":        time.Now().Add(time.Hour).Unix(),
		"iat":        time.Now().Unix(),
		"nbf":        time.Now().Add(-time.Minute).Unix(),
	})

	principal, err := authService.ValidateBearer(context.Background(), "Bearer "+token)
	if err != nil {
		t.Fatalf("validate bearer: %v", err)
	}
	if principal.UserID != "user_123" || !HasScope(principal, "marketplace:write") {
		t.Fatalf("unexpected principal: %+v", principal)
	}
}

func TestAuthServiceMissingToken(t *testing.T) {
	authService := &AuthService{}

	_, err := authService.ValidateBearer(context.Background(), "")
	authErr, ok := err.(AuthError)
	if !ok {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if authErr.Status != http.StatusUnauthorized || authErr.Code != "AUTH_TOKEN_MISSING" {
		t.Fatalf("unexpected auth error: %+v", authErr)
	}
}

func TestAuthServiceExpiredToken(t *testing.T) {
	privateKey, publicPEM := testRSAKey(t)
	authService := newTestAuthService(t, publicPEM, nil)

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"iss": "smartbank",
		"aud": "ecosystem",
		"sub": "user_123",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"nbf": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err := authService.ValidateBearer(context.Background(), "Bearer "+token)
	authErr, ok := err.(AuthError)
	if !ok {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if authErr.Code != "AUTH_INVALID_TOKEN" {
		t.Fatalf("expected AUTH_INVALID_TOKEN, got %s", authErr.Code)
	}
}

func TestAuthServiceRevokedToken(t *testing.T) {
	privateKey, publicPEM := testRSAKey(t)
	authService := newTestAuthService(t, publicPEM, fakeBlacklist{revoked: true})

	token := signTestToken(t, privateKey, jwt.MapClaims{
		"iss": "smartbank",
		"aud": "ecosystem",
		"sub": "user_123",
		"jti": "revoked-jti",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
	})

	_, err := authService.ValidateBearer(context.Background(), "Bearer "+token)
	authErr, ok := err.(AuthError)
	if !ok {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if authErr.Code != "AUTH_INVALID_TOKEN" {
		t.Fatalf("expected AUTH_INVALID_TOKEN, got %s", authErr.Code)
	}
}

func newTestAuthService(t *testing.T, publicPEM string, blacklist repository.JWTBlacklistRepository) *AuthService {
	t.Helper()

	authService, err := NewAuthService(config.AuthConfig{
		Issuer:           "smartbank",
		Audience:         "ecosystem",
		PublicKeyPEM:     publicPEM,
		ClockSkewSeconds: 30,
	}, blacklist)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	return authService
}

func testRSAKey(t *testing.T) (*rsa.PrivateKey, string) {
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
	return privateKey, string(publicPEM)
}

func signTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
