//go:build integration

package repositories_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLinkAccessSessionRepositoryCreateIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	sessions := repositories.NewLinkAccessSessionRepository(pool)

	owner, err := users.CreateWithPassword(t.Context(), "link-session-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	linkID, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() error = %v", err)
	}
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	code, err := generator.Generate(linkID)
	if err != nil {
		t.Fatalf("Generate(%d) error = %v", linkID, err)
	}
	passwordHash := "$argon2id$private-link-test-hash"
	_, err = urls.Create(t.Context(), models.URL{
		ID: linkID, ShortCode: code, UserID: owner.ID, OriginalURL: "https://example.com/private",
		Visibility: models.VisibilityPrivate, PasswordHash: &passwordHash,
	})
	if err != nil {
		t.Fatalf("create private URL: %v", err)
	}

	tokenHash := sha256.Sum256([]byte("test-link-access-token"))
	expiresAt := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Microsecond)
	if err := sessions.CreateLinkAccessSession(t.Context(), linkID, tokenHash[:], expiresAt); err != nil {
		t.Fatalf("CreateLinkAccessSession() error = %v", err)
	}

	var (
		storedHash      []byte
		storedLinkID    int64
		storedCreatedAt time.Time
		storedExpiresAt time.Time
	)
	err = pool.QueryRow(t.Context(), `
		SELECT token_hash, link_id, created_at, expires_at
		FROM link_access_sessions
		WHERE token_hash = @tokenHash
	`, pgx.StrictNamedArgs{"tokenHash": tokenHash[:]}).Scan(&storedHash, &storedLinkID, &storedCreatedAt, &storedExpiresAt)
	if err != nil {
		t.Fatalf("query created link access session: %v", err)
	}
	if !bytes.Equal(storedHash, tokenHash[:]) || storedLinkID != linkID || !storedExpiresAt.Equal(expiresAt) {
		t.Errorf("stored session = (hash %x, link ID %d, expiry %v), want (%x, %d, %v)",
			storedHash, storedLinkID, storedExpiresAt, tokenHash, linkID, expiresAt)
	}
	if storedCreatedAt.IsZero() || !storedCreatedAt.Before(storedExpiresAt) {
		t.Errorf("created_at = %v, want nonzero time before expiry %v", storedCreatedAt, storedExpiresAt)
	}

	err = sessions.CreateLinkAccessSession(t.Context(), linkID, tokenHash[:], expiresAt)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Errorf("duplicate token hash error = %v, want unique constraint violation", err)
	}
}
