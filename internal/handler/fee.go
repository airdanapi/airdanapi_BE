package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/service"

	"github.com/go-chi/chi/v5"
)

type FeeHandler struct {
	repo    repository.GatewayFeeRepository
	service service.FeeService
}

func NewFeeHandler(repo repository.GatewayFeeRepository, feeService service.FeeService) FeeHandler {
	return FeeHandler{repo: repo, service: feeService}
}

func (h FeeHandler) List(w http.ResponseWriter, r *http.Request) {
	if !hasRequiredScope(w, r, "admin:read") {
		return
	}
	if h.repo == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "gateway fee repository is unavailable")
		return
	}

	fees, err := h.repo.List(r.Context(), repository.GatewayFeeFilter{
		Status:    r.URL.Query().Get("status"),
		UserID:    r.URL.Query().Get("user_id"),
		RequestID: r.URL.Query().Get("request_id"),
		Page:      intQuery(r, "page", 1),
		PerPage:   intQuery(r, "per_page", 20),
	})
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "gateway fees could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"items":    gatewayFeesToResponse(fees),
		"page":     intQuery(r, "page", 1),
		"per_page": intQuery(r, "per_page", 20),
	})
}

func (h FeeHandler) Retry(w http.ResponseWriter, r *http.Request) {
	if !hasRequiredScope(w, r, "admin:write") {
		return
	}
	if h.repo == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "gateway fee repository is unavailable")
		return
	}

	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "id")), 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "fee id must be a positive integer")
		return
	}

	fee, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "gateway fee was not found")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "gateway fee lookup failed")
		return
	}

	updated, err := h.service.Retry(r.Context(), fee)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "gateway fee retry failed")
		return
	}

	WriteSuccess(w, r, http.StatusOK, gatewayFeeToResponse(updated))
}

func hasRequiredScope(w http.ResponseWriter, r *http.Request, requiredScope string) bool {
	if _, ok := middleware.ConsoleSessionFromContext(r.Context()); ok {
		return true
	}

	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "jwt principal is missing")
		return false
	}
	if !service.HasScope(principal, requiredScope) {
		WriteError(w, r, http.StatusForbidden, "AUTH_SCOPE_DENIED", "scope is not allowed for this endpoint")
		return false
	}
	return true
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
