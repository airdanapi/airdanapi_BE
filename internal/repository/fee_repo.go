package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

type GatewayFeeRepository interface {
	Create(ctx context.Context, fee domain.GatewayFee) (int64, error)
	FindByRequestID(ctx context.Context, requestID string) (domain.GatewayFee, error)
	FindByID(ctx context.Context, id int64) (domain.GatewayFee, error)
	List(ctx context.Context, filter GatewayFeeFilter) ([]domain.GatewayFee, error)
	UpdateRetryState(ctx context.Context, fee domain.GatewayFee) error
	ListDueRetries(ctx context.Context, now time.Time, limit int) ([]domain.GatewayFee, error)
}

type GatewayFeeFilter struct {
	Status    string
	UserID    string
	RequestID string
	Page      int
	PerPage   int
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

func (r MySQLGatewayFeeRepository) FindByID(ctx context.Context, id int64) (domain.GatewayFee, error) {
	const query = `
		SELECT id, request_id, user_id, source_app, transaction_amount, fee_amount, fee_rate,
		       status, retry_count, max_retries, next_retry_at, smartbank_ref, error_message,
		       created_at, updated_at
		FROM gateway_fees
		WHERE id = ?
		LIMIT 1`

	var fee domain.GatewayFee
	if err := r.db.GetContext(ctx, &fee, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.GatewayFee{}, ErrNotFound
		}
		return domain.GatewayFee{}, err
	}

	return fee, nil
}

func (r MySQLGatewayFeeRepository) List(ctx context.Context, filter GatewayFeeFilter) ([]domain.GatewayFee, error) {
	conditions := []string{"1=1"}
	args := []any{}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, strings.ToUpper(strings.TrimSpace(filter.Status)))
	}
	if strings.TrimSpace(filter.UserID) != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, strings.TrimSpace(filter.UserID))
	}
	if strings.TrimSpace(filter.RequestID) != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, strings.TrimSpace(filter.RequestID))
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	query := `
		SELECT id, request_id, user_id, source_app, transaction_amount, fee_amount, fee_rate,
		       status, retry_count, max_retries, next_retry_at, smartbank_ref, error_message,
		       created_at, updated_at
		FROM gateway_fees
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	var fees []domain.GatewayFee
	if err := r.db.SelectContext(ctx, &fees, query, args...); err != nil {
		return nil, err
	}

	return fees, nil
}

func (r MySQLGatewayFeeRepository) UpdateRetryState(ctx context.Context, fee domain.GatewayFee) error {
	const query = `
		UPDATE gateway_fees
		SET status = ?, retry_count = ?, next_retry_at = ?, smartbank_ref = ?, error_message = ?
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query,
		fee.Status,
		fee.RetryCount,
		fee.NextRetryAt,
		fee.SmartBankRef,
		fee.ErrorMessage,
		fee.ID,
	)
	return err
}

func (r MySQLGatewayFeeRepository) ListDueRetries(ctx context.Context, now time.Time, limit int) ([]domain.GatewayFee, error) {
	if limit <= 0 {
		limit = 20
	}

	const query = `
		SELECT id, request_id, user_id, source_app, transaction_amount, fee_amount, fee_rate,
		       status, retry_count, max_retries, next_retry_at, smartbank_ref, error_message,
		       created_at, updated_at
		FROM gateway_fees
		WHERE status = 'PENDING'
		  AND retry_count < max_retries
		  AND next_retry_at IS NOT NULL
		  AND next_retry_at <= ?
		ORDER BY next_retry_at ASC
		LIMIT ?`

	var fees []domain.GatewayFee
	if err := r.db.SelectContext(ctx, &fees, query, now, limit); err != nil {
		return nil, err
	}

	return fees, nil
}
