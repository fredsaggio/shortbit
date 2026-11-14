package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/jackc/pgx/v5"
)

type LinkAccessSessionRepository struct {
	db db.DB
}

func NewLinkAccessSessionRepository(db db.DB) *LinkAccessSessionRepository {
	return &LinkAccessSessionRepository{
		db: db,
	}
}

func (r *LinkAccessSessionRepository) CreateLinkAccessSession(ctx context.Context, linkID int64, tokenHash []byte, expiresAt time.Time) error {
	const q = `
		INSERT INTO link_access_sessions(token_hash, link_id, expires_at)
		VALUES(@tokenHash, @linkID, @expiresAt)
	`

	args := pgx.StrictNamedArgs{
		"tokenHash": tokenHash,
		"linkID":    linkID,
		"expiresAt": expiresAt,
	}

	_, err := r.db.Exec(ctx, q, args)

	if err != nil {
		return fmt.Errorf("insert link access session: %w", err)
	}

	return nil
}
