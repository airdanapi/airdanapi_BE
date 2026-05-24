package middleware

import (
	"context"
	"errors"
	"net/http"

	"airdanapi-be/internal/response"
	"airdanapi-be/internal/service"
)

type authContextKey struct{}

func AuthRequired(authService *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := authService.ValidateBearer(r.Context(), r.Header.Get("Authorization"))
			if err != nil {
				var authErr service.AuthError
				if errors.As(err, &authErr) {
					response.WriteJSON(w, authErr.Status, response.Error(r.Context(), authErr.Code, authErr.Message, authErr.Status))
					return
				}

				response.WriteJSON(w, http.StatusUnauthorized, response.Error(r.Context(), "AUTH_INVALID_TOKEN", "jwt token is invalid", http.StatusUnauthorized))
				return
			}

			if principal.UserID != "" {
				r.Header.Set("X-User-Id", principal.UserID)
			}
			if principal.SourceApp != "" {
				r.Header.Set("X-Source-App", principal.SourceApp)
			}

			ctx := context.WithValue(r.Context(), authContextKey{}, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func PrincipalFromContext(ctx context.Context) (service.Principal, bool) {
	principal, ok := ctx.Value(authContextKey{}).(service.Principal)
	return principal, ok
}
