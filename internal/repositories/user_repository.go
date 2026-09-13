package repositories

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrEmailAlreadyExists         = errors.New("email already exists")
	ErrPasswordCredentialNotFound = errors.New("password credential not found")
	ErrUserNotFound               = errors.New("user not found")
)

type UserRepository struct {
	db db.DB
}

func NewUserRepository(db db.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
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
			   INSERT INTO users(email)
			   VALUES (@email)
			   RETURNING id, email, created_at, updated_at
			   `

	args := pgx.StrictNamedArgs{
		"email": email,
	}

	var user models.User

	err = tx.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if isEmailConflict(err) {
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

	_, err = tx.Exec(ctx, q2, args2)

	if err != nil {
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

func isEmailConflict(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_users_email"
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	const q = `
		SELECT id, email, created_at, updated_at
		FROM users
		WHERE id = @userID
	`

	args := pgx.StrictNamedArgs{
		"userID": userID,
	}

	var user models.User

	if err := r.db.QueryRow(ctx, q, args).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, fmt.Errorf("find user by ID: %w", err)
	}

	return user, nil
}
