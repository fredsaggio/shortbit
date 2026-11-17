package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

// DailyClicks returns the link's clicks per day and source from the day
// from to the day to, both included, oldest first. Days without clicks have
// no row.
func (r *URLRepository) DailyClicks(ctx context.Context, urlID int64, from, to time.Time) ([]models.DailyClickCount, error) {
	const q = `
		SELECT day, source, clicks
		FROM url_daily_clicks
		WHERE url_id = @urlID AND day BETWEEN @from::date AND @to::date
		ORDER BY day, source
	`
	rows, err := r.db.Query(ctx, q, pgx.StrictNamedArgs{
		"urlID": urlID,
		"from":  from.Format(time.DateOnly),
		"to":    to.Format(time.DateOnly),
	})
	if err != nil {
		return nil, fmt.Errorf("query daily clicks: %w", err)
	}

	counts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DailyClickCount, error) {
		var count models.DailyClickCount
		var source string
		err := row.Scan(&count.Day, &source, &count.Clicks)
		count.Source = models.ClickSource(source)
		return count, err
	})
	if err != nil {
		return nil, fmt.Errorf("read daily clicks: %w", err)
	}
	return counts, nil
}

// DailyAudience returns the link's unique visitors and robot visits per day
// from the day from to the day to, both included, oldest first. Days without
// either have no row.
func (r *URLRepository) DailyAudience(ctx context.Context, urlID int64, from, to time.Time) ([]models.DailyAudience, error) {
	const q = `
		SELECT day, unique_visitors, robot_visits
		FROM url_daily_audience
		WHERE url_id = @urlID AND day BETWEEN @from::date AND @to::date
		ORDER BY day
	`
	rows, err := r.db.Query(ctx, q, pgx.StrictNamedArgs{
		"urlID": urlID,
		"from":  from.Format(time.DateOnly),
		"to":    to.Format(time.DateOnly),
	})
	if err != nil {
		return nil, fmt.Errorf("query daily audience: %w", err)
	}

	audience, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DailyAudience, error) {
		var day models.DailyAudience
		err := row.Scan(&day.Day, &day.UniqueVisitors, &day.RobotVisits)
		return day, err
	})
	if err != nil {
		return nil, fmt.Errorf("read daily audience: %w", err)
	}
	return audience, nil
}

// CountryClicks returns the link's clicks per country from the day from to
// the day to, both included, most clicks first. Clicks without a country are
// not in it.
func (r *URLRepository) CountryClicks(ctx context.Context, urlID int64, from, to time.Time) ([]models.CountryClicks, error) {
	const q = `
		SELECT country, SUM(clicks)::bigint
		FROM url_daily_countries
		WHERE url_id = @urlID AND day BETWEEN @from::date AND @to::date
		GROUP BY country
		ORDER BY SUM(clicks) DESC, country
	`
	rows, err := r.db.Query(ctx, q, pgx.StrictNamedArgs{
		"urlID": urlID,
		"from":  from.Format(time.DateOnly),
		"to":    to.Format(time.DateOnly),
	})
	if err != nil {
		return nil, fmt.Errorf("query country clicks: %w", err)
	}

	countries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.CountryClicks, error) {
		var country models.CountryClicks
		err := row.Scan(&country.Country, &country.Clicks)
		return country, err
	})
	if err != nil {
		return nil, fmt.Errorf("read country clicks: %w", err)
	}
	return countries, nil
}
