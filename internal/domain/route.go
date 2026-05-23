package domain

import "time"

type Route struct {
	ID            int64     `db:"id"`
	ServiceName   string    `db:"service_name"`
	FeatureName   string    `db:"feature_name"`
	Method        string    `db:"method"`
	DownstreamURL string    `db:"downstream_url"`
	Transactional bool      `db:"transactional"`
	RouteClass    string    `db:"route_class"`
	TimeoutMS     int       `db:"timeout_ms"`
	RetryCount    int       `db:"retry_count"`
	RequiredScope *string   `db:"required_scope"`
	IsActive      bool      `db:"is_active"`
	Description   *string   `db:"description"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
