//go:build integration

package repositories_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

func TestAccountRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	sessions := repositories.NewUserSessionRepository(pool)

	name := "Ana Souza"
	passwordUser, err := users.CreateWithPassword(t.Context(), "account-password@example.com", "$argon2id$old-hash", &name)
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}
	googleUser, err := users.CreateWithIdentity(t.Context(), "account-google@example.com", "google", "google-sub-account")
	if err != nil {
		t.Fatalf("CreateWithIdentity() error = %v", err)
	}

	t.Run("reports how each account signs in", func(t *testing.T) {
		account, err := users.FindAccountByID(t.Context(), passwordUser.ID)
		if err != nil || !account.HasPassword || account.HasGoogle || account.FullName == nil || *account.FullName != name {
			t.Errorf("password account = (%+v, %v), want password, no Google, name %q", account, err, name)
		}

		account, err = users.FindAccountByID(t.Context(), googleUser.ID)
		if err != nil || account.HasPassword || !account.HasGoogle || account.FullName != nil {
			t.Errorf("Google account = (%+v, %v), want Google only and no name yet", account, err)
		}
	})

	t.Run("fills a missing name once and keeps an edited one", func(t *testing.T) {
		if err := users.FillMissingFullName(t.Context(), googleUser.ID, "Nome do Google"); err != nil {
			t.Fatalf("FillMissingFullName() error = %v", err)
		}
		if err := users.UpdateFullName(t.Context(), googleUser.ID, "Nome Editado"); err != nil {
			t.Fatalf("UpdateFullName() error = %v", err)
		}
		if err := users.FillMissingFullName(t.Context(), googleUser.ID, "Nome do Google"); err != nil {
			t.Fatalf("second FillMissingFullName() error = %v", err)
		}

		account, err := users.FindAccountByID(t.Context(), googleUser.ID)
		if err != nil || account.FullName == nil || *account.FullName != "Nome Editado" {
			t.Errorf("full name = %v (%v), want the edited name", account.FullName, err)
		}
	})

	t.Run("rejects a name longer than the column allows", func(t *testing.T) {
		long := make([]rune, 101)
		for i := range long {
			long[i] = 'a'
		}
		if err := users.UpdateFullName(t.Context(), passwordUser.ID, string(long)); err == nil {
			t.Error("UpdateFullName() accepted 101 characters")
		}
	})

	t.Run("replaces the password and every session in one step", func(t *testing.T) {
		first := sha256.Sum256([]byte("first-device"))
		second := sha256.Sum256([]byte("second-device"))
		renewed := sha256.Sum256([]byte("renewed-session"))
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
		for _, hash := range [][32]byte{first, second} {
			if err := sessions.Create(t.Context(), passwordUser.ID, hash[:], expiresAt); err != nil {
				t.Fatalf("create session: %v", err)
			}
		}

		if err := users.ReplacePasswordAndSessions(t.Context(), passwordUser.ID, "$argon2id$new-hash", renewed[:], expiresAt); err != nil {
			t.Fatalf("ReplacePasswordAndSessions() error = %v", err)
		}

		hash, err := users.FindPasswordHashByUserID(t.Context(), passwordUser.ID)
		if err != nil || hash != "$argon2id$new-hash" {
			t.Errorf("password hash = (%q, %v), want the new hash", hash, err)
		}
		for _, old := range [][32]byte{first, second} {
			if _, err := sessions.FindSessionByTokenHash(t.Context(), old[:]); !errors.Is(err, repositories.ErrUserSessionNotFound) {
				t.Errorf("old session lookup error = %v, want it revoked", err)
			}
		}
		session, err := sessions.FindSessionByTokenHash(t.Context(), renewed[:])
		if err != nil || session.UserID != passwordUser.ID || !session.ExpiresAt.Equal(expiresAt) {
			t.Errorf("renewed session = (%+v, %v), want it with the same expiry", session, err)
		}
	})

	t.Run("reports an account without password", func(t *testing.T) {
		if _, err := users.FindPasswordHashByUserID(t.Context(), googleUser.ID); !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Errorf("FindPasswordHashByUserID() error = %v, want %v", err, repositories.ErrPasswordCredentialNotFound)
		}
		renewed := sha256.Sum256([]byte("google-renewed"))
		err := users.ReplacePasswordAndSessions(t.Context(), googleUser.ID, "$argon2id$hash", renewed[:], time.Now().Add(time.Hour))
		if !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Errorf("ReplacePasswordAndSessions() error = %v, want %v", err, repositories.ErrPasswordCredentialNotFound)
		}
	})
}
