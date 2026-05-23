package domain

import "time"

type RequestLog struct {
	ID              int64     `db:"id"`
	RequestID       string    `db:"request_id"`
	ParentRequestID *string   `db:"parent_request_id"`
	UserID          *string   `db:"user_id"`
	SourceApp       *string   `db:"source_app"`
	TargetApp       string    `db:"target_app"`
	Endpoint        string    `db:"endpoint"`
	Method          string    `db:"method"`
	StatusCode      *int      `db:"status_code"`
	LatencyMS       *int      `db:"latency_ms"`
	IPAddress       string    `db:"ip_address"`
	RequestHash     *string   `db:"request_hash"`
	ResponseHash    *string   `db:"response_hash"`
	Lifecycle       string    `db:"lifecycle"`
	ErrorMessage    *string   `db:"error_message"`
	CreatedAt       time.Time `db:"created_at"`
}
