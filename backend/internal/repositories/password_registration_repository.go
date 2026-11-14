package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

func (r *UserRepository) CreatePasswordRegistrationAttempt(ctx context.Context, attempt models.PasswordRegistrationAttempt) error {
	const q = `
		INSERT INTO password_registration_attempts (
			token_hash,
			email,
			password_hash,
			verification_proof_hash,
			last_code_sent_at,
			code_expires_at,
			attempt_expires_at
		)
		VALUES (
			@tokenHash,
			@email,
			@passwordHash,
			@verificationProofHash,
			@lastCodeSentAt,
			@codeExpiresAt,
			@attemptExpiresAt
		)
	`

	args := pgx.StrictNamedArgs{
		"tokenHash":             attempt.TokenHash,
		"email":                 attempt.Email,
		"passwordHash":          attempt.PasswordHash,
		"verificationProofHash": attempt.VerificationProofHash,
		"lastCodeSentAt":        attempt.LastCodeSentAt,
		"codeExpiresAt":         attempt.CodeExpiresAt,
		"attemptExpiresAt":      attempt.AttemptExpiresAt,
	}

	if _, err := r.db.Exec(ctx, q, args); err != nil {
		return fmt.Errorf("create password registration attempt: %w", err)
	}

	return nil
}

func (r *UserRepository) FindPasswordRegistrationAttemptByTokenHash(ctx context.Context, tokenHash []byte) (models.PasswordRegistrationAttempt, error) {
	const q = `
		SELECT
		token_hash,
		email,
		password_hash,
		verification_proof_hash,
		failed_attempts,
		locked_until,
		last_code_sent_at,
		code_expires_at,
		attempt_expires_at,
		created_at,
		updated_at
	FROM password_registration_attempts
	WHERE token_hash = @tokenHash
	`

	args := pgx.StrictNamedArgs{
		"tokenHash": tokenHash,
	}

	var attempt models.PasswordRegistrationAttempt

	err := r.db.QueryRow(ctx, q, args).Scan(&attempt.TokenHash,
		&attempt.Email,
		&attempt.PasswordHash,
		&attempt.VerificationProofHash,
		&attempt.FailedAttempts,
		&attempt.LockedUntil,
		&attempt.LastCodeSentAt,
		&attempt.CodeExpiresAt,
		&attempt.AttemptExpiresAt,
		&attempt.CreatedAt,
		&attempt.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.PasswordRegistrationAttempt{}, ErrRegistrationAttemptNotFound
		}
		return models.PasswordRegistrationAttempt{}, fmt.Errorf("find password registration attempt by token hash: %w", err)
	}

	return attempt, nil
}

func (r *UserRepository) RecordPasswordRegistrationFailure(ctx context.Context, tokenHash []byte, now, retryAt, lockUntil time.Time, maxAttempts int16) (int16, *time.Time, error) {
	const q = `
		UPDATE password_registration_attempts
		SET
			failed_attempts = CASE
				WHEN locked_until IS NOT NULL AND locked_until > @now
					THEN failed_attempts
				WHEN failed_attempts + 1 >= @maxAttempts
					THEN 0
				ELSE failed_attempts + 1
			END,
			locked_until = CASE
				WHEN locked_until IS NOT NULL AND locked_until > @now
					THEN locked_until
				WHEN failed_attempts + 1 >= @maxAttempts
					THEN LEAST(@lockUntil, attempt_expires_at)
				ELSE LEAST(@retryAt, attempt_expires_at)
			END
		WHERE
			token_hash = @tokenHash
			AND attempt_expires_at > @now
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
			return 0, nil, ErrRegistrationAttemptNotFound
		}

		return 0, nil, fmt.Errorf("record password registration failure: %w", err)
	}

	return failedAttempts, lockedUntil, nil
}

func (r *UserRepository) UpdatePasswordRegistrationCode(ctx context.Context, tokenHash, proofHash []byte, now, codeExpiresAt, resendAllowedBefore time.Time) (bool, error) {
	const q = `
		UPDATE password_registration_attempts
		SET
			verification_proof_hash = @proofHash,
			last_code_sent_at = @now,
			code_expires_at = LEAST(@codeExpiresAt, attempt_expires_at)
		WHERE 
			token_hash = @tokenHash
			AND attempt_expires_at > @now
			AND (locked_until IS NULL OR locked_until <= @now)
			AND last_code_sent_at <= @resendAllowedBefore
	`

	args := pgx.StrictNamedArgs{
		"tokenHash":           tokenHash,
		"proofHash":           proofHash,
		"now":                 now,
		"codeExpiresAt":       codeExpiresAt,
		"resendAllowedBefore": resendAllowedBefore,
	}

	result, err := r.db.Exec(ctx, q, args)

	if err != nil {
		return false, fmt.Errorf("update password registration code: %w", err)
	}

	return result.RowsAffected() == 1, nil
}

func (r *UserRepository) DeleteObsoletePasswordRegistrationAttempts(ctx context.Context, now time.Time) (int64, error) {
	const q = `
		DELETE FROM password_registration_attempts AS attempt
		WHERE
			attempt.attempt_expires_at <= @now
			OR EXISTS (
				SELECT 1
				FROM users
				WHERE users.email = attempt.email
			)
	`

	args := pgx.StrictNamedArgs{
		"now": now,
	}

	result, err := r.db.Exec(ctx, q, args)
	if err != nil {
		return 0, fmt.Errorf("delete obsolete password registration attempts: %w", err)
	}

	return result.RowsAffected(), nil
}
