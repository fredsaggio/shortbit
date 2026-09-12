package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresImage = "postgres:18"

	databaseName     = "url_shortener_test"
	databaseUser     = "postgres"
	databasePassword = "password"
)

func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := t.Context()

	postgresContainer, err := postgres.Run(ctx, postgresImage, postgres.WithDatabase(databaseName), postgres.WithUsername(databaseUser), postgres.WithPassword(databasePassword), postgres.BasicWaitStrategies())

	testcontainers.CleanupContainer(t, postgresContainer)

	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}

	connectionString, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")

	if err != nil {
		t.Fatalf("get PostgreSQL connection string: %v", err)
	}

	if err := applyMigrations(ctx, connectionString); err != nil {
		t.Fatalf("apply database migrations: %v", err)
	}

	pool, err := db.Connect(ctx, connectionString)

	if err != nil {
		t.Fatalf("open application database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func applyMigrations(ctx context.Context, connectionString string) error {
	migrationDB, err := sql.Open("pgx", connectionString)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}

	defer migrationDB.Close()

	if err := migrationDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.Migrations)

	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	
	return nil
}
