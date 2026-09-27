package repositories

import (
	"context"
	"fmt"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

type URLRepository struct {
	db db.DB
}

func NewURLRepository(db db.DB) *URLRepository {
	return &URLRepository{db: db}
}

func (r *URLRepository) ReserveID(ctx context.Context) (int64, error) {
	const q = `
		SELECT nextval(pg_get_serial_sequence('urls', 'id'))
	`

	var id int64

	if err := r.db.QueryRow(ctx, q).Scan(&id); err != nil {
		return 0, fmt.Errorf("reserve URL ID: %w", err)
	}

	return id, nil
}

func (r *URLRepository) Create(ctx context.Context, url models.URL) (models.URL, error) {
	const q = `
		INSERT INTO urls (
		id, short_code, user_id, original_url, visibility, password_hash
		)
		OVERRIDING SYSTEM VALUE
		VALUES(
			@id, @shortCode, @userID, @originalURL, @visibility, @passwordHash
		)
		RETURNING
			id, short_code, user_id, original_url, visibility, password_hash, click_count, created_at, updated_at
	`

	args := pgx.StrictNamedArgs{
		"id":           url.ID,
		"shortCode":    url.ShortCode,
		"userID":       url.UserID,
		"originalURL":  url.OriginalURL,
		"visibility":   url.Visibility,
		"passwordHash": url.PasswordHash,
	}

	var urlCreated models.URL

	err := r.db.QueryRow(ctx, q, args).Scan(
		&urlCreated.ID,
		&urlCreated.ShortCode,
		&urlCreated.UserID,
		&urlCreated.OriginalURL,
		&urlCreated.Visibility,
		&urlCreated.PasswordHash,
		&urlCreated.ClickCount,
		&urlCreated.CreatedAt,
		&urlCreated.UpdatedAt,
	)

	if err != nil {
		return models.URL{}, fmt.Errorf("create URL: %w", err)
	}

	return urlCreated, nil
}
