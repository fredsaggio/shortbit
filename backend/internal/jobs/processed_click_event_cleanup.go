package jobs

import (
	"context"
	"log/slog"
	"time"
)

const processedClickEventCleanupInterval = time.Hour

type ProcessedClickEventRepository interface {
	DeleteProcessedClickEventsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// The retention must exceed the longest time an event can wait in the queue,
// or a late redelivery would be counted again.
type ProcessedClickEventCleanup struct {
	repo      ProcessedClickEventRepository
	retention time.Duration
}

func NewProcessedClickEventCleanup(repo ProcessedClickEventRepository, retention time.Duration) *ProcessedClickEventCleanup {
	return &ProcessedClickEventCleanup{
		repo:      repo,
		retention: retention,
	}
}

func (j *ProcessedClickEventCleanup) Run(ctx context.Context) {
	j.runOnce(ctx)

	ticker := time.NewTicker(processedClickEventCleanupInterval)
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

func (j *ProcessedClickEventCleanup) runOnce(ctx context.Context) {
	deleted, err := j.repo.DeleteProcessedClickEventsBefore(ctx, time.Now().UTC().Add(-j.retention))

	if err != nil {
		slog.ErrorContext(ctx, "processed click event cleanup failed", "error", err)
		return
	}

	if deleted > 0 {
		slog.InfoContext(ctx, "old processed click events deleted", "deleted_count", deleted)
	}
}
