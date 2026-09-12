//go:build integration

package repositories_test

import (
	"errors"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

func TestUserRepositoryCreateWithPasswordIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	repository := repositories.NewUserRepository(pool)

	t.Run("creates user and password credential", func(t *testing.T) {
		const (
			email        = "user@example.com"
			passwordHash = "$argon2id$integration-test-hash"
		)

		user, err := repository.CreateWithPassword(
			t.Context(),
			email,
			passwordHash,
		)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if user.ID == uuid.Nil() {
			t.Error("CreateWithPassword() returned a nil user ID")
		}

		if version := user.ID[6] >> 4; version != 7 {
			t.Errorf("CreateWithPassword() UUID version = %d, want 7", version)
		}

		if user.Email != email {
			t.Errorf("CreateWithPassword() email = %q, want %q", user.Email, email)
		}

		if user.CreatedAt.IsZero() {
			t.Error("CreateWithPassword() returned a zero CreatedAt")
		}

		if user.UpdatedAt.IsZero() {
			t.Error("CreateWithPassword() returned a zero UpdatedAt")
		}

		var (
			storedEmail        string
			credentialUserID   uuid.UUID
			storedPasswordHash string
		)

		err = pool.QueryRow(
			t.Context(),
			`
				SELECT u.email, pc.user_id, pc.password_hash
				FROM users AS u
				JOIN password_credentials AS pc ON pc.user_id = u.id
				WHERE u.id = $1
			`,
			user.ID,
		).Scan(
			&storedEmail,
			&credentialUserID,
			&storedPasswordHash,
		)
		if err != nil {
			t.Fatalf("query created user and credential: %v", err)
		}

		if storedEmail != email {
			t.Errorf("stored email = %q, want %q", storedEmail, email)
		}

		if credentialUserID != user.ID {
			t.Errorf(
				"credential user ID = %s, want %s",
				credentialUserID,
				user.ID,
			)
		}

		if storedPasswordHash != passwordHash {
			t.Errorf(
				"stored password hash = %q, want %q",
				storedPasswordHash,
				passwordHash,
			)
		}
	})

	t.Run("recognizes duplicate email", func(t *testing.T) {
		const email = "duplicate@example.com"

		if _, err := repository.CreateWithPassword(
			t.Context(),
			email,
			"$argon2id$first-hash",
		); err != nil {
			t.Fatalf("first CreateWithPassword() error = %v", err)
		}

		_, err := repository.CreateWithPassword(
			t.Context(),
			email,
			"$argon2id$second-hash",
		)
		if !errors.Is(err, repositories.ErrEmailAlreadyExists) {
			t.Fatalf(
				"second CreateWithPassword() error = %v, want %v",
				err,
				repositories.ErrEmailAlreadyExists,
			)
		}

		var usersWithEmail int
		if err := pool.QueryRow(
			t.Context(),
			"SELECT COUNT(*) FROM users WHERE email = $1",
			email,
		).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users with duplicate email: %v", err)
		}

		if usersWithEmail != 1 {
			t.Errorf("users with duplicate email = %d, want 1", usersWithEmail)
		}
	})

	t.Run("rolls back user when credential insert fails", func(t *testing.T) {
		const email = "rollback@example.com"

		_, err := repository.CreateWithPassword(
			t.Context(),
			email,
			" ",
		)
		if err == nil {
			t.Fatal("CreateWithPassword() error = nil, want credential constraint error")
		}

		var usersWithEmail int
		if err := pool.QueryRow(
			t.Context(),
			"SELECT COUNT(*) FROM users WHERE email = $1",
			email,
		).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after credential failure: %v", err)
		}

		if usersWithEmail != 0 {
			t.Errorf("users left after credential failure = %d, want 0", usersWithEmail)
		}
	})
}
