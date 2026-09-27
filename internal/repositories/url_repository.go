package repositories

import (
	"context"
	"fmt"
	"time"
	"uuid"

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

type URLCursor struct {
	CreatedAt time.Time
	ID        int64
}

func (r *URLRepository) List(ctx context.Context, userID uuid.UUID, limit int, cursor *URLCursor) ([]models.URL, error) {
	const q = `
		SELECT id, short_code, user_id, original_url, visibility, click_count, created_at, updated_at
		FROM urls
		WHERE user_id = @userID
			AND (CAST(@cursorCreatedAt AS timestamptz) IS NULL
				OR (created_at, id) < (CAST(@cursorCreatedAt AS timestamptz), CAST(@cursorID AS bigint)))
		ORDER BY created_at DESC, id DESC
		LIMIT @limit
	`

	var cursorCreatedAt any
	var cursorID any
	if cursor != nil {
		cursorCreatedAt = cursor.CreatedAt
		cursorID = cursor.ID
	}

	args := pgx.StrictNamedArgs{
		"userID":          userID,
		"cursorCreatedAt": cursorCreatedAt,
		"cursorID":        cursorID,
		"limit":           limit,
	}
	rows, err := r.db.Query(ctx, q, args)
	if err != nil {
		return nil, fmt.Errorf("list URLs: %w", err)
	}
	defer rows.Close()

	urls := make([]models.URL, 0)
	for rows.Next() {
		var url models.URL
		if err := rows.Scan(&url.ID, &url.ShortCode, &url.UserID, &url.OriginalURL, &url.Visibility,
			&url.ClickCount, &url.CreatedAt, &url.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan listed URL: %w", err)
		}
		urls = append(urls, url)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed URLs: %w", err)
	}

	return urls, nil
}
