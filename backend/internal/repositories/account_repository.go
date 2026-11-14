package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

func (r *UserRepository) FindAccountByID(ctx context.Context, userID uuid.UUID) (models.Account, error) {
	const q = `
		SELECT
			u.id,
			u.email,
			u.full_name,
			u.email_verified_at,
			u.created_at,
			u.updated_at,
			EXISTS (SELECT 1 FROM password_credentials AS pc WHERE pc.user_id = u.id),
			EXISTS (SELECT 1 FROM auth_identities AS ai WHERE ai.user_id = u.id AND ai.provider = 'google')
		FROM users AS u
		WHERE u.id = @userID
	`

	var account models.Account

	err := r.db.QueryRow(ctx, q, pgx.StrictNamedArgs{"userID": userID}).Scan(
		&account.ID,
		&account.Email,
		&account.FullName,
		&account.EmailVerifiedAt,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.HasPassword,
		&account.HasGoogle,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Account{}, ErrUserNotFound
		}
		return models.Account{}, fmt.Errorf("find account by ID: %w", err)
	}

	return account, nil
}

func (r *UserRepository) UpdateFullName(ctx context.Context, userID uuid.UUID, fullName string) error {
	const q = `
		UPDATE users
		SET full_name = @fullName
		WHERE id = @userID
	`

	updated, err := r.db.Exec(ctx, q, pgx.StrictNamedArgs{"userID": userID, "fullName": fullName})
	if err != nil {
		return fmt.Errorf("update full name: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return ErrUserNotFound
	}

	return nil
}

func (r *UserRepository) FindPasswordHashByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	const q = `
		SELECT password_hash
		FROM password_credentials
		WHERE user_id = @userID
	`

	var passwordHash string
	if err := r.db.QueryRow(ctx, q, pgx.StrictNamedArgs{"userID": userID}).Scan(&passwordHash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPasswordCredentialNotFound
		}
		return "", fmt.Errorf("find password hash by user ID: %w", err)
	}

	return passwordHash, nil
}

// ReplacePasswordAndSessions changes the password, revokes every session of
// the account and creates the one that replaces the caller's session, all in
// one transaction: a password change also renews the session ID.
func (r *UserRepository) ReplacePasswordAndSessions(ctx context.Context, userID uuid.UUID, passwordHash string, sessionTokenHash []byte, sessionExpiresAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password change transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const updatePassword = `
		UPDATE password_credentials
		SET password_hash = @passwordHash
		WHERE user_id = @userID
	`
	updated, err := tx.Exec(ctx, updatePassword, pgx.StrictNamedArgs{"userID": userID, "passwordHash": passwordHash})
	if err != nil {
		return fmt.Errorf("update password credential: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return ErrPasswordCredentialNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id = @userID`, pgx.StrictNamedArgs{"userID": userID}); err != nil {
		return fmt.Errorf("revoke sessions after password change: %w", err)
	}

	const createSession = `
		INSERT INTO user_sessions (token_hash, user_id, expires_at)
		VALUES (@tokenHash, @userID, @expiresAt)
	`
	if _, err := tx.Exec(ctx, createSession, pgx.StrictNamedArgs{"tokenHash": sessionTokenHash, "userID": userID, "expiresAt": sessionExpiresAt}); err != nil {
		return fmt.Errorf("create session after password change: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change transaction: %w", err)
	}

	return nil
}
