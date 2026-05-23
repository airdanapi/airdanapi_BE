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
