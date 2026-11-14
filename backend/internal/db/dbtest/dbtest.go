//go:build integration

package dbtest

import (
	"testing"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
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

	if err := db.Migrate(ctx, connectionString); err != nil {
		t.Fatalf("apply database migrations: %v", err)
	}

	pool, err := db.Connect(ctx, connectionString)

	if err != nil {
		t.Fatalf("open application database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}
