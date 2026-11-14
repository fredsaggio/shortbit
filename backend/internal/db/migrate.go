package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/fredsaggio/url-shortener/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate applies the embedded migrations that are still pending. A Postgres
// advisory lock makes concurrent callers, such as two instances starting
// during a deploy, run them one at a time; the second finds nothing to do.
// The lock lives in the session, so the URL must reach Postgres directly, not
// through a transaction pooler such as PgBouncer.
func Migrate(ctx context.Context, connStr string) error {
	database, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer database.Close()

	if err := database.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrations.Migrations, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	for _, result := range results {
		slog.InfoContext(ctx, "migration applied", "version", result.Source.Version, "duration_ms", result.Duration.Milliseconds())
	}

	return nil
}
