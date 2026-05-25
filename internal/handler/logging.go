package handler

import (
	"net/http"
	"strconv"
	"time"

	"airdanapi-be/internal/repository"
)

type LoggingHandler struct {
	repo repository.RequestLogRepository
}

func NewLoggingHandler(repo repository.RequestLogRepository) LoggingHandler {
	return LoggingHandler{repo: repo}
}

func (h LoggingHandler) List(w http.ResponseWriter, r *http.Request) {
	if !hasRequiredScope(w, r, "admin:read") {
		return
	}
	if h.repo == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "request log repository is unavailable")
		return
	}

	filter, ok := requestLogFilterFromQuery(w, r)
	if !ok {
		return
	}

	logs, err := h.repo.List(r.Context(), filter)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "request logs could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"items":    requestLogsToResponse(logs),
		"page":     filter.Page,
		"per_page": filter.PerPage,
	})
}

func requestLogFilterFromQuery(w http.ResponseWriter, r *http.Request) (repository.RequestLogFilter, bool) {
	query := r.URL.Query()
	filter := repository.RequestLogFilter{
		UserID:    query.Get("user_id"),
		RequestID: query.Get("request_id"),
		TargetApp: query.Get("target_app"),
		Page:      intQuery(r, "page", 1),
		PerPage:   intQuery(r, "per_page", 20),
	}

	if status := query.Get("status_code"); status != "" {
		parsed, err := strconv.Atoi(status)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "status_code must be an integer")
			return repository.RequestLogFilter{}, false
		}
		filter.StatusCode = &parsed
	}

	if from := query.Get("from"); from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "from must use RFC3339 datetime")
			return repository.RequestLogFilter{}, false
		}
		filter.From = &parsed
	}

	if to := query.Get("to"); to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "to must use RFC3339 datetime")
			return repository.RequestLogFilter{}, false
		}
		filter.To = &parsed
	}

	return filter, true
}
