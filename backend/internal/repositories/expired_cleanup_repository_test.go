//go:build integration

package repositories_test

import (
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

func TestExpiredRecordsRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	ctx := t.Context()
	now := time.Now().UTC()

	var userID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, email_verified_at) VALUES ('cleanup@example.com', NOW()) RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var linkID int64
	if err := pool.QueryRow(ctx, `INSERT INTO urls (short_code, user_id, original_url, visibility, password_hash) VALUES ('Clean01', $1, 'https://example.com', 'private', 'hash') RETURNING id`, userID).Scan(&linkID); err != nil {
		t.Fatalf("insert link: %v", err)
	}

	insert := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, query, args...); err != nil {
			t.Fatalf("insert fixture: %v", err)
		}
	}
	hash := func() []byte {
		_, h, err := sessiontoken.Generate()
		if err != nil {
			t.Fatalf("generate token: %v", err)
		}
		return h
	}

	// Three expired and one valid row per session table.
	for range 3 {
		insert(`INSERT INTO user_sessions (token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`, hash(), userID, now.Add(-2*time.Hour), now.Add(-time.Hour))
		insert(`INSERT INTO link_access_sessions (token_hash, link_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`, hash(), linkID, now.Add(-2*time.Hour), now.Add(-time.Hour))
	}
	insert(`INSERT INTO user_sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, hash(), userID, now.Add(time.Hour))
	insert(`INSERT INTO link_access_sessions (token_hash, link_id, expires_at) VALUES ($1, $2, $3)`, hash(), linkID, now.Add(time.Hour))

	repo := repositories.NewExpiredRecordsRepository(pool)

	// Batches of two: the first is full, the second takes the rest.
	first, err := repo.DeleteExpiredBatch(ctx, "user_sessions", now, 2)
	if err != nil || first != 2 {
		t.Fatalf("first batch = %d, %v; want 2", first, err)
	}
	second, err := repo.DeleteExpiredBatch(ctx, "user_sessions", now, 2)
	if err != nil || second != 1 {
		t.Fatalf("second batch = %d, %v; want 1", second, err)
	}
	links, err := repo.DeleteExpiredBatch(ctx, "link_access_sessions", now, 1000)
	if err != nil || links != 3 {
		t.Fatalf("link access batch = %d, %v; want 3", links, err)
	}
	if _, err := repo.DeleteExpiredBatch(ctx, "password_reset_attempts", now, 1000); err != nil {
		t.Fatalf("password reset attempts: %v", err)
	}

	for table, want := range map[string]int{"user_sessions": 1, "link_access_sessions": 1} {
		var left int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table).Scan(&left); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if left != want {
			t.Errorf("%s left = %d, want only the valid one", table, left)
		}
	}

	if _, err := repo.DeleteExpiredBatch(ctx, "users", now, 10); err == nil {
		t.Error("DeleteExpiredBatch(users) error = nil, want a refusal for an unknown table")
	}
}
