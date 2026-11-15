package services

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

type clickStatsRepoStub struct {
	link     models.URL
	findErr  error
	counts   []models.DailyClickCount
	gotFrom  time.Time
	gotTo    time.Time
	gotURLID int64
}

func (s *clickStatsRepoStub) GetByShortcode(context.Context, uuid.UUID, string) (models.URL, error) {
	return s.link, s.findErr
}

func (s *clickStatsRepoStub) DailyClicks(_ context.Context, urlID int64, from, to time.Time) ([]models.DailyClickCount, error) {
	s.gotURLID, s.gotFrom, s.gotTo = urlID, from, to
	return s.counts, nil
}

func day(text string) time.Time {
	parsed, err := time.Parse(time.DateOnly, text)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestClickStatsFillsEveryDayAndSumsBySource(t *testing.T) {
	repo := &clickStatsRepoStub{
		link: models.URL{ID: 42},
		counts: []models.DailyClickCount{
			{Day: day("2026-11-09"), Source: models.ClickSourceLink, Clicks: 3},
			{Day: day("2026-11-09"), Source: models.ClickSourceQR, Clicks: 1},
			{Day: day("2026-11-14"), Source: models.ClickSourceQR, Clicks: 5},
		},
	}
	service, err := NewClickStatsService(repo)
	if err != nil {
		t.Fatal(err)
	}
	// 01:30 UTC on the 15th is still the 14th in São Paulo.
	service.now = func() time.Time { return time.Date(2026, 11, 15, 1, 30, 0, 0, time.UTC) }

	stats, err := service.DailyClicks(t.Context(), uuid.NewV7(), "Ab3dX9", 7)
	if err != nil {
		t.Fatalf("DailyClicks() error = %v", err)
	}

	if repo.gotURLID != 42 || !repo.gotFrom.Equal(day("2026-11-08")) || !repo.gotTo.Equal(day("2026-11-14")) {
		t.Errorf("query = link %d from %v to %v, want link 42 from 2026-11-08 to 2026-11-14", repo.gotURLID, repo.gotFrom, repo.gotTo)
	}
	if len(stats.Days) != 7 || !stats.Days[0].Day.Equal(day("2026-11-08")) || !stats.Days[6].Day.Equal(day("2026-11-14")) {
		t.Fatalf("days = %+v, want 7 days from 2026-11-08 to today, 2026-11-14", stats.Days)
	}
	if got := stats.Days[1]; got.Link != 3 || got.QR != 1 {
		t.Errorf("2026-11-09 = %+v, want 3 by link and 1 by QR code", got)
	}
	if got := stats.Days[2]; got.Link != 0 || got.QR != 0 {
		t.Errorf("2026-11-10 = %+v, want zero", got)
	}
	if got := stats.Days[6]; got.QR != 5 {
		t.Errorf("today = %+v, want 5 by QR code", got)
	}
	if stats.Link != 3 || stats.QR != 6 {
		t.Errorf("totals = %d link, %d QR; want 3 and 6", stats.Link, stats.QR)
	}
}

func TestClickStatsRejectsOtherPeriodsAndMissingLinks(t *testing.T) {
	service, err := NewClickStatsService(&clickStatsRepoStub{findErr: repositories.ErrURLNotFound})
	if err != nil {
		t.Fatal(err)
	}

	for _, days := range []int{0, 1, 14, 365} {
		if _, err := service.DailyClicks(t.Context(), uuid.NewV7(), "Ab3dX9", days); !errors.Is(err, ErrInvalidClickPeriod) {
			t.Errorf("DailyClicks(%d days) error = %v, want ErrInvalidClickPeriod", days, err)
		}
	}
	if _, err := service.DailyClicks(t.Context(), uuid.NewV7(), "Ab3dX9", 30); !errors.Is(err, ErrURLNotFound) {
		t.Errorf("DailyClicks(missing link) error = %v, want ErrURLNotFound", err)
	}
	if _, err := service.DailyClicks(t.Context(), uuid.Nil(), "Ab3dX9", 30); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("DailyClicks(no user) error = %v, want ErrUnauthenticated", err)
	}
}
