package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

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
