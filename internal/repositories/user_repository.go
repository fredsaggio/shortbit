package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrEmailAlreadyExists           = errors.New("email already exists")
	ErrPasswordCredentialNotFound   = errors.New("password credential not found")
	ErrUserNotFound                 = errors.New("user not found")
	ErrAuthIdentityNotFound         = errors.New("auth identity not found")
	ErrRegistrationAttemptNotFound  = errors.New("registration attempt not found")
	ErrPasswordResetAttemptNotFound = errors.New("password reset attempt not found")
	ErrPasswordAccountNotFound      = errors.New("account not found")

	ErrAuthIdentityAlreadyExists = errors.New("auth identity already exists")
	ErrProviderAlreadyLinked     = errors.New("provider already linked")
)

type UserRepository struct {
	db db.DB
}

func NewUserRepository(db db.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

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

func (r *UserRepository) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	const q = `
		SELECT EXISTS (
			SELECT 1
			FROM USERS
			WHERE email = @email
		)
	`

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	var exists bool

	if err := r.db.QueryRow(ctx, q, args).Scan(&exists); err != nil {
		return false, fmt.Errorf("check user existence with email: %w", err)
	}

	return exists, nil
}

func (r *UserRepository) CreateWithPassword(ctx context.Context, email, passwordHash string) (models.User, error) {
	tx, err := r.db.Begin(ctx)

	if err != nil {
		return models.User{}, fmt.Errorf("begin create acount transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
			   INSERT INTO users(email, email_verified_at)
			   VALUES (@email, NOW())
			   RETURNING id, email, email_verified_at, created_at, updated_at
			   `

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	var user models.User

	err = tx.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if isConstraintConflict(err, "uq_users_email") {
			return models.User{}, ErrEmailAlreadyExists
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}

	const q2 = `
				INSERT INTO password_credentials(user_id, password_hash)
				VALUES(@userID, @passwordHash)
			   `

	args2 := pgx.StrictNamedArgs{
		"userID":       user.ID,
		"passwordHash": passwordHash,
	}

	// A gente usa underline aqui porque a primeira variável retorna quantas linhas foram afetadas, e não quero saber isso, só se o exec deu certo.
	if _, err = tx.Exec(ctx, q2, args2); err != nil {
		return models.User{}, fmt.Errorf("insert password credential: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.User{}, fmt.Errorf("commit create account transaction: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindPasswordCredentialsByEmail(
	ctx context.Context,
	email string,
) (models.User, models.PasswordCredential, error) {
	const q = `
		SELECT
			u.id,
			u.email,
			u.email_verified_at,
			u.created_at,
			u.updated_at,
			pc.user_id,
			pc.password_hash,
			pc.created_at,
			pc.updated_at
		FROM users AS u
		JOIN password_credentials AS pc
			ON pc.user_id = u.id
		WHERE u.email = @email
	`

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	var (
		user       models.User
		credential models.PasswordCredential
	)

	err := r.db.QueryRow(ctx, q, args).Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&credential.UserID,
		&credential.PasswordHash,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, models.PasswordCredential{}, ErrPasswordCredentialNotFound
		}

		return models.User{}, models.PasswordCredential{}, fmt.Errorf(
			"find password credentials by email: %w",
			err,
		)
	}

	return user, credential, nil
}

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

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	const q = `
		SELECT id, email, email_verified_at, created_at, updated_at
		FROM users
		WHERE id = @userID
	`

	args := pgx.StrictNamedArgs{
		"userID": userID,
	}

	var user models.User

	if err := r.db.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, fmt.Errorf("find user by ID: %w", err)
	}

	return user, nil
}

func (r *UserRepository) CreateWithIdentity(ctx context.Context, email, provider, providerUserID string) (models.User, error) {
	tx, err := r.db.Begin(ctx)

	if err != nil {
		return models.User{}, fmt.Errorf("create transaction to save user and provider: %w", err)
	}

	// A gente não precisa desse erro: Se tiver dado erro antes, vamos tratar antes. Se tiver dado commit, isso geraria um erro desnecessário.
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
		INSERT INTO users(email, email_verified_at)
		VALUES(@email, NOW())
		RETURNING id, email, email_verified_at, created_at, updated_at
	`

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	var user models.User

	if err := tx.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if isConstraintConflict(err, "uq_users_email") {
			return models.User{}, ErrEmailAlreadyExists
		}
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}

	const q2 = `
		INSERT INTO auth_identities(user_id, provider, provider_user_id)
		VALUES(@userID, @provider, @providerUserID)
	`

	args2 := pgx.StrictNamedArgs{
		"userID":         user.ID,
		"provider":       provider,
		"providerUserID": providerUserID,
	}

	if _, err = tx.Exec(ctx, q2, args2); err != nil {
		return models.User{}, fmt.Errorf("insert provider: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.User{}, fmt.Errorf("commit create account transaction with provider: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (models.User, error) {
	const q = `
		SELECT
			u.id,
			u.email,
			u.email_verified_at,
			u.created_at,
			u.updated_at
		FROM auth_identities AS ai
		JOIN users AS u ON u.id = ai.user_id
		WHERE ai.provider = @provider
			AND ai.provider_user_id = @providerUserID
	`

	args := pgx.StrictNamedArgs{
		"provider":       provider,
		"providerUserID": providerUserID,
	}

	var user models.User

	if err := r.db.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrAuthIdentityNotFound
		}
		return models.User{}, fmt.Errorf("find user by provider: %w", err)
	}

	return user, nil
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

func (r *UserRepository) LinkIdentityToPasswordUserByEmail(ctx context.Context, email, provider, providerUserID string) (models.User, error) {
	tx, err := r.db.Begin(ctx)

	if err != nil {
		return models.User{}, fmt.Errorf("begin link identity transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q1 = `
		SELECT
			u.id,
			u.email,
			u.email_verified_at,
			u.created_at,
			u.updated_at
		FROM users AS u
		JOIN password_credentials AS pc ON pc.user_id = u.id
		WHERE u.email = @email
		FOR UPDATE OF u
	`

	var user models.User

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	err = tx.QueryRow(ctx, q1, args).Scan(&user.ID, &user.Email, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrPasswordAccountNotFound
		}

		return models.User{}, fmt.Errorf("find password account for identity linking: %w", err)
	}

	const q2 = `
		INSERT INTO auth_identities(user_id, provider, provider_user_id)
		VALUES(@userID, @provider, @providerUserID)
	`

	args2 := pgx.StrictNamedArgs{
		"userID":         user.ID,
		"provider":       provider,
		"providerUserID": providerUserID,
	}

	if _, err := tx.Exec(ctx, q2, args2); err != nil {
		switch {
		case isConstraintConflict(err, "pk_auth_identities"):
			return models.User{}, ErrAuthIdentityAlreadyExists

		case isConstraintConflict(err, "uq_auth_identities_user_provider"):
			return models.User{}, ErrProviderAlreadyLinked

		default:
			return models.User{}, fmt.Errorf("insert identity for password account: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.User{}, fmt.Errorf("commit identity linking transaction: %w", err)
	}

	return user, nil
}

func isConstraintConflict(err error, constraintName string) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
}
