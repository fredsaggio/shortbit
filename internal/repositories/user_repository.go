package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailAlreadyExists = errors.New("email already exists")

type AccountRepository struct {
	db db.DB
}

func NewAccountRepository(db db.DB) *AccountRepository {
	return &AccountRepository{
		db: db,
	}
}

func (r *AccountRepository) CreateWithPassword(ctx context.Context, email, passwordHash string) (models.User, error) {
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

func isEmailConflict(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_users_email"
}
