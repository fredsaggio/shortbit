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
