//go:build integration

package repositories_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/jackc/pgx/v5/pgxpool"
)

func visitorHash(b byte) []byte {
	return bytes.Repeat([]byte{b}, models.VisitorHashSize)
}

func audience(t *testing.T, pool *pgxpool.Pool, urlID int64) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), `SELECT day::text, unique_visitors, robot_visits FROM url_daily_audience WHERE url_id = $1 ORDER BY day`, urlID)
	if err != nil {
		t.Fatalf("query audience: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var day string
		var visitors, robots int64
		if err := rows.Scan(&day, &visitors, &robots); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, fmt.Sprintf("%s visitors=%d robots=%d", day, visitors, robots))
	}
	return got
}

func countries(t *testing.T, pool *pgxpool.Pool, urlID int64) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), `SELECT day::text, country, clicks FROM url_daily_countries WHERE url_id = $1 ORDER BY day, country`, urlID)
	if err != nil {
		t.Fatalf("query countries: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var day, country string
		var clicks int64
		if err := rows.Scan(&day, &country, &clicks); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, fmt.Sprintf("%s %s %d", day, country, clicks))
	}
	return got
}

func TestClickEventRepositoryCountsRobotsApart(t *testing.T) {
	pool := dbtest.Open(t)
	repo := repositories.NewClickEventRepository(pool)
	url := createClickTestURL(t, pool)
	at := time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)

	robot := models.Click{EventID: uuid.NewV7(), URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: at, VisitDetails: models.VisitDetails{Robot: true}}
	for range 2 {
		if _, err := repo.CountClickOnce(t.Context(), robot); err != nil {
			t.Fatalf("CountClickOnce() error = %v", err)
		}
	}
	robot.EventID = uuid.NewV7()
	if _, err := repo.CountClickOnce(t.Context(), robot); err != nil {
		t.Fatalf("CountClickOnce() error = %v", err)
	}

	if got := clickCount(t, pool, url.ID); got != 0 {
		t.Errorf("click_count = %d, want 0: robots are not clicks", got)
	}
	var daily int64
	if err := pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM url_daily_clicks WHERE url_id = $1`, url.ID).Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if daily != 0 {
		t.Errorf("url_daily_clicks rows = %d, want 0", daily)
	}
	if got, want := audience(t, pool, url.ID), []string{"2026-11-16 visitors=0 robots=2"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("audience = %v, want %v (the redelivery counts once)", got, want)
	}

	missing := models.Click{EventID: uuid.NewV7(), URLID: url.ID + 1_000_000, Source: models.ClickSourceLink, OccurredAt: at, VisitDetails: models.VisitDetails{Robot: true}}
	if _, err := repo.CountClickOnce(t.Context(), missing); !errors.Is(err, repositories.ErrURLNotFound) {
		t.Errorf("CountClickOnce() of a robot on a missing URL error = %v, want %v", err, repositories.ErrURLNotFound)
	}
}

func TestClickEventRepositoryCountsCountriesAndUniqueVisitors(t *testing.T) {
	pool := dbtest.Open(t)
	repo := repositories.NewClickEventRepository(pool)
	url := createClickTestURL(t, pool)
	// 02:30 UTC on the 17th is still the 16th in São Paulo.
	day16 := time.Date(2026, 11, 17, 2, 30, 0, 0, time.UTC)
	day17 := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	count := func(at time.Time, country string, visitor []byte, eventID uuid.UUID) {
		t.Helper()
		click := models.Click{EventID: eventID, URLID: url.ID, Source: models.ClickSourceLink, OccurredAt: at, VisitDetails: models.VisitDetails{Country: country, Visitor: visitor}}
		if _, err := repo.CountClickOnce(t.Context(), click); err != nil {
			t.Fatalf("CountClickOnce() error = %v", err)
		}
	}

	count(day16, "BR", visitorHash(1), uuid.NewV7())
	count(day16, "BR", visitorHash(1), uuid.NewV7()) // same visitor again
	count(day16, "PT", visitorHash(2), uuid.NewV7())
	count(day16, "", visitorHash(3), uuid.NewV7()) // unknown country
	count(day16, "BR", nil, uuid.NewV7())          // no salt that day
	redelivered := uuid.NewV7()
	count(day17, "BR", visitorHash(1), redelivered) // new day, new count
	count(day17, "BR", visitorHash(1), redelivered)

	if got := clickCount(t, pool, url.ID); got != 6 {
		t.Errorf("click_count = %d, want 6", got)
	}
	if got, want := countries(t, pool, url.ID), []string{"2026-11-16 BR 3", "2026-11-16 PT 1", "2026-11-17 BR 1"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("countries = %v, want %v", got, want)
	}
	if got, want := audience(t, pool, url.ID), []string{"2026-11-16 visitors=3 robots=0", "2026-11-17 visitors=1 robots=0"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("audience = %v, want %v", got, want)
	}
}

func TestClickEventRepositoryDeletesOldVisitorData(t *testing.T) {
	pool := dbtest.Open(t)
	repo := repositories.NewClickEventRepository(pool)
	salts := repositories.NewVisitorSaltRepository(pool)
	url := createClickTestURL(t, pool)

	for _, day := range []string{"2026-11-14", "2026-11-15", "2026-11-16"} {
		at, _ := time.Parse(time.DateOnly, day)
		if _, err := salts.DailySalt(t.Context(), at, bytes.Repeat([]byte{1}, 32)); err != nil {
			t.Fatalf("DailySalt() error = %v", err)
		}
		if _, err := pool.Exec(t.Context(), `INSERT INTO url_daily_visitor_hashes (url_id, day, visitor_hash) VALUES ($1, $2::date, $3)`, url.ID, day, visitorHash(9)); err != nil {
			t.Fatalf("insert visitor hash: %v", err)
		}
	}

	today := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	deletedSalts, deletedHashes, err := repo.DeleteVisitorDataBefore(t.Context(), today)
	if err != nil {
		t.Fatalf("DeleteVisitorDataBefore() error = %v", err)
	}
	if deletedSalts != 2 || deletedHashes != 1 {
		t.Errorf("deleted salts = %d, hashes = %d, want 2 and 1", deletedSalts, deletedHashes)
	}

	var saltDays, hashDays string
	if err := pool.QueryRow(t.Context(), `SELECT string_agg(day::text, ',' ORDER BY day) FROM visitor_salts`).Scan(&saltDays); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(t.Context(), `SELECT string_agg(day::text, ',' ORDER BY day) FROM url_daily_visitor_hashes WHERE url_id = $1`, url.ID).Scan(&hashDays); err != nil {
		t.Fatal(err)
	}
	if saltDays != "2026-11-16" || hashDays != "2026-11-15,2026-11-16" {
		t.Errorf("kept salts %q and hashes %q, want today's salt and hashes from yesterday on", saltDays, hashDays)
	}
}

func TestVisitorSaltRepositoryKeepsTheFirstSaltOfTheDay(t *testing.T) {
	pool := dbtest.Open(t)
	repo := repositories.NewVisitorSaltRepository(pool)
	day := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
	first := bytes.Repeat([]byte{1}, 32)

	got, err := repo.DailySalt(t.Context(), day, first)
	if err != nil {
		t.Fatalf("DailySalt() error = %v", err)
	}
	again, err := repo.DailySalt(t.Context(), day, bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatalf("DailySalt() error = %v", err)
	}

	if !bytes.Equal(got, first) || !bytes.Equal(again, first) {
		t.Errorf("salts = %x, %x, want the first candidate both times", got, again)
	}
}
