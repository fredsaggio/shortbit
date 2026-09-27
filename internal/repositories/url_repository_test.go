//go:build integration

package repositories_test

import (
	"errors"
	"sync"
	"testing"
	"uuid"

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

func TestURLRepositoryListIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	owner, err := users.CreateWithPassword(t.Context(), "list-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := users.CreateWithPassword(t.Context(), "list-other@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	createURL := func(userID uuid.UUID, visibility models.Visibility) models.URL {
		t.Helper()
		id, err := urls.ReserveID(t.Context())
		if err != nil {
			t.Fatalf("ReserveID() error = %v", err)
		}
		code, err := generator.Generate(id)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", id, err)
		}
		url := models.URL{ID: id, ShortCode: code, UserID: userID, OriginalURL: "https://example.com", Visibility: visibility}
		if visibility == models.VisibilityPrivate {
			hash := "$argon2id$private-link-test-hash"
			url.PasswordHash = &hash
		}
		created, err := urls.Create(t.Context(), url)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		return created
	}

	first := createURL(owner.ID, models.VisibilityPublic)
	second := createURL(owner.ID, models.VisibilityPrivate)
	third := createURL(owner.ID, models.VisibilityPublic)
	createURL(other.ID, models.VisibilityPublic)

	// Same timestamps make the ID the decisive part of the keyset order.
	if _, err := pool.Exec(t.Context(), "UPDATE urls SET created_at = TIMESTAMPTZ '2026-01-01 00:00:00+00' WHERE user_id = $1", owner.ID); err != nil {
		t.Fatalf("set owner timestamps: %v", err)
	}

	page, err := urls.List(t.Context(), owner.ID, 2, nil)
	if err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	if len(page) != 2 || page[0].ID != third.ID || page[1].ID != second.ID {
		t.Fatalf("first page = %+v, want IDs %d, %d", page, third.ID, second.ID)
	}
	for _, url := range page {
		if url.UserID != owner.ID || url.PasswordHash != nil {
			t.Errorf("listed URL = %+v, want owner and no password hash", url)
		}
	}

	cursor := &repositories.URLCursor{CreatedAt: page[1].CreatedAt, ID: page[1].ID}
	page, err = urls.List(t.Context(), owner.ID, 2, cursor)
	if err != nil {
		t.Fatalf("second List() error = %v", err)
	}
	if len(page) != 1 || page[0].ID != first.ID {
		t.Fatalf("second page = %+v, want ID %d", page, first.ID)
	}
	if page[0].UserID != owner.ID || page[0].PasswordHash != nil {
		t.Errorf("listed URL = %+v, want owner and no password hash", page[0])
	}

	page, err = urls.List(t.Context(), other.ID, 2, nil)
	if err != nil || len(page) != 1 || page[0].UserID != other.ID {
		t.Fatalf("other user page = %+v, error = %v, want one owned URL", page, err)
	}
}

func TestURLRepositoryGetByShortcodeIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	owner, err := users.CreateWithPassword(t.Context(), "get-url-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := users.CreateWithPassword(t.Context(), "get-url-other@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}

	id, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() error = %v", err)
	}
	code, err := generator.Generate(id)
	if err != nil {
		t.Fatalf("Generate(%d) error = %v", id, err)
	}
	passwordHash := "$argon2id$private-link-test-hash"
	created, err := urls.Create(t.Context(), models.URL{
		ID: id, ShortCode: code, UserID: owner.ID,
		OriginalURL: "https://example.com/private", Visibility: models.VisibilityPrivate,
		PasswordHash: &passwordHash,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE urls SET click_count = 7 WHERE id = $1", created.ID); err != nil {
		t.Fatalf("set click count: %v", err)
	}
	created.ClickCount = 7

	missingID, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() for missing link error = %v", err)
	}
	missingCode, err := generator.Generate(missingID)
	if err != nil {
		t.Fatalf("Generate(%d) for missing link error = %v", missingID, err)
	}

	t.Run("owner finds link metadata without password hash", func(t *testing.T) {
		got, err := urls.GetByShortcode(t.Context(), owner.ID, code)
		if err != nil {
			t.Fatalf("GetByShortcode() error = %v", err)
		}
		if got.ID != created.ID || got.ShortCode != created.ShortCode || got.UserID != owner.ID ||
			got.OriginalURL != created.OriginalURL || got.Visibility != created.Visibility ||
			got.ClickCount != created.ClickCount || !got.CreatedAt.Equal(created.CreatedAt) ||
			!got.UpdatedAt.Equal(created.UpdatedAt) {
			t.Errorf("GetByShortcode() = %+v, want metadata from %+v", got, created)
		}
		if got.PasswordHash != nil {
			t.Errorf("GetByShortcode() password hash = %q, want nil", *got.PasswordHash)
		}
	})

	t.Run("another user cannot find the link", func(t *testing.T) {
		_, err := urls.GetByShortcode(t.Context(), other.ID, code)
		if !errors.Is(err, repositories.ErrURLNotFound) {
			t.Errorf("GetByShortcode() error = %v, want ErrURLNotFound", err)
		}
	})

	t.Run("unknown shortcode is not found", func(t *testing.T) {
		_, err := urls.GetByShortcode(t.Context(), owner.ID, missingCode)
		if !errors.Is(err, repositories.ErrURLNotFound) {
			t.Errorf("GetByShortcode() error = %v, want ErrURLNotFound", err)
		}
	})
}
