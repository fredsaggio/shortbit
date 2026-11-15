//go:build integration

package repositories_test

import (
	"errors"
	"fmt"
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

		counted, err := clickEventRepository.CountClickOnce(t.Context(), models.Click{EventID: uuid.NewV7(), URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: time.Now()})
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

		if _, err := clickEventRepository.CountClickOnce(t.Context(), models.Click{EventID: eventID, URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: time.Now()}); err != nil {
			t.Fatalf("first CountClickOnce() error = %v", err)
		}

		counted, err := clickEventRepository.CountClickOnce(t.Context(), models.Click{EventID: eventID, URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: time.Now()})
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
				if _, err := clickEventRepository.CountClickOnce(t.Context(), models.Click{EventID: eventID, URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: time.Now()}); err != nil {
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

		_, err := clickEventRepository.CountClickOnce(t.Context(), models.Click{EventID: eventID, URLID: url.ID + 1_000_000, Source: models.ClickSourceLink, OccurredAt: time.Now()})
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

	user, err := repositories.NewUserRepository(pool).CreateWithPassword(t.Context(), "click-events@example.com", "$argon2id$integration-test-hash", nil)
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

func TestClickEventRepositoryCountsByDayAndSource(t *testing.T) {
	pool := dbtest.Open(t)
	repo := repositories.NewClickEventRepository(pool)
	url := createClickTestURL(t, pool)

	// 02:30 UTC on the 15th is still 23:30 on the 14th in São Paulo.
	lateEvening := time.Date(2026, 11, 15, 2, 30, 0, 0, time.UTC)
	nextMorning := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	count := func(source models.ClickSource, at time.Time, eventID uuid.UUID) {
		t.Helper()
		if _, err := repo.CountClickOnce(t.Context(), models.Click{EventID: eventID, URLID: url.ID, Source: source, OccurredAt: at}); err != nil {
			t.Fatalf("CountClickOnce() error = %v", err)
		}
	}

	count(models.ClickSourceLink, lateEvening, uuid.NewV7())
	count(models.ClickSourceQR, lateEvening, uuid.NewV7())
	count(models.ClickSourceQR, lateEvening, uuid.NewV7())
	redelivered := uuid.NewV7()
	count(models.ClickSourceQR, nextMorning, redelivered)
	count(models.ClickSourceQR, nextMorning, redelivered)

	rows, err := pool.Query(t.Context(), `SELECT day::text, source, clicks FROM url_daily_clicks WHERE url_id = $1 ORDER BY day, source`, url.ID)
	if err != nil {
		t.Fatalf("query daily clicks: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var day, source string
		var clicks int64
		if err := rows.Scan(&day, &source, &clicks); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, fmt.Sprintf("%s %s %d", day, source, clicks))
	}
	want := []string{"2026-11-14 link 1", "2026-11-14 qr 2", "2026-11-15 qr 1"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("daily clicks = %v, want %v", got, want)
	}
	if total := clickCount(t, pool, url.ID); total != 4 {
		t.Errorf("click_count = %d, want 4 (the redelivery counts once)", total)
	}
}
