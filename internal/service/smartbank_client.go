package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"airdanapi-be/internal/config"
)

type SmartBankClient interface {
	ChargeFee(ctx context.Context, request SmartBankChargeRequest, idempotencyKey string) (SmartBankChargeResponse, error)
}

type SmartBankChargeRequest struct {
	FromUser string                 `json:"from_user"`
	ToUser   string                 `json:"to_user"`
	Amount   int64                  `json:"amount"`
	Metadata map[string]interface{} `json:"metadata"`
}

type SmartBankChargeResponse struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

var ErrSmartBankFailed = errors.New("smartbank fee charge failed")
var ErrSmartBankTimeout = errors.New("smartbank fee charge timed out")

type ConfiguredSmartBankClient struct {
	cfg    config.SmartBankConfig
	client *http.Client
}

func NewSmartBankClient(cfg config.SmartBankConfig) ConfiguredSmartBankClient {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return ConfiguredSmartBankClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c ConfiguredSmartBankClient) ChargeFee(ctx context.Context, request SmartBankChargeRequest, idempotencyKey string) (SmartBankChargeResponse, error) {
	switch strings.ToLower(strings.TrimSpace(c.cfg.Mode)) {
	case "", "mock_success":
		return SmartBankChargeResponse{Reference: "mock-" + idempotencyKey, Status: "SUCCESS"}, nil
	case "mock_failure":
		return SmartBankChargeResponse{}, ErrSmartBankFailed
	case "mock_timeout":
		return SmartBankChargeResponse{}, ErrSmartBankTimeout
	case "http":
		return c.chargeHTTP(ctx, request, idempotencyKey)
	default:
		return SmartBankChargeResponse{}, ErrSmartBankFailed
	}
}

func (c ConfiguredSmartBankClient) chargeHTTP(ctx context.Context, request SmartBankChargeRequest, idempotencyKey string) (SmartBankChargeResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return SmartBankChargeResponse{}, err
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/smartbank/pembayaran_transaksi"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return SmartBankChargeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Idempotency-Key", idempotencyKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return SmartBankChargeResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SmartBankChargeResponse{}, ErrSmartBankFailed
	}

	var response SmartBankChargeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return SmartBankChargeResponse{}, err
	}
	if response.Reference == "" {
		response.Reference = "smartbank-" + idempotencyKey
	}
	if response.Status == "" {
		response.Status = "SUCCESS"
	}
	return response, nil
}
