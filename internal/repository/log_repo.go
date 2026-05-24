package repository

import (
	"context"
	"strings"
	"time"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

type RequestLogRepository interface {
	Create(ctx context.Context, log domain.RequestLog) (int64, error)
	List(ctx context.Context, filter RequestLogFilter) ([]domain.RequestLog, error)
}

type RequestLogFilter struct {
	UserID     string
	RequestID  string
	From       *time.Time
	To         *time.Time
	StatusCode *int
	TargetApp  string
	Page       int
	PerPage    int
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

func (r MySQLRequestLogRepository) List(ctx context.Context, filter RequestLogFilter) ([]domain.RequestLog, error) {
	conditions := []string{"1=1"}
	args := []any{}

	if strings.TrimSpace(filter.UserID) != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, strings.TrimSpace(filter.UserID))
	}
	if strings.TrimSpace(filter.RequestID) != "" {
		conditions = append(conditions, "request_id = ?")
		args = append(args, strings.TrimSpace(filter.RequestID))
	}
	if filter.From != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, *filter.To)
	}
	if filter.StatusCode != nil {
		conditions = append(conditions, "status_code = ?")
		args = append(args, *filter.StatusCode)
	}
	if strings.TrimSpace(filter.TargetApp) != "" {
		conditions = append(conditions, "target_app = ?")
		args = append(args, strings.TrimSpace(filter.TargetApp))
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
		SELECT id, request_id, parent_request_id, user_id, source_app, target_app, endpoint,
		       method, status_code, latency_ms, ip_address, request_hash, response_hash,
		       lifecycle, error_message, created_at
		FROM request_logs
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	var logs []domain.RequestLog
	if err := r.db.SelectContext(ctx, &logs, query, args...); err != nil {
		return nil, err
	}

	return logs, nil
}
