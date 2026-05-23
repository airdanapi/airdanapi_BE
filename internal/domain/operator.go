package domain

import "time"

type Operator struct {
	ID           int64      `db:"id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	Name         string     `db:"name"`
	Role         string     `db:"role"`
	IsActive     bool       `db:"is_active"`
	LastLoginAt  *time.Time `db:"last_login_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}
