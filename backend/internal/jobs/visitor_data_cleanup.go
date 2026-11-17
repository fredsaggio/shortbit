package jobs

import (
	"context"
	"log/slog"
	"time"
)

const visitorDataCleanupInterval = time.Hour

type VisitorDataRepository interface {
	DeleteVisitorDataBefore(ctx context.Context, today time.Time) (int64, int64, error)
}

// VisitorDataCleanup deletes, every hour, the visitor salts of past days and
// the visitor hashes of days before yesterday (ADR 0007). A salt thus
// outlives its day by an hour at most.
type VisitorDataCleanup struct {
	repo     VisitorDataRepository
	location *time.Location
	now      func() time.Time
}

// NewVisitorDataCleanup counts days in location, the zone of the click day.
func NewVisitorDataCleanup(repo VisitorDataRepository, location *time.Location) *VisitorDataCleanup {
	return &VisitorDataCleanup{repo: repo, location: location, now: time.Now}
}

func (j *VisitorDataCleanup) Run(ctx context.Context) {
	j.runOnce(ctx)

	ticker := time.NewTicker(visitorDataCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.runOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (j *VisitorDataCleanup) runOnce(ctx context.Context) {
	local := j.now().In(j.location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	salts, hashes, err := j.repo.DeleteVisitorDataBefore(ctx, today)
	if err != nil {
		slog.ErrorContext(ctx, "visitor data cleanup failed", "error", err)
		return
	}

	if salts > 0 || hashes > 0 {
		slog.InfoContext(ctx, "old visitor data deleted", "salt_count", salts, "visitor_hash_count", hashes)
	}
}
