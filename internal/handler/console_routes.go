package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"

	"github.com/go-chi/chi/v5"
)

type ConsoleRouteRepository interface {
	ListAll(ctx context.Context) ([]domain.Route, error)
	Create(ctx context.Context, input repository.RouteInput) (domain.Route, error)
	Update(ctx context.Context, id int64, input repository.RouteInput) (domain.Route, error)
	Toggle(ctx context.Context, id int64, active bool) (domain.Route, error)
}

type ConsoleRoutesHandler struct {
	routes ConsoleRouteRepository
}

func NewConsoleRoutesHandler(routes ConsoleRouteRepository) ConsoleRoutesHandler {
	return ConsoleRoutesHandler{routes: routes}
}

type consoleRouteRequest struct {
	ServiceName   string  `json:"service_name"`
	FeatureName   string  `json:"feature_name"`
	Method        string  `json:"method"`
	DownstreamURL string  `json:"downstream_url"`
	Transactional bool    `json:"transactional"`
	RouteClass    string  `json:"route_class"`
	TimeoutMS     int     `json:"timeout_ms"`
	RetryCount    int     `json:"retry_count"`
	RequiredScope *string `json:"required_scope"`
	IsActive      *bool   `json:"is_active"`
	Description   *string `json:"description"`
}

type routeResponse struct {
	ID            int64   `json:"id"`
	ServiceName   string  `json:"service_name"`
	FeatureName   string  `json:"feature_name"`
	Method        string  `json:"method"`
	DownstreamURL string  `json:"downstream_url"`
	Transactional bool    `json:"transactional"`
	RouteClass    string  `json:"route_class"`
	TimeoutMS     int     `json:"timeout_ms"`
	RetryCount    int     `json:"retry_count"`
	RequiredScope *string `json:"required_scope"`
	IsActive      bool    `json:"is_active"`
	Description   *string `json:"description"`
}

func (h ConsoleRoutesHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route repository is unavailable")
		return
	}

	routes, err := h.routes.ListAll(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "routes could not be queried")
		return
	}

	items := make([]routeResponse, 0, len(routes))
	for _, route := range routes {
		items = append(items, routeToResponse(route))
	}
	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{"items": items})
}

func (h ConsoleRoutesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route repository is unavailable")
		return
	}

	input, ok := routeInputFromRequest(w, r)
	if !ok {
		return
	}

	route, err := h.routes.Create(r.Context(), input)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "route could not be created")
		return
	}

	WriteSuccess(w, r, http.StatusCreated, routeToResponse(route))
}

func (h ConsoleRoutesHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route repository is unavailable")
		return
	}

	id, ok := routeIDFromRequest(w, r)
	if !ok {
		return
	}
	input, ok := routeInputFromRequest(w, r)
	if !ok {
		return
	}

	route, err := h.routes.Update(r.Context(), id, input)
	if err != nil {
		writeRouteMutationError(w, r, err, "route could not be updated")
		return
	}

	WriteSuccess(w, r, http.StatusOK, routeToResponse(route))
}

func (h ConsoleRoutesHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route repository is unavailable")
		return
	}

	id, ok := routeIDFromRequest(w, r)
	if !ok {
		return
	}

	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body must be valid json")
		return
	}

	route, err := h.routes.Toggle(r.Context(), id, body.IsActive)
	if err != nil {
		writeRouteMutationError(w, r, err, "route could not be toggled")
		return
	}

	WriteSuccess(w, r, http.StatusOK, routeToResponse(route))
}

func routeInputFromRequest(w http.ResponseWriter, r *http.Request) (repository.RouteInput, bool) {
	var body consoleRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body must be valid json")
		return repository.RouteInput{}, false
	}

	body.ServiceName = strings.TrimSpace(strings.ToLower(body.ServiceName))
	body.FeatureName = strings.TrimSpace(strings.ToLower(body.FeatureName))
	body.Method = strings.ToUpper(strings.TrimSpace(body.Method))
	body.DownstreamURL = strings.TrimSpace(body.DownstreamURL)
	body.RouteClass = strings.TrimSpace(strings.ToLower(body.RouteClass))

	if body.ServiceName == "" || body.FeatureName == "" || body.Method == "" || body.DownstreamURL == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "service_name, feature_name, method, and downstream_url are required")
		return repository.RouteInput{}, false
	}
	if body.RouteClass == "" {
		if body.Transactional {
			body.RouteClass = "transactional"
		} else {
			body.RouteClass = "read"
		}
	}
	if body.RouteClass != "read" && body.RouteClass != "transactional" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "route_class must be read or transactional")
		return repository.RouteInput{}, false
	}
	if body.TimeoutMS <= 0 {
		body.TimeoutMS = 5000
	}
	if body.RetryCount < 0 {
		body.RetryCount = 0
	}
	active := true
	if body.IsActive != nil {
		active = *body.IsActive
	}

	body.RequiredScope = cleanStringPointer(body.RequiredScope)
	body.Description = cleanStringPointer(body.Description)

	return repository.RouteInput{
		ServiceName:   body.ServiceName,
		FeatureName:   body.FeatureName,
		Method:        body.Method,
		DownstreamURL: body.DownstreamURL,
		Transactional: body.Transactional,
		RouteClass:    body.RouteClass,
		TimeoutMS:     body.TimeoutMS,
		RetryCount:    body.RetryCount,
		RequiredScope: body.RequiredScope,
		IsActive:      active,
		Description:   body.Description,
	}, true
}

func routeIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "id")), 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "route id must be a positive integer")
		return 0, false
	}
	return id, true
}

func routeToResponse(route domain.Route) routeResponse {
	return routeResponse{
		ID:            route.ID,
		ServiceName:   route.ServiceName,
		FeatureName:   route.FeatureName,
		Method:        route.Method,
		DownstreamURL: route.DownstreamURL,
		Transactional: route.Transactional,
		RouteClass:    route.RouteClass,
		TimeoutMS:     route.TimeoutMS,
		RetryCount:    route.RetryCount,
		RequiredScope: route.RequiredScope,
		IsActive:      route.IsActive,
		Description:   route.Description,
	}
}

func writeRouteMutationError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	if errors.Is(err, repository.ErrNotFound) {
		WriteError(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "route was not found")
		return
	}
	WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", fallback)
}

func cleanStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
