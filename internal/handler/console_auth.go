package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/service"
)

type ConsoleAuthHandler struct {
	operators service.ConsoleOperatorRepository
	sessions  *service.ConsoleSessionStore
}

func NewConsoleAuthHandler(operators service.ConsoleOperatorRepository, sessions *service.ConsoleSessionStore) ConsoleAuthHandler {
	return ConsoleAuthHandler{operators: operators, sessions: sessions}
}

type consoleLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type consoleOperatorResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (h ConsoleAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.operators == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "operator repository is unavailable")
		return
	}

	var body consoleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body must be valid json")
		return
	}

	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" || body.Password == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "email and password are required")
		return
	}

	session, err := h.sessions.Login(r.Context(), h.operators, body.Email, body.Password)
	if err != nil {
		if errors.Is(err, service.ErrConsoleUnauthorized) {
			WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "email or password is invalid")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "console login failed")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"token":      session.Token,
		"expires_at": session.ExpiresAt,
		"operator":   operatorResponse(session.Operator),
	})
}

func (h ConsoleAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	token = strings.TrimSpace(token)
	if token != "" {
		h.sessions.Logout(token)
	}

	WriteSuccess(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h ConsoleAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.ConsoleSessionFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "console session is missing")
		return
	}
	WriteSuccess(w, r, http.StatusOK, operatorResponse(session.Operator))
}

func operatorResponse(operator domain.Operator) consoleOperatorResponse {
	return consoleOperatorResponse{
		ID:    operator.ID,
		Email: operator.Email,
		Name:  operator.Name,
		Role:  operator.Role,
	}
}
