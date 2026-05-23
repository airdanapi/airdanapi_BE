package repository

import (
	"context"
	"database/sql"
	"errors"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

type GatewayFeeRepository interface {
	Create(ctx context.Context, fee domain.GatewayFee) (int64, error)
	FindByRequestID(ctx context.Context, requestID string) (domain.GatewayFee, error)
}

type MySQLGatewayFeeRepository struct {
	db *sqlx.DB
}

func NewGatewayFeeRepository(db *sqlx.DB) MySQLGatewayFeeRepository {
	return MySQLGatewayFeeRepository{db: db}
}

func (r MySQLGatewayFeeRepository) Create(ctx context.Context, fee domain.GatewayFee) (int64, error) {
	const query = `
		INSERT INTO gateway_fees (
			request_id, user_id, source_app, transaction_amount, fee_amount, fee_rate, status,
			retry_count, max_retries, next_retry_at, smartbank_ref, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		fee.RequestID,
		fee.UserID,
		fee.SourceApp,
		fee.TransactionAmount,
		fee.FeeAmount,
		fee.FeeRate,
		fee.Status,
		fee.RetryCount,
		fee.MaxRetries,
		fee.NextRetryAt,
		fee.SmartBankRef,
		fee.ErrorMessage,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (r MySQLGatewayFeeRepository) FindByRequestID(ctx context.Context, requestID string) (domain.GatewayFee, error) {
	const query = `
		SELECT id, request_id, user_id, source_app, transaction_amount, fee_amount, fee_rate,
		       status, retry_count, max_retries, next_retry_at, smartbank_ref, error_message,
		       created_at, updated_at
		FROM gateway_fees
		WHERE request_id = ?
		LIMIT 1`

	var fee domain.GatewayFee
	if err := r.db.GetContext(ctx, &fee, query, requestID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.GatewayFee{}, ErrNotFound
		}
		return domain.GatewayFee{}, err
	}

	return fee, nil
}
