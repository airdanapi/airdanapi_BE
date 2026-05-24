package repository

import (
	"context"
	"database/sql"
	"errors"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("record not found")

type RouteRepository interface {
	FindActiveByServiceFeatureMethod(ctx context.Context, service, feature, method string) (domain.Route, error)
	ListActive(ctx context.Context) ([]domain.Route, error)
}

type RouteInput struct {
	ServiceName   string
	FeatureName   string
	Method        string
	DownstreamURL string
	Transactional bool
	RouteClass    string
	TimeoutMS     int
	RetryCount    int
	RequiredScope *string
	IsActive      bool
	Description   *string
}

type MySQLRouteRepository struct {
	db *sqlx.DB
}

func NewRouteRepository(db *sqlx.DB) MySQLRouteRepository {
	return MySQLRouteRepository{db: db}
}

func (r MySQLRouteRepository) FindActiveByServiceFeatureMethod(ctx context.Context, service, feature, method string) (domain.Route, error) {
	const query = `
		SELECT id, service_name, feature_name, method, downstream_url, transactional, route_class,
		       timeout_ms, retry_count, required_scope, is_active, description, created_at, updated_at
		FROM routes_registry
		WHERE service_name = ? AND feature_name = ? AND method = ? AND is_active = TRUE
		LIMIT 1`

	var route domain.Route
	if err := r.db.GetContext(ctx, &route, query, service, feature, method); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Route{}, ErrNotFound
		}
		return domain.Route{}, err
	}

	return route, nil
}

func (r MySQLRouteRepository) ListActive(ctx context.Context) ([]domain.Route, error) {
	const query = `
		SELECT id, service_name, feature_name, method, downstream_url, transactional, route_class,
		       timeout_ms, retry_count, required_scope, is_active, description, created_at, updated_at
		FROM routes_registry
		WHERE is_active = TRUE
		ORDER BY service_name, feature_name, method`

	var routes []domain.Route
	if err := r.db.SelectContext(ctx, &routes, query); err != nil {
		return nil, err
	}

	return routes, nil
}

func (r MySQLRouteRepository) ListAll(ctx context.Context) ([]domain.Route, error) {
	const query = `
		SELECT id, service_name, feature_name, method, downstream_url, transactional, route_class,
		       timeout_ms, retry_count, required_scope, is_active, description, created_at, updated_at
		FROM routes_registry
		ORDER BY service_name, feature_name, method`

	var routes []domain.Route
	if err := r.db.SelectContext(ctx, &routes, query); err != nil {
		return nil, err
	}

	return routes, nil
}

func (r MySQLRouteRepository) Create(ctx context.Context, input RouteInput) (domain.Route, error) {
	const query = `
		INSERT INTO routes_registry (
			service_name, feature_name, method, downstream_url, transactional, route_class,
			timeout_ms, retry_count, required_scope, is_active, description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		input.ServiceName,
		input.FeatureName,
		input.Method,
		input.DownstreamURL,
		input.Transactional,
		input.RouteClass,
		input.TimeoutMS,
		input.RetryCount,
		input.RequiredScope,
		input.IsActive,
		input.Description,
	)
	if err != nil {
		return domain.Route{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return domain.Route{}, err
	}

	return r.FindByID(ctx, id)
}

func (r MySQLRouteRepository) Update(ctx context.Context, id int64, input RouteInput) (domain.Route, error) {
	const query = `
		UPDATE routes_registry
		SET service_name = ?, feature_name = ?, method = ?, downstream_url = ?, transactional = ?,
		    route_class = ?, timeout_ms = ?, retry_count = ?, required_scope = ?, is_active = ?,
		    description = ?
		WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query,
		input.ServiceName,
		input.FeatureName,
		input.Method,
		input.DownstreamURL,
		input.Transactional,
		input.RouteClass,
		input.TimeoutMS,
		input.RetryCount,
		input.RequiredScope,
		input.IsActive,
		input.Description,
		id,
	)
	if err != nil {
		return domain.Route{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Route{}, err
	}
	if affected == 0 {
		return domain.Route{}, ErrNotFound
	}

	return r.FindByID(ctx, id)
}

func (r MySQLRouteRepository) Toggle(ctx context.Context, id int64, active bool) (domain.Route, error) {
	const query = `UPDATE routes_registry SET is_active = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, active, id)
	if err != nil {
		return domain.Route{}, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Route{}, err
	}
	if affected == 0 {
		return domain.Route{}, ErrNotFound
	}

	return r.FindByID(ctx, id)
}

func (r MySQLRouteRepository) FindByID(ctx context.Context, id int64) (domain.Route, error) {
	const query = `
		SELECT id, service_name, feature_name, method, downstream_url, transactional, route_class,
		       timeout_ms, retry_count, required_scope, is_active, description, created_at, updated_at
		FROM routes_registry
		WHERE id = ?
		LIMIT 1`

	var route domain.Route
	if err := r.db.GetContext(ctx, &route, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Route{}, ErrNotFound
		}
		return domain.Route{}, err
	}

	return route, nil
}
