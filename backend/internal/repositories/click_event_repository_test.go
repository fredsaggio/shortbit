//go:build integration

package repositories_test

import (
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClickEventRepositoryIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	clickEventRepository := repositories.NewClickEventRepository(pool)
	url := createClickTestURL(t, pool)

	t.Run("counts a new event", func(t *testing.T) {
		before := clickCount(t, pool, url.ID)

		counted, err := clickEventRepository.CountClickOnce(t.Context(), uuid.NewV7(), url.ID)
		if err != nil {
			t.Fatalf("CountClickOnce() error = %v", err)
		}

		if !counted {
			t.Error("CountClickOnce() counted = false, want true")
		}
		if got := clickCount(t, pool, url.ID); got != before+1 {
			t.Errorf("click_count = %d, want %d", got, before+1)
		}
	})

	t.Run("does not count a redelivered event twice", func(t *testing.T) {
		eventID := uuid.NewV7()
		before := clickCount(t, pool, url.ID)

		if _, err := clickEventRepository.CountClickOnce(t.Context(), eventID, url.ID); err != nil {
			t.Fatalf("first CountClickOnce() error = %v", err)
		}

		counted, err := clickEventRepository.CountClickOnce(t.Context(), eventID, url.ID)
		if err != nil {
			t.Fatalf("second CountClickOnce() error = %v", err)
		}

		if counted {
			t.Error("second CountClickOnce() counted = true, want false")
		}
		if got := clickCount(t, pool, url.ID); got != before+1 {
			t.Errorf("click_count = %d, want %d", got, before+1)
		}
	})

	t.Run("counts concurrent deliveries of the same event once", func(t *testing.T) {
		eventID := uuid.NewV7()
		before := clickCount(t, pool, url.ID)

		const deliveries = 10
		var wg sync.WaitGroup
		errs := make(chan error, deliveries)

		for range deliveries {
			wg.Go(func() {
				if _, err := clickEventRepository.CountClickOnce(t.Context(), eventID, url.ID); err != nil {
					errs <- err
				}
			})
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			t.Errorf("CountClickOnce() error = %v", err)
		}
		if got := clickCount(t, pool, url.ID); got != before+1 {
			t.Errorf("click_count = %d, want %d", got, before+1)
		}
	})

	t.Run("reports a missing URL without recording the event", func(t *testing.T) {
		eventID := uuid.NewV7()

		_, err := clickEventRepository.CountClickOnce(t.Context(), eventID, url.ID+1_000_000)
		if !errors.Is(err, repositories.ErrURLNotFound) {
			t.Fatalf("CountClickOnce() error = %v, want %v", err, repositories.ErrURLNotFound)
		}

		var exists bool
		if err := pool.QueryRow(t.Context(), `SELECT EXISTS (SELECT 1 FROM processed_click_events WHERE event_id = $1)`, eventID).Scan(&exists); err != nil {
			t.Fatalf("check processed event: %v", err)
		}
		if exists {
			t.Error("event recorded for a missing URL")
		}
	})

	t.Run("deletes processed events older than the cutoff", func(t *testing.T) {
		oldEventID := uuid.NewV7()
		recentEventID := uuid.NewV7()

		if _, err := pool.Exec(t.Context(), `
			INSERT INTO processed_click_events (event_id, processed_at)
			VALUES ($1, NOW() - INTERVAL '8 days'), ($2, NOW())
		`, oldEventID, recentEventID); err != nil {
			t.Fatalf("insert processed events: %v", err)
		}

		deleted, err := clickEventRepository.DeleteProcessedClickEventsBefore(t.Context(), time.Now().Add(-7*24*time.Hour))
		if err != nil {
			t.Fatalf("DeleteProcessedClickEventsBefore() error = %v", err)
		}

		if deleted != 1 {
			t.Errorf("deleted = %d, want 1", deleted)
		}

		var remaining int
		if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM processed_click_events WHERE event_id IN ($1, $2)`, oldEventID, recentEventID).Scan(&remaining); err != nil {
			t.Fatalf("count remaining events: %v", err)
		}
		if remaining != 1 {
			t.Errorf("remaining events = %d, want only the recent one", remaining)
		}
	})
}

func createClickTestURL(t *testing.T, pool *pgxpool.Pool) models.URL {
	t.Helper()

	user, err := repositories.NewUserRepository(pool).CreateWithPassword(t.Context(), "click-events@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("CreateWithPassword() error = %v", err)
	}

	urlRepository := repositories.NewURLRepository(pool)

	id, err := urlRepository.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("ReserveID() error = %v", err)
	}

	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}

	code, err := generator.Generate(id)
	if err != nil {
		t.Fatalf("Generate(%d) error = %v", id, err)
	}

	url, err := urlRepository.Create(t.Context(), models.URL{
		ID:          id,
		ShortCode:   code,
		UserID:      user.ID,
		OriginalURL: "https://example.com/clicks",
		Visibility:  models.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	return url
}

func clickCount(t *testing.T, pool *pgxpool.Pool, urlID int64) int64 {
	t.Helper()

	var count int64
	if err := pool.QueryRow(t.Context(), `SELECT click_count FROM urls WHERE id = $1`, urlID).Scan(&count); err != nil {
		t.Fatalf("read click_count: %v", err)
	}

	return count
}
