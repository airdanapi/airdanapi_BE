package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/repository"

	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	UserID    string
	SourceApp string
	Roles     []string
	Scopes    []string
	JTI       string
	ExpiresAt int64
}

type AuthError struct {
	Status  int
	Code    string
	Message string
}

func (e AuthError) Error() string {
	return e.Message
}

type AuthService struct {
	cfg       config.AuthConfig
	publicKey any
	blacklist repository.JWTBlacklistRepository
}

func NewAuthService(cfg config.AuthConfig, blacklist repository.JWTBlacklistRepository) (*AuthService, error) {
	var publicKey any
	if strings.TrimSpace(cfg.PublicKeyPEM) != "" {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parse jwt public key: %w", err)
		}
		publicKey = key
	}

	return &AuthService{
		cfg:       cfg,
		publicKey: publicKey,
		blacklist: blacklist,
	}, nil
}

func (s *AuthService) ValidateBearer(ctx context.Context, authorization string) (Principal, error) {
	tokenValue, err := bearerToken(authorization)
	if err != nil {
		return Principal{}, err
	}
	if s == nil || s.publicKey == nil {
		return Principal{}, invalidToken("jwt public key is not configured")
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return s.publicKey, nil
	},
		jwt.WithIssuer(s.cfg.Issuer),
		jwt.WithAudience(s.cfg.Audience),
		jwt.WithLeeway(time.Duration(s.cfg.ClockSkewSeconds)*time.Second),
		jwt.WithIssuedAt(),
	)
	if err != nil || !token.Valid {
		return Principal{}, invalidToken("jwt token is invalid")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return Principal{}, invalidToken("jwt subject is required")
	}
	if _, ok := claims["nbf"]; !ok {
		return Principal{}, invalidToken("jwt nbf is required")
	}
	if _, ok := claims["exp"]; !ok {
		return Principal{}, invalidToken("jwt exp is required")
	}
	if _, ok := claims["iat"]; !ok {
		return Principal{}, invalidToken("jwt iat is required")
	}

	jti, _ := claims["jti"].(string)
	if jti != "" && s.blacklist != nil {
		revoked, err := s.blacklist.ExistsActiveJTI(ctx, jti)
		if err != nil {
			return Principal{}, invalidToken("jwt blacklist check failed")
		}
		if revoked {
			return Principal{}, invalidToken("jwt token is revoked")
		}
	}

	sourceApp, _ := claims["source_app"].(string)

	return Principal{
		UserID:    sub,
		SourceApp: sourceApp,
		Roles:     stringListClaim(claims, "roles", "role"),
		Scopes:    stringListClaim(claims, "scopes", "scope"),
		JTI:       jti,
		ExpiresAt: int64NumericClaim(claims, "exp"),
	}, nil
}

func HasScope(principal Principal, requiredScope string) bool {
	requiredScope = strings.TrimSpace(requiredScope)
	if requiredScope == "" {
		return true
	}

	for _, scope := range principal.Scopes {
		if scope == requiredScope {
			return true
		}
	}
	return false
}

func bearerToken(authorization string) (string, error) {
	if strings.TrimSpace(authorization) == "" {
		return "", AuthError{Status: http.StatusUnauthorized, Code: "AUTH_TOKEN_MISSING", Message: "authorization bearer token is required"}
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", invalidToken("authorization header must use bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		return "", AuthError{Status: http.StatusUnauthorized, Code: "AUTH_TOKEN_MISSING", Message: "authorization bearer token is required"}
	}
	return token, nil
}

func invalidToken(message string) AuthError {
	return AuthError{Status: http.StatusUnauthorized, Code: "AUTH_INVALID_TOKEN", Message: message}
}

func stringListClaim(claims jwt.MapClaims, keys ...string) []string {
	for _, key := range keys {
		value, ok := claims[key]
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case string:
			if typed == "" {
				return nil
			}
			return strings.Fields(typed)
		case []string:
			return typed
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				text, ok := item.(string)
				if ok && text != "" {
					items = append(items, text)
				}
			}
			return items
		}
	}

	return nil
}

func int64NumericClaim(claims jwt.MapClaims, key string) int64 {
	value, ok := claims[key]
	if !ok {
		return 0
	}

	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case jsonNumber:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

type jsonNumber interface {
	Int64() (int64, error)
}
