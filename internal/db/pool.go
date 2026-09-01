package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ DB = (*pgxpool.Pool)(nil)

func Connect(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		defer pool.Close()
		return nil, err
	}

	return pool, nil
}
