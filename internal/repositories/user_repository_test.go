//go:build integration

package repositories_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

func TestUserRepositoryIntegration(t *testing.T) {
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
			"INSERT INTO users(email) VALUES ($1)",
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

	t.Run("creates user and auth identity", func(t *testing.T) {
		const (
			email          = "identity@example.com"
			provider       = "google"
			providerUserID = "google-subject-123"
		)

		user, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		if user.ID == uuid.Nil() {
			t.Error("CreateWithIdentity() returned a nil user ID")
		}

		if version := user.ID[6] >> 4; version != 7 {
			t.Errorf("CreateWithIdentity() UUID version = %d, want 7", version)
		}

		if user.Email != email {
			t.Errorf("CreateWithIdentity() email = %q, want %q", user.Email, email)
		}

		if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
			t.Error("CreateWithIdentity() returned zero timestamps")
		}

		var (
			storedUserID         uuid.UUID
			storedProvider       string
			storedProviderUserID string
			identityCreatedAt    time.Time
		)

		err = pool.QueryRow(
			t.Context(),
			`
				SELECT user_id, provider, provider_user_id, created_at
				FROM auth_identities
				WHERE provider = $1 AND provider_user_id = $2
			`,
			provider,
			providerUserID,
		).Scan(&storedUserID, &storedProvider, &storedProviderUserID, &identityCreatedAt)
		if err != nil {
			t.Fatalf("query created auth identity: %v", err)
		}

		if storedUserID != user.ID {
			t.Errorf("identity user ID = %s, want %s", storedUserID, user.ID)
		}

		if storedProvider != provider {
			t.Errorf("identity provider = %q, want %q", storedProvider, provider)
		}

		if storedProviderUserID != providerUserID {
			t.Errorf("identity provider user ID = %q, want %q", storedProviderUserID, providerUserID)
		}

		if identityCreatedAt.IsZero() {
			t.Error("identity has a zero CreatedAt")
		}

		var hasPasswordCredential bool
		if err := pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM password_credentials WHERE user_id = $1)", user.ID).Scan(&hasPasswordCredential); err != nil {
			t.Fatalf("check password credential: %v", err)
		}

		if hasPasswordCredential {
			t.Error("CreateWithIdentity() created an unexpected password credential")
		}
	})

	t.Run("finds user by provider identity", func(t *testing.T) {
		const (
			email          = "find-identity@example.com"
			provider       = "google"
			providerUserID = "google-subject-to-find"
		)

		createdUser, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		user, err := repository.FindUserByProviderIdentity(t.Context(), provider, providerUserID)
		if err != nil {
			t.Fatalf("FindUserByProviderIdentity() error = %v", err)
		}

		if user != createdUser {
			t.Errorf("FindUserByProviderIdentity() user = %+v, want %+v", user, createdUser)
		}
	})

	t.Run("returns not found for unknown provider identity", func(t *testing.T) {
		_, err := repository.FindUserByProviderIdentity(t.Context(), "google", "unknown-google-subject")
		if !errors.Is(err, repositories.ErrAuthIdentityNotFound) {
			t.Fatalf("FindUserByProviderIdentity() error = %v, want %v", err, repositories.ErrAuthIdentityNotFound)
		}
	})

	t.Run("recognizes duplicate email when creating identity", func(t *testing.T) {
		const (
			email          = "identity-duplicate@example.com"
			provider       = "google"
			providerUserID = "duplicate-email-google-subject"
		)

		if _, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$existing-account-hash"); err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		_, err := repository.CreateWithIdentity(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrEmailAlreadyExists) {
			t.Fatalf("CreateWithIdentity() error = %v, want %v", err, repositories.ErrEmailAlreadyExists)
		}

		var identityExists bool
		if err := pool.QueryRow(
			t.Context(),
			"SELECT EXISTS(SELECT 1 FROM auth_identities WHERE provider = $1 AND provider_user_id = $2)",
			provider,
			providerUserID,
		).Scan(&identityExists); err != nil {
			t.Fatalf("check auth identity after duplicate email: %v", err)
		}

		if identityExists {
			t.Error("auth identity was created for an existing email")
		}
	})

	t.Run("rolls back user when identity insert fails", func(t *testing.T) {
		const email = "identity-rollback@example.com"

		_, err := repository.CreateWithIdentity(t.Context(), email, "", "google-subject-with-invalid-provider")
		if err == nil {
			t.Fatal("CreateWithIdentity() error = nil, want identity constraint error")
		}

		var usersWithEmail int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after identity failure: %v", err)
		}

		if usersWithEmail != 0 {
			t.Errorf("users left after identity failure = %d, want 0", usersWithEmail)
		}
	})
}
