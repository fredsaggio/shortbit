//go:build integration

package repositories_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

func TestUserSessionRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	userRepository := repositories.NewUserRepository(pool)
	sessionRepository := repositories.NewUserSessionRepository(pool)

	user, err := userRepository.CreateWithPassword(t.Context(), "session-repository@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}

	t.Run("finds a valid session", func(t *testing.T) {
		tokenHashArray := sha256.Sum256([]byte("valid-session-token"))
		tokenHash := tokenHashArray[:]
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)

		if err := sessionRepository.Create(t.Context(), user.ID, tokenHash, expiresAt); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		session, err := sessionRepository.FindSessionByTokenHash(t.Context(), tokenHash)
		if err != nil {
			t.Fatalf("FindSessionByTokenHash() error = %v", err)
		}

		if !bytes.Equal(session.TokenHash, tokenHash) {
			t.Errorf("session token hash = %x, want %x", session.TokenHash, tokenHash)
		}

		if session.UserID != user.ID {
			t.Errorf("session user ID = %s, want %s", session.UserID, user.ID)
		}

		if session.CreatedAt.IsZero() {
			t.Error("session has a zero CreatedAt")
		}

		if !session.ExpiresAt.Equal(expiresAt) {
			t.Errorf("session expiration = %s, want %s", session.ExpiresAt, expiresAt)
		}
	})

	t.Run("returns not found for an unknown token hash", func(t *testing.T) {
		unknownTokenHash := sha256.Sum256([]byte("unknown-session-token"))

		_, err := sessionRepository.FindSessionByTokenHash(t.Context(), unknownTokenHash[:])
		if !errors.Is(err, repositories.ErrUserSessionNotFound) {
			t.Fatalf("FindSessionByTokenHash() error = %v, want %v", err, repositories.ErrUserSessionNotFound)
		}
	})

	t.Run("returns not found for an expired session", func(t *testing.T) {
		expiredTokenHash := sha256.Sum256([]byte("expired-session-token"))
		now := time.Now().UTC()

		_, err := pool.Exec(
			t.Context(),
			`INSERT INTO user_sessions(token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`,
			expiredTokenHash[:],
			user.ID,
			now.Add(-2*time.Hour),
			now.Add(-time.Hour),
		)
		if err != nil {
			t.Fatalf("insert expired session: %v", err)
		}

		_, err = sessionRepository.FindSessionByTokenHash(t.Context(), expiredTokenHash[:])
		if !errors.Is(err, repositories.ErrUserSessionNotFound) {
			t.Fatalf("FindSessionByTokenHash() error = %v, want %v", err, repositories.ErrUserSessionNotFound)
		}
	})

	t.Run("deletes a session by token hash idempotently", func(t *testing.T) {
		tokenHash := sha256.Sum256([]byte("session-token-to-delete"))
		expiresAt := time.Now().UTC().Add(time.Hour)

		if err := sessionRepository.Create(t.Context(), user.ID, tokenHash[:], expiresAt); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if err := sessionRepository.DeleteSessionByTokenHash(t.Context(), tokenHash[:]); err != nil {
			t.Fatalf("DeleteSessionByTokenHash() error = %v", err)
		}

		_, err := sessionRepository.FindSessionByTokenHash(t.Context(), tokenHash[:])
		if !errors.Is(err, repositories.ErrUserSessionNotFound) {
			t.Fatalf("FindSessionByTokenHash() error = %v, want %v", err, repositories.ErrUserSessionNotFound)
		}

		if err := sessionRepository.DeleteSessionByTokenHash(t.Context(), tokenHash[:]); err != nil {
			t.Fatalf("second DeleteSessionByTokenHash() error = %v, want nil", err)
		}
	})
}
