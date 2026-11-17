package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/jackc/pgx/v5"
)

type VisitorSaltRepository struct {
	db db.DB
}

func NewVisitorSaltRepository(db db.DB) *VisitorSaltRepository {
	return &VisitorSaltRepository{db: db}
}

// DailySalt stores candidate as the salt of day unless the day has one, and
// returns the day's salt. Every API instance gets the same salt.
func (r *VisitorSaltRepository) DailySalt(ctx context.Context, day time.Time, candidate []byte) ([]byte, error) {
	const insert = `
		INSERT INTO visitor_salts (day, salt)
		VALUES (@day::date, @salt)
		ON CONFLICT (day) DO NOTHING
	`
	args := pgx.StrictNamedArgs{"day": day.Format(time.DateOnly), "salt": candidate}
	if _, err := r.db.Exec(ctx, insert, args); err != nil {
		return nil, fmt.Errorf("store visitor salt: %w", err)
	}

	// A separate statement sees the salt another instance just committed,
	// which a SELECT in the same statement as the INSERT would not.
	const read = `SELECT salt FROM visitor_salts WHERE day = @day::date`
	var salt []byte
	if err := r.db.QueryRow(ctx, read, pgx.StrictNamedArgs{"day": day.Format(time.DateOnly)}).Scan(&salt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("visitor salt missing right after storing it")
		}
		return nil, fmt.Errorf("read visitor salt: %w", err)
	}
	return salt, nil
}
