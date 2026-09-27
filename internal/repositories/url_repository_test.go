//go:build integration

package repositories_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestURLRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	userRepository := repositories.NewUserRepository(pool)
	urlRepository := repositories.NewURLRepository(pool)

	user, err := userRepository.CreateWithPassword(t.Context(), "url-repository@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}

	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	t.Run("reserves distinct IDs", func(t *testing.T) {
		first, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("first ReserveID() error = %v", err)
		}

		second, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("second ReserveID() error = %v", err)
		}

		if first <= 0 || second <= first {
			t.Errorf("reserved IDs = (%d, %d), want positive increasing IDs", first, second)
		}
	})

	t.Run("creates a public URL with an explicitly reserved identity ID", func(t *testing.T) {
		id, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}

		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}

		want := models.URL{
			ID:          id,
			ShortCode:   code,
			UserID:      user.ID,
			OriginalURL: "https://example.com/public",
			Visibility:  models.VisibilityPublic,
		}

		created, err := urlRepository.Create(t.Context(), want)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if created.ID != want.ID || created.ShortCode != want.ShortCode || created.UserID != want.UserID ||
			created.OriginalURL != want.OriginalURL || created.Visibility != want.Visibility {
			t.Errorf("Create() = %+v, want URL fields %+v", created, want)
		}
		if created.PasswordHash != nil {
			t.Errorf("public URL password hash = %q, want nil", *created.PasswordHash)
		}
		if created.ClickCount != 0 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
			t.Errorf("Create() defaults = %+v, want zero clicks and nonzero timestamps", created)
		}

		var storedCode string
		if err := pool.QueryRow(t.Context(), "SELECT short_code FROM urls WHERE id = $1", id).Scan(&storedCode); err != nil {
			t.Fatalf("find created URL by reserved ID: %v", err)
		}
		if storedCode != code {
			t.Errorf("stored shortcode = %q, want %q", storedCode, code)
		}
	})

	t.Run("creates a private URL with a password hash", func(t *testing.T) {
		id, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}

		passwordHash := "$argon2id$private-link-test-hash"
		created, err := urlRepository.Create(t.Context(), models.URL{
			ID:           id,
			ShortCode:    code,
			UserID:       user.ID,
			OriginalURL:  "https://example.com/private",
			Visibility:   models.VisibilityPrivate,
			PasswordHash: &passwordHash,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if created.PasswordHash == nil || *created.PasswordHash != passwordHash {
			t.Errorf("private URL password hash = %v, want %q", created.PasswordHash, passwordHash)
		}
		if created.Visibility != models.VisibilityPrivate || created.ID != id || created.ShortCode != code {
			t.Errorf("Create() = %+v, want private URL with reserved ID and code", created)
		}
	})

	t.Run("rejects a duplicate shortcode without creating a second URL", func(t *testing.T) {
		firstID, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("first ReserveID() error = %v", err)
		}
		code, err := generator.Generate(firstID)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", firstID, err)
		}
		first := models.URL{
			ID: firstID, ShortCode: code, UserID: user.ID,
			OriginalURL: "https://example.com/first", Visibility: models.VisibilityPublic,
		}
		if _, err := urlRepository.Create(t.Context(), first); err != nil {
			t.Fatalf("first Create() error = %v", err)
		}

		secondID, err := urlRepository.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("second ReserveID() error = %v", err)
		}
		second := models.URL{
			ID: secondID, ShortCode: code, UserID: user.ID,
			OriginalURL: "https://example.com/second", Visibility: models.VisibilityPublic,
		}
		_, err = urlRepository.Create(t.Context(), second)

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "uq_urls_short_code" {
			t.Fatalf("duplicate Create() error = %v, want uq_urls_short_code violation", err)
		}

		var count int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM urls WHERE id = $1", secondID).Scan(&count); err != nil {
			t.Fatalf("count rejected URL: %v", err)
		}
		if count != 0 {
			t.Errorf("rejected URL count = %d, want 0", count)
		}
	})

	t.Run("reserves IDs concurrently without duplicates", func(t *testing.T) {
		const workers = 20
		type result struct {
			id  int64
			err error
		}

		results := make(chan result, workers)
		var group sync.WaitGroup
		for range workers {
			group.Add(1)
			go func() {
				defer group.Done()
				id, err := urlRepository.ReserveID(t.Context())
				results <- result{id: id, err: err}
			}()
		}
		group.Wait()
		close(results)

		seen := make(map[int64]bool, workers)
		for got := range results {
			if got.err != nil {
				t.Errorf("ReserveID() error = %v", got.err)
				continue
			}
			if got.id <= 0 || seen[got.id] {
				t.Errorf("ReserveID() returned invalid or duplicate ID %d", got.id)
			}
			seen[got.id] = true
		}
		if len(seen) != workers {
			t.Errorf("unique reserved IDs = %d, want %d", len(seen), workers)
		}
	})
}
