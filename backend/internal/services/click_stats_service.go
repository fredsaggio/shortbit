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

// ClickDayZone is where a day starts and ends; it matches the AT TIME ZONE
// of the click counting query (ADR 0005).
const ClickDayZone = "America/Sao_Paulo"

type ClickStatsRepository interface {
	GetByShortcode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error)
	DailyClicks(ctx context.Context, urlID int64, from, to time.Time) ([]models.DailyClickCount, error)
	DailyAudience(ctx context.Context, urlID int64, from, to time.Time) ([]models.DailyAudience, error)
	CountryClicks(ctx context.Context, urlID int64, from, to time.Time) ([]models.CountryClicks, error)
}

// DayClicks is one day of a link: clicks by source, unique visitors and
// robot visits, which are not clicks (ADR 0007).
type DayClicks struct {
	Day      time.Time
	Link     int64
	QR       int64
	Visitors int64
	Robots   int64
}

// ClickStats sums the period. Visitors is the sum of the daily unique
// visitors, so someone back on another day counts again. NoCountry is the
// clicks whose country is unknown, older clicks included.
type ClickStats struct {
	Days      []DayClicks
	Link      int64
	QR        int64
	Visitors  int64
	Robots    int64
	Countries []models.CountryClicks
	NoCountry int64
}

type ClickStatsService struct {
	repo     ClickStatsRepository
	location *time.Location
	now      func() time.Time
}

func NewClickStatsService(repo ClickStatsRepository) (*ClickStatsService, error) {
	location, err := time.LoadLocation(ClickDayZone)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", ClickDayZone, err)
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
	audience, err := s.repo.DailyAudience(ctx, link.ID, from, to)
	if err != nil {
		return ClickStats{}, fmt.Errorf("daily audience: %w", err)
	}
	countries, err := s.repo.CountryClicks(ctx, link.ID, from, to)
	if err != nil {
		return ClickStats{}, fmt.Errorf("country clicks: %w", err)
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
	for _, day := range audience {
		i, ok := index[day.Day.Format(time.DateOnly)]
		if !ok {
			continue
		}
		stats.Days[i].Visitors += day.UniqueVisitors
		stats.Days[i].Robots += day.RobotVisits
		stats.Visitors += day.UniqueVisitors
		stats.Robots += day.RobotVisits
	}

	stats.Countries = countries
	stats.NoCountry = stats.Link + stats.QR
	for _, country := range countries {
		stats.NoCountry -= country.Clicks
	}
	stats.NoCountry = max(stats.NoCountry, 0)
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
