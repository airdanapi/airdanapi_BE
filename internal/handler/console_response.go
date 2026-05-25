package handler

import (
	"time"

	"airdanapi-be/internal/domain"
)

type requestLogResponse struct {
	ID              int64     `json:"id"`
	RequestID       string    `json:"request_id"`
	ParentRequestID *string   `json:"parent_request_id"`
	UserID          *string   `json:"user_id"`
	SourceApp       *string   `json:"source_app"`
	TargetApp       string    `json:"target_app"`
	Endpoint        string    `json:"endpoint"`
	Method          string    `json:"method"`
	StatusCode      *int      `json:"status_code"`
	LatencyMS       *int      `json:"latency_ms"`
	IPAddress       string    `json:"ip_address"`
	RequestHash     *string   `json:"request_hash"`
	ResponseHash    *string   `json:"response_hash"`
	Lifecycle       string    `json:"lifecycle"`
	ErrorMessage    *string   `json:"error_message"`
	CreatedAt       time.Time `json:"created_at"`
}

type gatewayFeeResponse struct {
	ID                int64      `json:"id"`
	RequestID         string     `json:"request_id"`
	UserID            string     `json:"user_id"`
	SourceApp         string     `json:"source_app"`
	TransactionAmount int64      `json:"transaction_amount"`
	FeeAmount         int64      `json:"fee_amount"`
	FeeRate           float64    `json:"fee_rate"`
	Status            string     `json:"status"`
	RetryCount        int        `json:"retry_count"`
	MaxRetries        int        `json:"max_retries"`
	NextRetryAt       *time.Time `json:"next_retry_at"`
	SmartBankRef      *string    `json:"smartbank_ref"`
	ErrorMessage      *string    `json:"error_message"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func requestLogsToResponse(logs []domain.RequestLog) []requestLogResponse {
	items := make([]requestLogResponse, 0, len(logs))
	for _, log := range logs {
		items = append(items, requestLogToResponse(log))
	}
	return items
}

func requestLogToResponse(log domain.RequestLog) requestLogResponse {
	return requestLogResponse{
		ID:              log.ID,
		RequestID:       log.RequestID,
		ParentRequestID: log.ParentRequestID,
		UserID:          log.UserID,
		SourceApp:       log.SourceApp,
		TargetApp:       log.TargetApp,
		Endpoint:        log.Endpoint,
		Method:          log.Method,
		StatusCode:      log.StatusCode,
		LatencyMS:       log.LatencyMS,
		IPAddress:       log.IPAddress,
		RequestHash:     log.RequestHash,
		ResponseHash:    log.ResponseHash,
		Lifecycle:       log.Lifecycle,
		ErrorMessage:    log.ErrorMessage,
		CreatedAt:       log.CreatedAt,
	}
}

func gatewayFeesToResponse(fees []domain.GatewayFee) []gatewayFeeResponse {
	items := make([]gatewayFeeResponse, 0, len(fees))
	for _, fee := range fees {
		items = append(items, gatewayFeeToResponse(fee))
	}
	return items
}

func gatewayFeeToResponse(fee domain.GatewayFee) gatewayFeeResponse {
	return gatewayFeeResponse{
		ID:                fee.ID,
		RequestID:         fee.RequestID,
		UserID:            fee.UserID,
		SourceApp:         fee.SourceApp,
		TransactionAmount: fee.TransactionAmount,
		FeeAmount:         fee.FeeAmount,
		FeeRate:           fee.FeeRate,
		Status:            fee.Status,
		RetryCount:        fee.RetryCount,
		MaxRetries:        fee.MaxRetries,
		NextRetryAt:       fee.NextRetryAt,
		SmartBankRef:      fee.SmartBankRef,
		ErrorMessage:      fee.ErrorMessage,
		CreatedAt:         fee.CreatedAt,
		UpdatedAt:         fee.UpdatedAt,
	}
}
