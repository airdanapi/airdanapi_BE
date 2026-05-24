package handler

import (
	"context"
	"net/http"

	"airdanapi-be/internal/repository"
)

type DashboardLogRepository interface {
	DashboardSummary(ctx context.Context) (repository.DashboardSummary, error)
	Throughput(ctx context.Context) ([]repository.ThroughputPoint, error)
}

type DashboardFeeRepository interface {
	Summary(ctx context.Context) (repository.GatewayFeeSummary, error)
}

type ConsoleDashboardHandler struct {
	logs DashboardLogRepository
	fees DashboardFeeRepository
}

func NewConsoleDashboardHandler(logs DashboardLogRepository, fees DashboardFeeRepository) ConsoleDashboardHandler {
	return ConsoleDashboardHandler{logs: logs, fees: fees}
}

func (h ConsoleDashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if h.logs == nil || h.fees == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "dashboard repositories are unavailable")
		return
	}

	logSummary, err := h.logs.DashboardSummary(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "dashboard request summary could not be queried")
		return
	}
	feeSummary, err := h.fees.Summary(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "dashboard fee summary could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{
		"total_requests":     logSummary.TotalRequests,
		"error_rate":         logSummary.ErrorRate,
		"average_latency_ms": logSummary.AverageLatency,
		"fee_revenue":        feeSummary.RevenueTotal,
		"pending_fees":       feeSummary.PendingCount,
		"failed_fees":        feeSummary.FailedCount,
	})
}

func (h ConsoleDashboardHandler) Throughput(w http.ResponseWriter, r *http.Request) {
	if h.logs == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "request log repository is unavailable")
		return
	}

	points, err := h.logs.Throughput(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "dashboard throughput could not be queried")
		return
	}

	WriteSuccess(w, r, http.StatusOK, map[string]interface{}{"items": points})
}
