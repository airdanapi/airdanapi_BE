package handler

import (
	"context"
	"net/http"

	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"
)

type ConsoleFeeRepository interface {
	Summary(ctx context.Context) (repository.GatewayFeeSummary, error)
	List(ctx context.Context, filter repository.GatewayFeeFilter) ([]domain.GatewayFee, error)
}

type ConsoleFeesHandler struct {
	fees ConsoleFeeRepository
}

func NewConsoleFeesHandler(fees ConsoleFeeRepository) ConsoleFeesHandler {
	return ConsoleFeesHandler{fees: fees}
}

func (h ConsoleFeesHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if h.fees == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "gateway fee repository is unavailable")
		return
	}

	summary, err := h.fees.Summary(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "gateway fee summary could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, summary)
}

func (h ConsoleFeesHandler) Pending(w http.ResponseWriter, r *http.Request) {
	if h.fees == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "gateway fee repository is unavailable")
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "PENDING"
	}
	fees, err := h.fees.List(r.Context(), repository.GatewayFeeFilter{
		Status:  status,
		Page:    intQuery(r, "page", 1),
		PerPage: intQuery(r, "per_page", 20),
	})
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "gateway fees could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"items":    fees,
		"page":     intQuery(r, "page", 1),
		"per_page": intQuery(r, "per_page", 20),
	})
}
