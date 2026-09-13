package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

var ErrUserSessionNotFound = errors.New("user session not found")

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

func (r *UserSessionRepository) FindSessionByTokenHash(ctx context.Context, tokenHash []byte) (models.UserSession, error) {
	const q = `
		SELECT
			us.token_hash,
			us.user_id,
			us.created_at,
			us.expires_at
		FROM user_sessions AS us
		WHERE us.token_hash = @tokenHash
			AND us.expires_at > NOW()
			`

	args := pgx.StrictNamedArgs{
		"tokenHash": tokenHash,
	}

	var userSession models.UserSession

	err := r.db.QueryRow(ctx, q, args).Scan(&userSession.TokenHash, &userSession.UserID, &userSession.CreatedAt, &userSession.ExpiresAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.UserSession{}, ErrUserSessionNotFound
		}
		return models.UserSession{}, fmt.Errorf("find user session with token: %w", err)
	}

	return userSession, nil
}
