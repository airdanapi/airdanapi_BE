package domain

import "time"

type GatewayFee struct {
	ID                int64      `db:"id"`
	RequestID         string     `db:"request_id"`
	UserID            string     `db:"user_id"`
	SourceApp         string     `db:"source_app"`
	TransactionAmount int64      `db:"transaction_amount"`
	FeeAmount         int64      `db:"fee_amount"`
	FeeRate           float64    `db:"fee_rate"`
	Status            string     `db:"status"`
	RetryCount        int        `db:"retry_count"`
	MaxRetries        int        `db:"max_retries"`
	NextRetryAt       *time.Time `db:"next_retry_at"`
	SmartBankRef      *string    `db:"smartbank_ref"`
	ErrorMessage      *string    `db:"error_message"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}
