package middleware

import (
	"context"
	"net/http"
	"strings"

	"airdanapi-be/internal/response"
	"airdanapi-be/internal/service"
)

type consoleSessionContextKey struct{}

func ConsoleAuthRequired(store *service.ConsoleSessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				response.WriteJSON(w, http.StatusUnauthorized, response.Error(r.Context(), "AUTH_TOKEN_MISSING", "console session token is required", http.StatusUnauthorized))
				return
			}

			session, ok := store.Find(token)
			if !ok {
				response.WriteJSON(w, http.StatusUnauthorized, response.Error(r.Context(), "AUTH_INVALID_TOKEN", "console session token is invalid", http.StatusUnauthorized))
				return
			}

			ctx := context.WithValue(r.Context(), consoleSessionContextKey{}, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ConsoleSessionFromContext(ctx context.Context) (service.ConsoleSession, bool) {
	session, ok := ctx.Value(consoleSessionContextKey{}).(service.ConsoleSession)
	return session, ok
}

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
