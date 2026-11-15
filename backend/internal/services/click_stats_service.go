package services

import (
	"context"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // The day boundary must not depend on the image's zone files.
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

var ErrInvalidClickPeriod = errors.New("click period must be 7, 30 or 90 days")

// ClickPeriods are the periods the link page offers, in days.
var ClickPeriods = []int{7, 30, 90}

// clickDayZone is where a day starts and ends; it matches the AT TIME ZONE
// of the click counting query (ADR 0005).
const clickDayZone = "America/Sao_Paulo"

type ClickStatsRepository interface {
	GetByShortcode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error)
	DailyClicks(ctx context.Context, urlID int64, from, to time.Time) ([]models.DailyClickCount, error)
}

type DayClicks struct {
	Day  time.Time
	Link int64
	QR   int64
}

type ClickStats struct {
	Days []DayClicks
	Link int64
	QR   int64
}

type ClickStatsService struct {
	repo     ClickStatsRepository
	location *time.Location
	now      func() time.Time
}

func NewClickStatsService(repo ClickStatsRepository) (*ClickStatsService, error) {
	location, err := time.LoadLocation(clickDayZone)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", clickDayZone, err)
	}
	return &ClickStatsService{repo: repo, location: location, now: time.Now}, nil
}

// DailyClicks returns one entry per day of the last days days, today
// included, with zero for days without clicks, and the totals per source.
func (s *ClickStatsService) DailyClicks(ctx context.Context, userID uuid.UUID, shortCode string, days int) (ClickStats, error) {
	if userID == uuid.Nil() {
		return ClickStats{}, ErrUnauthenticated
	}
	if !validClickPeriod(days) {
		return ClickStats{}, ErrInvalidClickPeriod
	}

	link, err := s.repo.GetByShortcode(ctx, userID, shortCode)
	if err != nil {
		if errors.Is(err, repositories.ErrURLNotFound) {
			return ClickStats{}, ErrURLNotFound
		}
		return ClickStats{}, fmt.Errorf("find link for click stats: %w", err)
	}

	now := s.now().In(s.location)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -(days - 1))

	counts, err := s.repo.DailyClicks(ctx, link.ID, from, to)
	if err != nil {
		return ClickStats{}, fmt.Errorf("daily clicks: %w", err)
	}

	stats := ClickStats{Days: make([]DayClicks, days)}
	index := make(map[string]int, days)
	for i := range stats.Days {
		day := from.AddDate(0, 0, i)
		stats.Days[i].Day = day
		index[day.Format(time.DateOnly)] = i
	}
	for _, count := range counts {
		i, ok := index[count.Day.Format(time.DateOnly)]
		if !ok {
			continue
		}
		switch count.Source {
		case models.ClickSourceQR:
			stats.Days[i].QR += count.Clicks
			stats.QR += count.Clicks
		default:
			stats.Days[i].Link += count.Clicks
			stats.Link += count.Clicks
		}
	}
	return stats, nil
}

func validClickPeriod(days int) bool {
	for _, period := range ClickPeriods {
		if days == period {
			return true
		}
	}
	return false
}
