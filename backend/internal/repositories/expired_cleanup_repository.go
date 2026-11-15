package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/fredsaggio/url-shortener/internal/db"
	"github.com/jackc/pgx/v5"
)

// ExpiredRecordsRepository deletes records that stopped being valid. Every
// query uses the table's expiry index and deletes in batches, so a large
// backlog never holds row locks for long.
type ExpiredRecordsRepository struct {
	db db.DB
}

func NewExpiredRecordsRepository(db db.DB) *ExpiredRecordsRepository {
	return &ExpiredRecordsRepository{db: db}
}

// Each query deletes at most @limit rows whose expiry is not after @now. The
// tables and columns are fixed here, never built from input.
var expiredRecordQueries = map[string]string{
	"user_sessions": `
		DELETE FROM user_sessions
		WHERE token_hash IN (
			SELECT token_hash FROM user_sessions WHERE expires_at <= @now LIMIT @limit
		)`,
	"link_access_sessions": `
		DELETE FROM link_access_sessions
		WHERE token_hash IN (
			SELECT token_hash FROM link_access_sessions WHERE expires_at <= @now LIMIT @limit
		)`,
	"password_reset_attempts": `
		DELETE FROM password_reset_attempts
		WHERE token_hash IN (
			SELECT token_hash FROM password_reset_attempts WHERE attempt_expires_at <= @now LIMIT @limit
		)`,
}

// ExpiredTables lists the tables DeleteExpiredBatch accepts, in a stable order.
var ExpiredTables = []string{"user_sessions", "link_access_sessions", "password_reset_attempts"}

// DeleteExpiredBatch deletes up to limit expired rows of table and reports how
// many it removed; fewer than limit means the table is clean.
func (r *ExpiredRecordsRepository) DeleteExpiredBatch(ctx context.Context, table string, now time.Time, limit int) (int64, error) {
	query, ok := expiredRecordQueries[table]
	if !ok {
		return 0, fmt.Errorf("no expiry cleanup for table %q", table)
	}

	result, err := r.db.Exec(ctx, query, pgx.StrictNamedArgs{"now": now, "limit": limit})
	if err != nil {
		return 0, fmt.Errorf("delete expired %s: %w", table, err)
	}

	return result.RowsAffected(), nil
}
