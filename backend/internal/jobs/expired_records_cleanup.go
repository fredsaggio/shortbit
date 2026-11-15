package jobs

import (
	"context"
	"log/slog"
	"time"
)

const (
	expiredRecordsCleanupInterval = time.Hour
	expiredRecordsBatchSize       = 1000
)

type ExpiredRecordsRepository interface {
	DeleteExpiredBatch(ctx context.Context, table string, now time.Time, limit int) (int64, error)
}

// ExpiredRecordsCleanup deletes login sessions, link unlock sessions and
// password reset attempts once they expire. They are already rejected after
// expiry; this only keeps them from piling up in the database. Deleting an
// expired row is safe to repeat, so a restart or two instances running it at
// once change nothing.
type ExpiredRecordsCleanup struct {
	repo   ExpiredRecordsRepository
	tables []string
	now    func() time.Time
}

func NewExpiredRecordsCleanup(repo ExpiredRecordsRepository, tables []string) *ExpiredRecordsCleanup {
	return &ExpiredRecordsCleanup{repo: repo, tables: tables, now: time.Now}
}

func (j *ExpiredRecordsCleanup) Run(ctx context.Context) {
	j.RunOnce(ctx)

	ticker := time.NewTicker(expiredRecordsCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.RunOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// RunOnce cleans every table, batch after batch, until a batch comes back
// short. A failure on one table is logged and does not stop the others.
func (j *ExpiredRecordsCleanup) RunOnce(ctx context.Context) {
	now := j.now().UTC()

	for _, table := range j.tables {
		var total int64
		for {
			if ctx.Err() != nil {
				return
			}
			deleted, err := j.repo.DeleteExpiredBatch(ctx, table, now, expiredRecordsBatchSize)
			if err != nil {
				slog.ErrorContext(ctx, "expired records cleanup failed", "table", table, "error", err)
				break
			}
			total += deleted
			if deleted < expiredRecordsBatchSize {
				break
			}
		}
		if total > 0 {
			slog.InfoContext(ctx, "expired records deleted", "table", table, "deleted_count", total)
		}
	}
}
