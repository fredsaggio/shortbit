package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/jackc/pgx/v5"
)

type ClickEventRepository struct {
	db db.DB
}

func NewClickEventRepository(db db.DB) *ClickEventRepository {
	return &ClickEventRepository{db: db}
}

// CountClickOnce adds the click to the link's total and to its day and
// source, all in one transaction keyed by the event ID, so a redelivered
// event changes nothing. The day is the click's date in America/Sao_Paulo.
func (r *ClickEventRepository) CountClickOnce(ctx context.Context, click models.Click) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin count click transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const recordEvent = `
		INSERT INTO processed_click_events (event_id)
		VALUES (@eventID)
		ON CONFLICT (event_id) DO NOTHING
	`
	recorded, err := tx.Exec(ctx, recordEvent, pgx.StrictNamedArgs{"eventID": click.EventID})
	if err != nil {
		return false, fmt.Errorf("record processed click event: %w", err)
	}
	if recorded.RowsAffected() == 0 {
		return false, nil
	}

	const countClick = `
		UPDATE urls
		SET click_count = click_count + 1
		WHERE id = @urlID
	`
	updated, err := tx.Exec(ctx, countClick, pgx.StrictNamedArgs{"urlID": click.URLID})
	if err != nil {
		return false, fmt.Errorf("count click: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return false, ErrURLNotFound
	}

	const countDailyClick = `
		INSERT INTO url_daily_clicks (url_id, day, source, clicks)
		VALUES (@urlID, (@occurredAt::timestamptz AT TIME ZONE 'America/Sao_Paulo')::date, @source, 1)
		ON CONFLICT (url_id, day, source) DO UPDATE
		SET clicks = url_daily_clicks.clicks + 1
	`
	if _, err := tx.Exec(ctx, countDailyClick, pgx.StrictNamedArgs{
		"urlID":      click.URLID,
		"occurredAt": click.OccurredAt,
		"source":     string(click.Source),
	}); err != nil {
		return false, fmt.Errorf("count daily click: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit count click transaction: %w", err)
	}

	return true, nil
}

func (r *ClickEventRepository) DeleteProcessedClickEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const q = `
		DELETE FROM processed_click_events
		WHERE processed_at < @cutoff
	`

	deleted, err := r.db.Exec(ctx, q, pgx.StrictNamedArgs{"cutoff": cutoff})
	if err != nil {
		return 0, fmt.Errorf("delete processed click events: %w", err)
	}

	return deleted.RowsAffected(), nil
}
