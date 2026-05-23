package domain

import "time"

type JWTBlacklist struct {
	ID        int64     `db:"id"`
	JTI       string    `db:"jti"`
	UserID    string    `db:"user_id"`
	Reason    *string   `db:"reason"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}
