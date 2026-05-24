package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"

	"github.com/rs/zerolog/log"
)

type FeeService struct {
	cfg       config.FeeConfig
	repo      repository.GatewayFeeRepository
	smartbank SmartBankClient
}

type FeeResult struct {
	Amount int64
	Status string
	Record domain.GatewayFee
}

var ErrTransactionAmountMissing = errors.New("transaction_amount missing from downstream response")

func NewFeeService(cfg config.FeeConfig, repo repository.GatewayFeeRepository, smartbank SmartBankClient) FeeService {
	return FeeService{cfg: cfg, repo: repo, smartbank: smartbank}
}

func CalculateFee(amount int64, rate float64) int64 {
	return int64(math.Floor(float64(amount)*rate + 0.5))
}

func ExtractTransactionAmount(body []byte) (int64, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, ErrTransactionAmountMissing
	}

	if amount, ok := numericField(payload["transaction_amount"]); ok {
		return amount, nil
	}
	if data, ok := payload["data"].(map[string]interface{}); ok {
		if amount, ok := numericField(data["transaction_amount"]); ok {
			return amount, nil
		}
	}

	return 0, ErrTransactionAmountMissing
}

func (s FeeService) Charge(ctx context.Context, principal Principal, requestID string, sourceApp string, downstreamBody []byte) (FeeResult, error) {
	amount, err := ExtractTransactionAmount(downstreamBody)
	if err != nil {
		return FeeResult{}, err
	}

	feeAmount := CalculateFee(amount, s.cfg.Rate)
	fee := domain.GatewayFee{
		RequestID:         requestID,
		UserID:            principal.UserID,
		SourceApp:         sourceAppOrDefault(sourceApp, principal.SourceApp),
		TransactionAmount: amount,
		FeeAmount:         feeAmount,
		FeeRate:           s.cfg.Rate,
		Status:            "SUCCESS",
		RetryCount:        0,
		MaxRetries:        5,
	}

	ref, err := s.callSmartBank(ctx, fee)
	if err != nil {
		fee.Status = "PENDING"
		next := time.Now().Add(retryDelay(0))
		fee.NextRetryAt = &next
		message := err.Error()
		fee.ErrorMessage = &message
		if s.repo != nil {
			if id, createErr := s.repo.Create(ctx, fee); createErr == nil {
				fee.ID = id
			} else {
				return FeeResult{}, createErr
			}
		}
		return FeeResult{Amount: feeAmount, Status: "deferred", Record: fee}, nil
	}

	fee.SmartBankRef = &ref
	if s.repo != nil {
		id, err := s.repo.Create(ctx, fee)
		if err != nil {
			return FeeResult{}, err
		}
		fee.ID = id
	}
	return FeeResult{Amount: feeAmount, Status: "success", Record: fee}, nil
}

func (s FeeService) Retry(ctx context.Context, fee domain.GatewayFee) (domain.GatewayFee, error) {
	ref, err := s.callSmartBank(ctx, fee)
	if err != nil {
		fee.RetryCount++
		message := err.Error()
		fee.ErrorMessage = &message
		if fee.RetryCount >= fee.MaxRetries {
			fee.Status = "FAILED"
			fee.NextRetryAt = nil
		} else {
			fee.Status = "PENDING"
			next := time.Now().Add(retryDelay(fee.RetryCount))
			fee.NextRetryAt = &next
		}
		if s.repo != nil {
			return fee, s.repo.UpdateRetryState(ctx, fee)
		}
		return fee, nil
	}

	fee.Status = "SUCCESS"
	fee.NextRetryAt = nil
	fee.SmartBankRef = &ref
	fee.ErrorMessage = nil
	if s.repo != nil {
		return fee, s.repo.UpdateRetryState(ctx, fee)
	}
	return fee, nil
}

func (s FeeService) StartRetryWorker(ctx context.Context) {
	if s.repo == nil {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.retryDue(ctx)
			}
		}
	}()
}

func (s FeeService) retryDue(ctx context.Context) {
	fees, err := s.repo.ListDueRetries(ctx, time.Now(), 20)
	if err != nil {
		log.Warn().Err(err).Msg("gateway fee retry query failed")
		return
	}

	for _, fee := range fees {
		if _, err := s.Retry(ctx, fee); err != nil {
			log.Warn().Err(err).Int64("fee_id", fee.ID).Msg("gateway fee retry failed")
		}
	}
}

func (s FeeService) callSmartBank(ctx context.Context, fee domain.GatewayFee) (string, error) {
	if s.smartbank == nil {
		return "", ErrSmartBankFailed
	}

	response, err := s.smartbank.ChargeFee(ctx, SmartBankChargeRequest{
		FromUser: fee.UserID,
		ToUser:   s.cfg.RevenueUser,
		Amount:   fee.FeeAmount,
		Metadata: map[string]interface{}{
			"kind":            "gateway_fee",
			"request_id":      fee.RequestID,
			"original_amount": fee.TransactionAmount,
		},
	}, "fee-"+fee.RequestID)
	if err != nil {
		return "", err
	}
	if response.Reference == "" {
		return "smartbank-fee-" + fee.RequestID, nil
	}
	return response.Reference, nil
}

func retryDelay(retryCount int) time.Duration {
	delays := []time.Duration{
		30 * time.Second,
		2 * time.Minute,
		8 * time.Minute,
		32 * time.Minute,
		2 * time.Hour,
	}
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount >= len(delays) {
		return delays[len(delays)-1]
	}
	return delays[retryCount]
}

func numericField(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func sourceAppOrDefault(sourceApp string, fallback string) string {
	if sourceApp != "" {
		return sourceApp
	}
	if fallback != "" {
		return fallback
	}
	return "unknown"
}
