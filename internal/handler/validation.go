package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"
)

type ValidationHandler struct {
	routes repository.RouteRepository
}

func NewValidationHandler(routes repository.RouteRepository) ValidationHandler {
	return ValidationHandler{routes: routes}
}

type validateRequestBody struct {
	Service string `json:"service"`
	Feature string `json:"feature"`
	Method  string `json:"method"`
}

type validateRequestResponse struct {
	Valid  bool     `json:"valid"`
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
	Scopes []string `json:"scopes"`
	Exp    int64    `json:"exp"`
}

func (h ValidationHandler) ValidateRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "jwt principal is missing")
		return
	}

	var body validateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body must be valid json")
		return
	}

	body.Service = strings.TrimSpace(body.Service)
	body.Feature = strings.TrimSpace(body.Feature)
	body.Method = strings.ToUpper(strings.TrimSpace(body.Method))
	if body.Service == "" || body.Feature == "" || body.Method == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "service, feature, and method are required")
		return
	}
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route registry is unavailable")
		return
	}

	route, err := h.routes.FindActiveByServiceFeatureMethod(r.Context(), body.Service, body.Feature, body.Method)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "route was not found")
			return
		}

		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "route lookup failed")
		return
	}

	if route.RequiredScope != nil && !service.HasScope(principal, *route.RequiredScope) {
		WriteError(w, r, http.StatusForbidden, "AUTH_SCOPE_DENIED", "scope is not allowed for this route")
		return
	}

	WriteSuccess(w, r, http.StatusOK, validateRequestResponse{
		Valid:  true,
		UserID: principal.UserID,
		Roles:  principal.Roles,
		Scopes: principal.Scopes,
		Exp:    principal.ExpiresAt,
	})
}
