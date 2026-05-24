package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/middleware"
	"airdanapi-be/internal/repository"
	"airdanapi-be/internal/response"
	"airdanapi-be/internal/service"
	"airdanapi-be/internal/store"

	"github.com/go-chi/chi/v5"
)

type RoutingHandler struct {
	routes     repository.RouteRepository
	feeService *service.FeeService
	protection *service.ProtectionService
	client     *http.Client
}

func NewRoutingHandler(routes repository.RouteRepository, feeService *service.FeeService, protection *service.ProtectionService) RoutingHandler {
	return RoutingHandler{
		routes:     routes,
		feeService: feeService,
		protection: protection,
		client:     http.DefaultClient,
	}
}

type envelopeRoutingRequest struct {
	TargetService string          `json:"target_service"`
	TargetFeature string          `json:"target_feature"`
	Method        string          `json:"method"`
	Payload       json.RawMessage `json:"payload"`
}

func (h RoutingHandler) Transparent(w http.ResponseWriter, r *http.Request) {
	serviceName := strings.TrimSpace(chi.URLParam(r, "service"))
	featureName := strings.TrimSpace(chi.URLParam(r, "feature"))
	method := strings.ToUpper(r.Method)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body could not be read")
		return
	}

	route, ok := h.resolveAndAuthorize(w, r, serviceName, featureName, method)
	if !ok {
		return
	}

	downstreamURL, err := withQuery(route.DownstreamURL, r.URL.RawQuery)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "downstream url is invalid")
		return
	}

	h.forward(w, r, route, method, downstreamURL, body, r.Header.Get("Content-Type"), false)
}

func (h RoutingHandler) Envelope(w http.ResponseWriter, r *http.Request) {
	var body envelopeRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "request body must be valid json")
		return
	}

	body.TargetService = strings.TrimSpace(body.TargetService)
	body.TargetFeature = strings.TrimSpace(body.TargetFeature)
	body.Method = strings.ToUpper(strings.TrimSpace(body.Method))
	if body.TargetService == "" || body.TargetFeature == "" || body.Method == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "target_service, target_feature, and method are required")
		return
	}

	payload := body.Payload
	if len(bytes.TrimSpace(payload)) == 0 || string(bytes.TrimSpace(payload)) == "null" {
		payload = json.RawMessage(`{}`)
	}

	route, ok := h.resolveAndAuthorize(w, r, body.TargetService, body.TargetFeature, body.Method)
	if !ok {
		return
	}

	h.forward(w, r, route, body.Method, route.DownstreamURL, payload, "application/json", true)
}

func (h RoutingHandler) resolveAndAuthorize(w http.ResponseWriter, r *http.Request, serviceName, featureName, method string) (domain.Route, bool) {
	if h.routes == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "route registry is unavailable")
		return domain.Route{}, false
	}
	if serviceName == "" || featureName == "" || method == "" {
		WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "service, feature, and method are required")
		return domain.Route{}, false
	}
	if exceedsHopLimit(r.Header.Get("X-Hop-Count")) {
		WriteError(w, r, http.StatusLoopDetected, "LOOP_DETECTED", "hop count exceeded")
		return domain.Route{}, false
	}

	route, err := h.routes.FindActiveByServiceFeatureMethod(r.Context(), serviceName, featureName, method)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "route was not found")
			return domain.Route{}, false
		}

		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "route lookup failed")
		return domain.Route{}, false
	}

	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "jwt principal is missing")
		return domain.Route{}, false
	}
	if route.RequiredScope != nil && !service.HasScope(principal, *route.RequiredScope) {
		WriteError(w, r, http.StatusForbidden, "AUTH_SCOPE_DENIED", "scope is not allowed for this route")
		return domain.Route{}, false
	}

	return route, true
}

func (h RoutingHandler) forward(w http.ResponseWriter, r *http.Request, route domain.Route, method, downstreamURL string, body []byte, contentType string, envelopeMode bool) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "jwt principal is missing")
		return
	}

	now := time.Now()
	protectionDecision := h.protection.Before(route, principal, method, r.URL.Path, body, r.Header, now)
	if !protectionDecision.Allowed {
		if protectionDecision.RetryAfter > 0 {
			w.Header().Set("Retry-After", service.RetryAfterSeconds(protectionDecision.RetryAfter))
		}
		WriteError(w, r, protectionDecision.Status, protectionDecision.Code, protectionDecision.Message)
		return
	}
	if protectionDecision.CachedResponse != nil {
		writeCachedResponse(w, protectionDecision.CachedResponse)
		return
	}

	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, downstreamURL, bytes.NewReader(body))
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "downstream request could not be created")
		return
	}

	forwardHeaders(r, req, contentType)

	resp, err := h.client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || isTimeout(err) {
			h.protection.After(route, protectionDecision, store.CachedResponse{StatusCode: http.StatusBadGateway}, true, false, time.Now())
			WriteError(w, r, http.StatusBadGateway, "UPSTREAM_TIMEOUT", "downstream request timed out")
			return
		}

		h.protection.After(route, protectionDecision, store.CachedResponse{StatusCode: http.StatusBadGateway}, false, true, time.Now())
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", "downstream request failed")
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.protection.After(route, protectionDecision, store.CachedResponse{StatusCode: http.StatusBadGateway}, false, true, time.Now())
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", "downstream response could not be read")
		return
	}

	if envelopeMode && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		h.protection.After(route, protectionDecision, store.CachedResponse{StatusCode: http.StatusBadGateway}, false, resp.StatusCode >= 500, time.Now())
		WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", "downstream returned unsuccessful status")
		return
	}

	var finalResponse store.CachedResponse
	if route.Transactional && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		finalResponse = h.transactionalResponse(r, route, resp, responseBody)
		if finalResponse.StatusCode == 0 {
			h.protection.After(route, protectionDecision, store.CachedResponse{StatusCode: http.StatusBadGateway}, false, true, time.Now())
			WriteError(w, r, http.StatusBadGateway, "UPSTREAM_FAILED", "transaction amount could not be read from downstream response")
			return
		}
		h.protection.After(route, protectionDecision, finalResponse, false, false, time.Now())
		writeCachedResponse(w, &finalResponse)
		return
	}

	if envelopeMode {
		body := response.MustJSON(response.Success(r.Context(), downstreamData(responseBody)))
		finalResponse = store.CachedResponse{
			StatusCode: resp.StatusCode,
			Headers:    http.Header{"Content-Type": []string{"application/json"}},
			Body:       body,
		}
		h.protection.After(route, protectionDecision, finalResponse, false, false, time.Now())
		writeCachedResponse(w, &finalResponse)
		return
	}

	headers := http.Header{}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	finalResponse = store.CachedResponse{StatusCode: resp.StatusCode, Headers: headers, Body: responseBody}
	h.protection.After(route, protectionDecision, finalResponse, false, false, time.Now())
	writeCachedResponse(w, &finalResponse)
}

func downstreamData(body []byte) any {
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return string(body)
	}
	return data
}

func (h RoutingHandler) transactionalResponse(r *http.Request, route domain.Route, resp *http.Response, body []byte) store.CachedResponse {
	if h.feeService == nil {
		return store.CachedResponse{}
	}

	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		return store.CachedResponse{}
	}

	fee, err := h.feeService.Charge(r.Context(), principal, middleware.RequestIDFromContext(r.Context()), route.ServiceName, body)
	if err != nil {
		return store.CachedResponse{}
	}

	responseBody := response.MustJSON(response.SuccessWithFee(r.Context(), downstreamData(body), response.FeeBody{
		Amount: fee.Amount,
		Status: fee.Status,
	}))
	return store.CachedResponse{
		StatusCode: resp.StatusCode,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       responseBody,
	}
}

func writeCachedResponse(w http.ResponseWriter, cached *store.CachedResponse) {
	for key, values := range cached.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(cached.StatusCode)
	_, _ = w.Write(cached.Body)
}

func forwardHeaders(source *http.Request, target *http.Request, contentType string) {
	for _, header := range []string{
		"Authorization",
		"X-Request-Id",
		"X-Idempotency-Key",
		"X-User-Id",
		"X-Parent-Request-Id",
	} {
		if value := source.Header.Get(header); value != "" {
			target.Header.Set(header, value)
		}
	}

	if contentType != "" {
		target.Header.Set("Content-Type", contentType)
	}

	requestID := middleware.RequestIDFromContext(source.Context())
	if target.Header.Get("X-Parent-Request-Id") == "" && requestID != "" {
		target.Header.Set("X-Parent-Request-Id", requestID)
	}
	if principal, ok := middleware.PrincipalFromContext(source.Context()); ok && principal.UserID != "" {
		target.Header.Set("X-User-Id", principal.UserID)
	}

	target.Header.Set("X-Hop-Count", strconv.Itoa(currentHopCount(source.Header.Get("X-Hop-Count"))+1))
}

func withQuery(rawURL string, rawQuery string) (string, error) {
	if rawQuery == "" {
		return rawURL, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if parsed.RawQuery == "" {
		parsed.RawQuery = rawQuery
	} else {
		parsed.RawQuery = parsed.RawQuery + "&" + rawQuery
	}
	return parsed.String(), nil
}

func exceedsHopLimit(value string) bool {
	return currentHopCount(value) >= 3
}

func currentHopCount(value string) int {
	count, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
