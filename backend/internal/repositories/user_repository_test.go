//go:build integration

package repositories_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUserRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	repository := repositories.NewUserRepository(pool)

	runPasswordRegistrationRepositoryTests(t, pool, repository)
	runUserRepositoryCoreBeforeResetTests(t, pool, repository)
	runPasswordResetRepositoryTests(t, pool, repository)
	runUserRepositoryCoreAfterResetTests(t, pool, repository)
	runGoogleIdentityRepositoryTests(t, pool, repository)
}

func runUserRepositoryCoreBeforeResetTests(t *testing.T, pool *pgxpool.Pool, repository *repositories.UserRepository) {
	t.Run("returns false when user email does not exist", func(t *testing.T) {
		exists, err := repository.UserExistsByEmail(t.Context(), "available@example.com")
		if err != nil {
			t.Fatalf("UserExistsByEmail() error = %v", err)
		}

		if exists {
			t.Error("UserExistsByEmail() = true, want false")
		}
	})

	t.Run("returns true when user email exists", func(t *testing.T) {
		const email = "registered@example.com"

		if _, err := pool.Exec(
			t.Context(),
			"INSERT INTO users(email, email_verified_at) VALUES ($1, $2)",
			email,
			time.Now().UTC(),
		); err != nil {
			t.Fatalf("insert existing user: %v", err)
		}

		exists, err := repository.UserExistsByEmail(t.Context(), email)
		if err != nil {
			t.Fatalf("UserExistsByEmail() error = %v", err)
		}

		if !exists {
			t.Error("UserExistsByEmail() = false, want true")
		}
	})

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

	t.Run("finds password credentials by email", func(t *testing.T) {
		const (
			email        = "login@example.com"
			passwordHash = "$argon2id$login-test-hash"
		)

		createdUser, err := repository.CreateWithPassword(
			t.Context(),
			email,
			passwordHash,
		)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		user, credential, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			email,
		)
		if err != nil {
			t.Fatalf("FindPasswordCredentialsByEmail() error = %v", err)
		}

		if user != createdUser {
			t.Errorf(
				"FindPasswordCredentialsByEmail() user = %+v, want %+v",
				user,
				createdUser,
			)
		}

		if credential.UserID != createdUser.ID {
			t.Errorf(
				"credential user ID = %s, want %s",
				credential.UserID,
				createdUser.ID,
			)
		}

		if credential.PasswordHash != passwordHash {
			t.Errorf(
				"credential password hash = %q, want %q",
				credential.PasswordHash,
				passwordHash,
			)
		}

		if credential.CreatedAt.IsZero() {
			t.Error("credential has a zero CreatedAt")
		}

		if credential.UpdatedAt.IsZero() {
			t.Error("credential has a zero UpdatedAt")
		}
	})
}

func runUserRepositoryCoreAfterResetTests(t *testing.T, pool *pgxpool.Pool, repository *repositories.UserRepository) {
	t.Run("finds user by ID", func(t *testing.T) {
		createdUser, err := repository.CreateWithPassword(t.Context(), "find-by-id@example.com", "$argon2id$find-by-id-test-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		user, err := repository.FindByID(t.Context(), createdUser.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}

		if user != createdUser {
			t.Errorf("FindByID() user = %+v, want %+v", user, createdUser)
		}
	})

	t.Run("returns not found for unknown user ID", func(t *testing.T) {
		unknownUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c399")

		_, err := repository.FindByID(t.Context(), unknownUserID)
		if !errors.Is(err, repositories.ErrUserNotFound) {
			t.Fatalf("FindByID() error = %v, want %v", err, repositories.ErrUserNotFound)
		}
	})

	t.Run("returns not found for unknown email", func(t *testing.T) {
		_, _, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			"missing@example.com",
		)
		if !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Fatalf(
				"FindPasswordCredentialsByEmail() error = %v, want %v",
				err,
				repositories.ErrPasswordCredentialNotFound,
			)
		}
	})

	t.Run("returns not found for user without password", func(t *testing.T) {
		const email = "google-only@example.com"

		if _, err := pool.Exec(
			t.Context(),
			"INSERT INTO users(email, email_verified_at) VALUES ($1, NOW())",
			email,
		); err != nil {
			t.Fatalf("insert user without password: %v", err)
		}

		_, _, err := repository.FindPasswordCredentialsByEmail(
			t.Context(),
			email,
		)
		if !errors.Is(err, repositories.ErrPasswordCredentialNotFound) {
			t.Fatalf(
				"FindPasswordCredentialsByEmail() error = %v, want %v",
				err,
				repositories.ErrPasswordCredentialNotFound,
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
