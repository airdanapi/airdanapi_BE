package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type JWTBlacklistRepository interface {
	ExistsActiveJTI(ctx context.Context, jti string) (bool, error)
}

type MySQLJWTBlacklistRepository struct {
	db *sqlx.DB
}

func NewJWTBlacklistRepository(db *sqlx.DB) MySQLJWTBlacklistRepository {
	return MySQLJWTBlacklistRepository{db: db}
}

func (r MySQLJWTBlacklistRepository) ExistsActiveJTI(ctx context.Context, jti string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM jwt_blacklist
			WHERE jti = ? AND expires_at > NOW()
		)`

	var exists bool
	if err := r.db.GetContext(ctx, &exists, query, jti); err != nil {
		return false, err
	}

	return exists, nil
}
