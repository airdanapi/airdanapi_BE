package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"airdanapi-be/internal/config"
	"airdanapi-be/internal/domain"
	"airdanapi-be/internal/repository"
)

type fakeSmartBank struct {
	err error
}

func (f fakeSmartBank) ChargeFee(ctx context.Context, request SmartBankChargeRequest, idempotencyKey string) (SmartBankChargeResponse, error) {
	if f.err != nil {
		return SmartBankChargeResponse{}, f.err
	}
	return SmartBankChargeResponse{Reference: "ref-123", Status: "SUCCESS"}, nil
}

type fakeFeeRepository struct {
	fees []domain.GatewayFee
}

func (f *fakeFeeRepository) Create(ctx context.Context, fee domain.GatewayFee) (int64, error) {
	fee.ID = int64(len(f.fees) + 1)
	f.fees = append(f.fees, fee)
	return fee.ID, nil
}

func (f *fakeFeeRepository) FindByRequestID(ctx context.Context, requestID string) (domain.GatewayFee, error) {
	for _, fee := range f.fees {
		if fee.RequestID == requestID {
			return fee, nil
		}
	}
	return domain.GatewayFee{}, repository.ErrNotFound
}

func (f *fakeFeeRepository) FindByID(ctx context.Context, id int64) (domain.GatewayFee, error) {
	for _, fee := range f.fees {
		if fee.ID == id {
			return fee, nil
		}
	}
	return domain.GatewayFee{}, repository.ErrNotFound
}

func (f *fakeFeeRepository) List(ctx context.Context, filter repository.GatewayFeeFilter) ([]domain.GatewayFee, error) {
	return f.fees, nil
}

func (f *fakeFeeRepository) UpdateRetryState(ctx context.Context, fee domain.GatewayFee) error {
	for i := range f.fees {
		if f.fees[i].ID == fee.ID {
			f.fees[i] = fee
			return nil
		}
	}
	f.fees = append(f.fees, fee)
	return nil
}

func (f *fakeFeeRepository) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]domain.GatewayFee, error) {
	return f.fees, nil
}

func TestCalculateFee(t *testing.T) {
	if got := CalculateFee(100000, 0.005); got != 500 {
		t.Fatalf("expected 500, got %d", got)
	}
	if got := CalculateFee(101, 0.005); got != 1 {
		t.Fatalf("expected half-up rounded fee 1, got %d", got)
	}
	if got := CalculateFee(99, 0.005); got != 0 {
		t.Fatalf("expected rounded down fee 0, got %d", got)
	}
	if got := CalculateFee(0, 0.005); got != 0 {
		t.Fatalf("expected fee 0, got %d", got)
	}
}

func TestFeeServiceChargeSuccess(t *testing.T) {
	repo := &fakeFeeRepository{}
	feeService := NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, repo, fakeSmartBank{})

	result, err := feeService.Charge(context.Background(), Principal{UserID: "user_123", SourceApp: "marketplace"}, "req-1", "marketplace", []byte(`{"transaction_amount":100000}`))
	if err != nil {
		t.Fatalf("charge fee: %v", err)
	}
	if result.Amount != 500 || result.Status != "success" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.fees) != 1 || repo.fees[0].Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS fee record, got %+v", repo.fees)
	}
}

func TestFeeServiceChargeDeferred(t *testing.T) {
	repo := &fakeFeeRepository{}
	feeService := NewFeeService(config.FeeConfig{
		RevenueUser: "GATEWAY_REVENUE",
		Rate:        0.005,
	}, repo, fakeSmartBank{err: errors.New("smartbank down")})

	result, err := feeService.Charge(context.Background(), Principal{UserID: "user_123", SourceApp: "marketplace"}, "req-1", "marketplace", []byte(`{"data":{"transaction_amount":100000}}`))
	if err != nil {
		t.Fatalf("charge fee: %v", err)
	}
	if result.Amount != 500 || result.Status != "deferred" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(repo.fees) != 1 || repo.fees[0].Status != "PENDING" || repo.fees[0].NextRetryAt == nil {
		t.Fatalf("expected PENDING fee record with retry schedule, got %+v", repo.fees)
	}
}

func TestExtractTransactionAmountMissing(t *testing.T) {
	_, err := ExtractTransactionAmount([]byte(`{"ok":true}`))
	if !errors.Is(err, ErrTransactionAmountMissing) {
		t.Fatalf("expected ErrTransactionAmountMissing, got %v", err)
	}
}
