package repositories

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/jackc/pgx/v5"
)

type UserSessionRepository struct {
	db db.DB
}

func NewUserSessionRepository(database db.DB) *UserSessionRepository {
	return &UserSessionRepository{
		db: database,
	}
}

func (r *UserSessionRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	const q = `	
		INSERT INTO user_sessions(token_hash, user_id, expires_at)
		VALUES(@tokenHash, @userID, @expiresAt)
	`

	args := pgx.StrictNamedArgs{
		"tokenHash": tokenHash,
		"userID":    userID,
		"expiresAt": expiresAt,
	}

	if _, err := r.db.Exec(ctx, q, args); err != nil {
		return fmt.Errorf("insert user session: %w", err)
	}

	return nil

}
