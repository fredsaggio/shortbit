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

var ErrURLNotFound = errors.New("url not found")

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

func (r *URLRepository) GetByShortcode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error) {
	const q = `
		SELECT
			id,
			short_code,
			user_id,
			original_url,
			visibility,
			click_count,
			created_at,
			updated_at
		FROM urls
		WHERE user_id = @userID AND short_code = @shortCode
	`

	args := pgx.StrictNamedArgs{
		"userID":    userID,
		"shortCode": shortCode,
	}

	var url models.URL

	if err := r.db.QueryRow(ctx, q, args).Scan(&url.ID, &url.ShortCode, &url.UserID, &url.OriginalURL, &url.Visibility, &url.ClickCount, &url.CreatedAt, &url.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.URL{}, ErrURLNotFound
		}

		return models.URL{}, fmt.Errorf("find url with shortcode: %w", err)
	}

	return url, nil
}

func (r *URLRepository) ResolvePublic(ctx context.Context, shortCode string) (int64, string, error) {
	const q = `
		SELECT id, original_url
		FROM urls
		WHERE short_code = @shortCode AND visibility = 'public'
	`

	args := pgx.StrictNamedArgs{
		"shortCode": shortCode,
	}

	var id int64
	var originalURL string

	if err := r.db.QueryRow(ctx, q, args).Scan(&id, &originalURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrURLNotFound
		}

		return 0, "", fmt.Errorf("resolve public URL: %w", err)
	}

	return id, originalURL, nil
}

func (r *URLRepository) FindPrivateByShortCode(ctx context.Context, shortCode string) (int64, string, error) {
	const q = `
		SELECT id, password_hash
		FROM urls
		WHERE short_code = @shortCode AND visibility = 'private'
	`

	var linkID int64
	var passwordHash string

	args := pgx.StrictNamedArgs{
		"shortCode": shortCode,
	}

	err := r.db.QueryRow(ctx, q, args).Scan(&linkID, &passwordHash)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrURLNotFound
		}

		return 0, "", fmt.Errorf("find private URL by shortcode: %w", err)
	}

	return linkID, passwordHash, nil
}

func (r *URLRepository) ResolvePrivate(ctx context.Context, linkID int64, tokenHash []byte) (string, error) {
	const q = `
		SELECT original_url
		FROM urls
		WHERE id = @linkID AND visibility = 'private'
			AND EXISTS (
				SELECT 1
				FROM link_access_sessions
				WHERE link_id = urls.id
					AND token_hash = @tokenHash
					AND expires_at > NOW()
			)
	`

	args := pgx.StrictNamedArgs{"linkID": linkID, "tokenHash": tokenHash}
	var originalURL string

	if err := r.db.QueryRow(ctx, q, args).Scan(&originalURL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", fmt.Errorf("resolve private URL: %w", err)
	}

	return originalURL, nil
}

// URLEdit is the full new state of a link. PasswordHash is used only when
// KeepPasswordHash is false; nil then clears it, as a public link requires.
type URLEdit struct {
	OriginalURL          string
	Visibility           models.Visibility
	PasswordHash         *string
	KeepPasswordHash     bool
	RevokeAccessSessions bool
}

// Update applies an edit to one of the owner's links. With
// RevokeAccessSessions, every unlock session of the link ends in the same
// transaction, so a new password locks out whoever used the old one.
func (r *URLRepository) Update(ctx context.Context, userID uuid.UUID, shortCode string, edit URLEdit) (models.URL, error) {
	const updateQuery = `
		UPDATE urls
		SET original_url = @originalURL,
			visibility = @visibility,
			password_hash = CASE WHEN CAST(@keepPasswordHash AS boolean) THEN password_hash ELSE CAST(@passwordHash AS text) END
		WHERE user_id = @userID AND short_code = @shortCode
		RETURNING id, short_code, user_id, original_url, visibility, click_count, created_at, updated_at
	`
	const revokeQuery = `DELETE FROM link_access_sessions WHERE link_id = @linkID`

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.URL{}, fmt.Errorf("begin URL update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updated models.URL
	err = tx.QueryRow(ctx, updateQuery, pgx.StrictNamedArgs{
		"originalURL":      edit.OriginalURL,
		"visibility":       edit.Visibility,
		"keepPasswordHash": edit.KeepPasswordHash,
		"passwordHash":     edit.PasswordHash,
		"userID":           userID,
		"shortCode":        shortCode,
	}).Scan(&updated.ID, &updated.ShortCode, &updated.UserID, &updated.OriginalURL, &updated.Visibility,
		&updated.ClickCount, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.URL{}, ErrURLNotFound
		}
		return models.URL{}, fmt.Errorf("update URL: %w", err)
	}

	if edit.RevokeAccessSessions {
		if _, err := tx.Exec(ctx, revokeQuery, pgx.StrictNamedArgs{"linkID": updated.ID}); err != nil {
			return models.URL{}, fmt.Errorf("revoke link access sessions: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.URL{}, fmt.Errorf("commit URL update: %w", err)
	}

	return updated, nil
}

// Delete removes one of the owner's links; its unlock sessions go with it
// (ON DELETE CASCADE). IDs are never reused, so neither is the short code.
func (r *URLRepository) Delete(ctx context.Context, userID uuid.UUID, shortCode string) error {
	const q = `DELETE FROM urls WHERE user_id = @userID AND short_code = @shortCode`

	tag, err := r.db.Exec(ctx, q, pgx.StrictNamedArgs{"userID": userID, "shortCode": shortCode})
	if err != nil {
		return fmt.Errorf("delete URL: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrURLNotFound
	}

	return nil
}
