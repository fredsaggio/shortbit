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

// clickDay is the click's date in America/Sao_Paulo (ADR 0005).
const clickDay = `(@occurredAt::timestamptz AT TIME ZONE 'America/Sao_Paulo')::date`

// CountClickOnce counts the click in one transaction keyed by the event ID,
// so a redelivered event changes nothing. A person adds to the link's total,
// its day and source, its country and, once per visitor hash, its unique
// visitors; a robot adds only to the day's robot visits (ADR 0007).
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

	if click.Robot {
		err = countRobotVisit(ctx, tx, click)
	} else {
		err = countPersonClick(ctx, tx, click)
	}
	if err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit count click transaction: %w", err)
	}

	return true, nil
}

func countRobotVisit(ctx context.Context, tx pgx.Tx, click models.Click) error {
	const q = `
		INSERT INTO url_daily_audience (url_id, day, robot_visits)
		VALUES (@urlID, ` + clickDay + `, 1)
		ON CONFLICT (url_id, day) DO UPDATE
		SET robot_visits = url_daily_audience.robot_visits + 1
	`
	if _, err := tx.Exec(ctx, q, pgx.StrictNamedArgs{"urlID": click.URLID, "occurredAt": click.OccurredAt}); err != nil {
		if isForeignKeyViolation(err) {
			return ErrURLNotFound
		}
		return fmt.Errorf("count robot visit: %w", err)
	}
	return nil
}

func countPersonClick(ctx context.Context, tx pgx.Tx, click models.Click) error {
	const countClick = `
		UPDATE urls
		SET click_count = click_count + 1
		WHERE id = @urlID
	`
	updated, err := tx.Exec(ctx, countClick, pgx.StrictNamedArgs{"urlID": click.URLID})
	if err != nil {
		return fmt.Errorf("count click: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return ErrURLNotFound
	}

	const countDailyClick = `
		INSERT INTO url_daily_clicks (url_id, day, source, clicks)
		VALUES (@urlID, ` + clickDay + `, @source, 1)
		ON CONFLICT (url_id, day, source) DO UPDATE
		SET clicks = url_daily_clicks.clicks + 1
	`
	if _, err := tx.Exec(ctx, countDailyClick, pgx.StrictNamedArgs{
		"urlID":      click.URLID,
		"occurredAt": click.OccurredAt,
		"source":     string(click.Source),
	}); err != nil {
		return fmt.Errorf("count daily click: %w", err)
	}

	if click.Country != "" {
		const countCountry = `
			INSERT INTO url_daily_countries (url_id, day, country, clicks)
			VALUES (@urlID, ` + clickDay + `, @country, 1)
			ON CONFLICT (url_id, day, country) DO UPDATE
			SET clicks = url_daily_countries.clicks + 1
		`
		if _, err := tx.Exec(ctx, countCountry, pgx.StrictNamedArgs{
			"urlID":      click.URLID,
			"occurredAt": click.OccurredAt,
			"country":    click.Country,
		}); err != nil {
			return fmt.Errorf("count click country: %w", err)
		}
	}

	if len(click.Visitor) == 0 {
		return nil
	}

	const recordVisitor = `
		INSERT INTO url_daily_visitor_hashes (url_id, day, visitor_hash)
		VALUES (@urlID, ` + clickDay + `, @visitor)
		ON CONFLICT (url_id, day, visitor_hash) DO NOTHING
	`
	recordedVisitor, err := tx.Exec(ctx, recordVisitor, pgx.StrictNamedArgs{
		"urlID":      click.URLID,
		"occurredAt": click.OccurredAt,
		"visitor":    click.Visitor,
	})
	if err != nil {
		return fmt.Errorf("record visitor: %w", err)
	}
	if recordedVisitor.RowsAffected() == 0 {
		return nil
	}

	const countVisitor = `
		INSERT INTO url_daily_audience (url_id, day, unique_visitors)
		VALUES (@urlID, ` + clickDay + `, 1)
		ON CONFLICT (url_id, day) DO UPDATE
		SET unique_visitors = url_daily_audience.unique_visitors + 1
	`
	if _, err := tx.Exec(ctx, countVisitor, pgx.StrictNamedArgs{"urlID": click.URLID, "occurredAt": click.OccurredAt}); err != nil {
		return fmt.Errorf("count unique visitor: %w", err)
	}
	return nil
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

// DeleteVisitorDataBefore deletes the salts of days before today and the
// visitor hashes of days before yesterday, today being the date in
// America/Sao_Paulo (ADR 0007). The day counts stay.
func (r *ClickEventRepository) DeleteVisitorDataBefore(ctx context.Context, today time.Time) (int64, int64, error) {
	const deleteSalts = `DELETE FROM visitor_salts WHERE day < @today::date`
	salts, err := r.db.Exec(ctx, deleteSalts, pgx.StrictNamedArgs{"today": today.Format(time.DateOnly)})
	if err != nil {
		return 0, 0, fmt.Errorf("delete visitor salts: %w", err)
	}

	const deleteHashes = `DELETE FROM url_daily_visitor_hashes WHERE day < @yesterday::date`
	hashes, err := r.db.Exec(ctx, deleteHashes, pgx.StrictNamedArgs{"yesterday": today.AddDate(0, 0, -1).Format(time.DateOnly)})
	if err != nil {
		return salts.RowsAffected(), 0, fmt.Errorf("delete visitor hashes: %w", err)
	}

	return salts.RowsAffected(), hashes.RowsAffected(), nil
}
