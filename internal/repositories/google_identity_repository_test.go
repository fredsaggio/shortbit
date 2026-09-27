//go:build integration

package repositories_test

import (
	"errors"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"time"
	"uuid"
)

func runGoogleIdentityRepositoryTests(t *testing.T, pool *pgxpool.Pool, repository *repositories.UserRepository) {
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

	t.Run("links identity to existing password user", func(t *testing.T) {
		const (
			email          = "link-identity-password-user@example.com"
			provider       = "google"
			providerUserID = "link-identity-google-subject"
			passwordHash   = "$argon2id$link-identity-password-hash"
		)

		passwordUser, err := repository.CreateWithPassword(t.Context(), email, passwordHash)
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		linkedUser, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if err != nil {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		if linkedUser != passwordUser {
			t.Errorf("LinkIdentityToPasswordUserByEmail() user = %+v, want %+v", linkedUser, passwordUser)
		}

		var (
			usersWithEmail       int
			storedIdentityUserID uuid.UUID
			storedPasswordHash   string
		)

		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&usersWithEmail); err != nil {
			t.Fatalf("count users after identity linking: %v", err)
		}
		if usersWithEmail != 1 {
			t.Errorf("users with linked email = %d, want 1", usersWithEmail)
		}

		if err := pool.QueryRow(
			t.Context(),
			"SELECT user_id FROM auth_identities WHERE provider = $1 AND provider_user_id = $2",
			provider,
			providerUserID,
		).Scan(&storedIdentityUserID); err != nil {
			t.Fatalf("query linked identity: %v", err)
		}
		if storedIdentityUserID != passwordUser.ID {
			t.Errorf("linked identity user ID = %s, want %s", storedIdentityUserID, passwordUser.ID)
		}

		if err := pool.QueryRow(
			t.Context(),
			"SELECT password_hash FROM password_credentials WHERE user_id = $1",
			passwordUser.ID,
		).Scan(&storedPasswordHash); err != nil {
			t.Fatalf("query password credential after identity linking: %v", err)
		}
		if storedPasswordHash != passwordHash {
			t.Errorf("password hash after identity linking = %q, want %q", storedPasswordHash, passwordHash)
		}
	})

	t.Run("returns password account not found for unknown email", func(t *testing.T) {
		const (
			email          = "unknown-password-account@example.com"
			provider       = "google"
			providerUserID = "unknown-password-account-google-subject"
		)

		_, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrPasswordAccountNotFound) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrPasswordAccountNotFound)
		}

		var identityExists bool
		if err := pool.QueryRow(
			t.Context(),
			"SELECT EXISTS(SELECT 1 FROM auth_identities WHERE provider = $1 AND provider_user_id = $2)",
			provider,
			providerUserID,
		).Scan(&identityExists); err != nil {
			t.Fatalf("check identity for unknown password account: %v", err)
		}
		if identityExists {
			t.Error("identity was created for an unknown password account")
		}
	})

	t.Run("does not link identity to Google-only user", func(t *testing.T) {
		const (
			email                  = "google-only-linking@example.com"
			provider               = "google"
			existingProviderUserID = "existing-google-only-subject"
			newProviderUserID      = "new-google-only-subject"
		)

		googleUser, err := repository.CreateWithIdentity(t.Context(), email, provider, existingProviderUserID)
		if err != nil {
			t.Fatalf("CreateWithIdentity() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, newProviderUserID)
		if !errors.Is(err, repositories.ErrPasswordAccountNotFound) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrPasswordAccountNotFound)
		}

		var identities int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM auth_identities WHERE user_id = $1", googleUser.ID).Scan(&identities); err != nil {
			t.Fatalf("count identities for Google-only user: %v", err)
		}
		if identities != 1 {
			t.Errorf("Google-only user identities = %d, want 1", identities)
		}
	})

	t.Run("recognizes identity that is already linked", func(t *testing.T) {
		const (
			email          = "identity-already-linked@example.com"
			provider       = "google"
			providerUserID = "already-linked-google-subject"
		)

		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$already-linked-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if _, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID); err != nil {
			t.Fatalf("first LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, providerUserID)
		if !errors.Is(err, repositories.ErrAuthIdentityAlreadyExists) {
			t.Fatalf("second LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrAuthIdentityAlreadyExists)
		}

		var identities int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM auth_identities WHERE user_id = $1", user.ID).Scan(&identities); err != nil {
			t.Fatalf("count identities after repeated linking: %v", err)
		}
		if identities != 1 {
			t.Errorf("identities after repeated linking = %d, want 1", identities)
		}
	})

	t.Run("does not replace provider identity already linked to user", func(t *testing.T) {
		const (
			email                  = "provider-already-linked@example.com"
			provider               = "google"
			originalProviderUserID = "original-linked-google-subject"
			newProviderUserID      = "replacement-google-subject"
		)

		user, err := repository.CreateWithPassword(t.Context(), email, "$argon2id$provider-linked-password-hash")
		if err != nil {
			t.Fatalf("CreateWithPassword() error = %v", err)
		}

		if _, err := repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, originalProviderUserID); err != nil {
			t.Fatalf("first LinkIdentityToPasswordUserByEmail() error = %v", err)
		}

		_, err = repository.LinkIdentityToPasswordUserByEmail(t.Context(), email, provider, newProviderUserID)
		if !errors.Is(err, repositories.ErrProviderAlreadyLinked) {
			t.Fatalf("LinkIdentityToPasswordUserByEmail() error = %v, want %v", err, repositories.ErrProviderAlreadyLinked)
		}

		var storedProviderUserID string
		if err := pool.QueryRow(
			t.Context(),
			"SELECT provider_user_id FROM auth_identities WHERE user_id = $1 AND provider = $2",
			user.ID,
			provider,
		).Scan(&storedProviderUserID); err != nil {
			t.Fatalf("query identity after rejected replacement: %v", err)
		}
		if storedProviderUserID != originalProviderUserID {
			t.Errorf("stored provider user ID = %q, want %q", storedProviderUserID, originalProviderUserID)
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
