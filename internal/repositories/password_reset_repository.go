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

// CreatePasswordResetAttempt cria ou substitui uma tentativa para uma conta com
// senha. Um pedido durante o cooldown ou bloqueio não altera o código existente.
// O bool indica apenas se a tentativa foi salva; ele não distingue conta ausente
// de pedido suprimido, para que o service não revele a existência do email.
func (r *UserRepository) CreatePasswordResetAttempt(ctx context.Context, email string, attempt models.PasswordResetAttempt, resendAllowedBefore time.Time) (bool, error) {
	const q = `
		INSERT INTO password_reset_attempts (
			token_hash,
			user_id,
			verification_proof_hash,
			last_code_sent_at,
			code_expires_at,
			attempt_expires_at,
			created_at,
			updated_at
		)
		SELECT
			@tokenHash,
			u.id,
			@proofHash,
			@now,
			@codeExpiresAt,
			@attemptExpiresAt,
			@now,
			@now
		FROM users AS u
		JOIN password_credentials AS pc ON pc.user_id = u.id
		WHERE u.email = @email
		ON CONFLICT (user_id) DO UPDATE
		SET
			token_hash = EXCLUDED.token_hash,
			verification_proof_hash = EXCLUDED.verification_proof_hash,
			failed_attempts = CASE
				WHEN password_reset_attempts.attempt_expires_at <= @now THEN 0
				ELSE password_reset_attempts.failed_attempts
			END,
			locked_until = CASE
				WHEN password_reset_attempts.attempt_expires_at <= @now THEN NULL
				ELSE password_reset_attempts.locked_until
			END,
			last_code_sent_at = @now,
			code_expires_at = CASE
				WHEN password_reset_attempts.attempt_expires_at <= @now THEN EXCLUDED.code_expires_at
				ELSE LEAST(EXCLUDED.code_expires_at, password_reset_attempts.attempt_expires_at)
			END,
			attempt_expires_at = CASE
				WHEN password_reset_attempts.attempt_expires_at <= @now THEN EXCLUDED.attempt_expires_at
				ELSE password_reset_attempts.attempt_expires_at
			END,
			used_at = NULL,
			created_at = CASE
				WHEN password_reset_attempts.attempt_expires_at <= @now THEN @now
				ELSE password_reset_attempts.created_at
			END,
			updated_at = @now
		WHERE password_reset_attempts.attempt_expires_at <= @now
			OR (
				password_reset_attempts.last_code_sent_at <= @resendAllowedBefore
				AND (password_reset_attempts.locked_until IS NULL OR password_reset_attempts.locked_until <= @now)
			)
	`

	args := pgx.StrictNamedArgs{
		"email":               email,
		"tokenHash":           attempt.TokenHash,
		"proofHash":           attempt.VerificationProofHash,
		"now":                 attempt.LastCodeSentAt,
		"codeExpiresAt":       attempt.CodeExpiresAt,
		"attemptExpiresAt":    attempt.AttemptExpiresAt,
		"resendAllowedBefore": resendAllowedBefore,
	}

	result, err := r.db.Exec(ctx, q, args)
	if err != nil {
		return false, fmt.Errorf("create password reset attempt: %w", err)
	}

	return result.RowsAffected() == 1, nil
}

func (r *UserRepository) FindPasswordResetAttemptByTokenHash(ctx context.Context, tokenHash []byte) (models.PasswordResetAttempt, error) {
	const q = `
		SELECT token_hash, user_id, verification_proof_hash, failed_attempts,
			locked_until, last_code_sent_at, code_expires_at, attempt_expires_at,
			used_at, created_at, updated_at
		FROM password_reset_attempts
		WHERE token_hash = @tokenHash
	`

	var attempt models.PasswordResetAttempt
	err := r.db.QueryRow(ctx, q, pgx.StrictNamedArgs{"tokenHash": tokenHash}).Scan(
		&attempt.TokenHash,
		&attempt.UserID,
		&attempt.VerificationProofHash,
		&attempt.FailedAttempts,
		&attempt.LockedUntil,
		&attempt.LastCodeSentAt,
		&attempt.CodeExpiresAt,
		&attempt.AttemptExpiresAt,
		&attempt.UsedAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.PasswordResetAttempt{}, ErrPasswordResetAttemptNotFound
		}
		return models.PasswordResetAttempt{}, fmt.Errorf("find password reset attempt by token hash: %w", err)
	}

	return attempt, nil
}

func (r *UserRepository) RecordPasswordResetFailure(ctx context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
	const q = `
		UPDATE password_reset_attempts
		SET
			failed_attempts = CASE
				WHEN locked_until IS NOT NULL AND locked_until > @now THEN failed_attempts
				WHEN failed_attempts + 1 >= @maxAttempts THEN 0
				ELSE failed_attempts + 1
			END,
			locked_until = CASE
				WHEN locked_until IS NOT NULL AND locked_until > @now THEN locked_until
				WHEN failed_attempts + 1 >= @maxAttempts THEN LEAST(@lockUntil, attempt_expires_at)
				ELSE LEAST(@retryAt, attempt_expires_at)
			END,
			updated_at = @now
		WHERE token_hash = @tokenHash
			AND attempt_expires_at > @now
			AND used_at IS NULL
		RETURNING failed_attempts, locked_until
	`

	args := pgx.StrictNamedArgs{
		"tokenHash":   tokenHash,
		"now":         now,
		"retryAt":     retryAt,
		"lockUntil":   lockUntil,
		"maxAttempts": maxAttempts,
	}

	var failedAttempts int16
	var lockedUntil *time.Time
	if err := r.db.QueryRow(ctx, q, args).Scan(&failedAttempts, &lockedUntil); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, ErrPasswordResetAttemptNotFound
		}
		return 0, nil, fmt.Errorf("record password reset failure: %w", err)
	}

	return failedAttempts, lockedUntil, nil
}

// CompletePasswordReset revalida a tentativa sob lock antes de alterar a
// credencial. Consumir o código, trocar a senha e revogar sessões são uma única
// transação: um erro em qualquer etapa desfaz todas as alterações.
func (r *UserRepository) CompletePasswordReset(ctx context.Context, tokenHash, proofHash []byte, passwordHash string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin password reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const findAttempt = `
		SELECT a.user_id, u.email
		FROM password_reset_attempts AS a
		JOIN users AS u ON u.id = a.user_id
		WHERE a.token_hash = @tokenHash
			AND a.verification_proof_hash = @proofHash
			AND a.used_at IS NULL
			AND a.attempt_expires_at > clock_timestamp()
			AND a.code_expires_at > clock_timestamp()
			AND (a.locked_until IS NULL OR a.locked_until <= clock_timestamp())
		FOR UPDATE OF a
	`
	var userID uuid.UUID
	var email string
	if err := tx.QueryRow(ctx, findAttempt, pgx.StrictNamedArgs{"tokenHash": tokenHash, "proofHash": proofHash}).Scan(&userID, &email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPasswordResetAttemptNotFound
		}
		return "", fmt.Errorf("lock password reset attempt: %w", err)
	}

	const updatePassword = `
		UPDATE password_credentials
		SET password_hash = @passwordHash, updated_at = clock_timestamp()
		WHERE user_id = @userID
	`
	updated, err := tx.Exec(ctx, updatePassword, pgx.StrictNamedArgs{"userID": userID, "passwordHash": passwordHash})
	if err != nil {
		return "", fmt.Errorf("update password credential: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return "", ErrPasswordAccountNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id = @userID`, pgx.StrictNamedArgs{"userID": userID}); err != nil {
		return "", fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE password_reset_attempts SET used_at = clock_timestamp(), updated_at = clock_timestamp() WHERE token_hash = @tokenHash`, pgx.StrictNamedArgs{"tokenHash": tokenHash}); err != nil {
		return "", fmt.Errorf("consume password reset attempt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit password reset transaction: %w", err)
	}
	return email, nil
}
