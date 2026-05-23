package repository

import (
	"context"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

type RequestLogRepository interface {
	Create(ctx context.Context, log domain.RequestLog) (int64, error)
}

type MySQLRequestLogRepository struct {
	db *sqlx.DB
}

func NewRequestLogRepository(db *sqlx.DB) MySQLRequestLogRepository {
	return MySQLRequestLogRepository{db: db}
}

func (r MySQLRequestLogRepository) Create(ctx context.Context, log domain.RequestLog) (int64, error) {
	const query = `
		INSERT INTO request_logs (
			request_id, parent_request_id, user_id, source_app, target_app, endpoint, method,
			status_code, latency_ms, ip_address, request_hash, response_hash, lifecycle, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		log.RequestID,
		log.ParentRequestID,
		log.UserID,
		log.SourceApp,
		log.TargetApp,
		log.Endpoint,
		log.Method,
		log.StatusCode,
		log.LatencyMS,
		log.IPAddress,
		log.RequestHash,
		log.ResponseHash,
		log.Lifecycle,
		log.ErrorMessage,
	)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}
