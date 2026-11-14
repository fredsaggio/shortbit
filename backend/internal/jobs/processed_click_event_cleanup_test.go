package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

type processedClickEventRepositoryStub struct {
	deleteBeforeFunc func(context.Context, time.Time) (int64, error)
}

func (s processedClickEventRepositoryStub) DeleteProcessedClickEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.deleteBeforeFunc(ctx, cutoff)
}

func TestProcessedClickEventCleanupRunOnce(t *testing.T) {
	const retention = 7 * 24 * time.Hour
	before := time.Now().UTC().Add(-retention)
	called := 0

	repository := processedClickEventRepositoryStub{deleteBeforeFunc: func(_ context.Context, cutoff time.Time) (int64, error) {
		called++
		after := time.Now().UTC().Add(-retention)
		if cutoff.Before(before) || cutoff.After(after) {
			t.Errorf("cutoff = %v, want between %v and %v", cutoff, before, after)
		}
		if cutoff.Location() != time.UTC {
			t.Errorf("cutoff location = %v, want UTC", cutoff.Location())
		}
		return 3, nil
	}}

	job := NewProcessedClickEventCleanup(repository, retention)
	job.runOnce(context.Background())

	if called != 1 {
		t.Errorf("DeleteProcessedClickEventsBefore() calls = %d, want 1", called)
	}
}

func TestProcessedClickEventCleanupRunOnceHandlesRepositoryError(t *testing.T) {
	called := 0
	repository := processedClickEventRepositoryStub{deleteBeforeFunc: func(context.Context, time.Time) (int64, error) {
		called++
		return 0, errors.New("database unavailable")
	}}

	job := NewProcessedClickEventCleanup(repository, time.Hour)
	job.runOnce(context.Background())

	if called != 1 {
		t.Errorf("DeleteProcessedClickEventsBefore() calls = %d, want 1", called)
	}
}
