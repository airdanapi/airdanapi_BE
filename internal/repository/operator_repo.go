package repository

import (
	"context"
	"database/sql"
	"errors"

	"airdanapi-be/internal/domain"

	"github.com/jmoiron/sqlx"
)

type OperatorRepository interface {
	FindByEmail(ctx context.Context, email string) (domain.Operator, error)
}

type MySQLOperatorRepository struct {
	db *sqlx.DB
}

func NewOperatorRepository(db *sqlx.DB) MySQLOperatorRepository {
	return MySQLOperatorRepository{db: db}
}

func (r MySQLOperatorRepository) FindByEmail(ctx context.Context, email string) (domain.Operator, error) {
	const query = `
		SELECT id, email, password_hash, name, role, is_active, last_login_at, created_at, updated_at
		FROM operators
		WHERE email = ? AND is_active = TRUE
		LIMIT 1`

	var operator domain.Operator
	if err := r.db.GetContext(ctx, &operator, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Operator{}, ErrNotFound
		}
		return domain.Operator{}, err
	}

	return operator, nil
}
