//go:build integration

package db_test

import (
	"sync"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMigrateIsSafeToRunConcurrentlyAndAgain(t *testing.T) {
	ctx := t.Context()

	container, err := postgres.Run(ctx, "postgres:18", postgres.WithDatabase("migrate_test"), postgres.WithUsername("postgres"), postgres.WithPassword("password"), postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	// Two instances starting together during a deploy.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Go(func() { errs[i] = db.Migrate(ctx, connStr) })
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Migrate() #%d error = %v", i+1, err)
		}
	}

	// A restart finds nothing to apply.
	if err := db.Migrate(ctx, connStr); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	pool, err := db.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	var users int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		t.Fatalf("query migrated schema: %v", err)
	}
}
