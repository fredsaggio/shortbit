package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

type visitorDataRepositoryStub func(context.Context, time.Time) (int64, int64, error)

func (s visitorDataRepositoryStub) DeleteVisitorDataBefore(ctx context.Context, today time.Time) (int64, int64, error) {
	return s(ctx, today)
}

func TestVisitorDataCleanupUsesTheLocalDay(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	var got []time.Time
	job := NewVisitorDataCleanup(visitorDataRepositoryStub(func(_ context.Context, today time.Time) (int64, int64, error) {
		got = append(got, today)
		return 1, 2, nil
	}), saoPaulo)
	// 01:30 UTC on November 17 is still November 16 in São Paulo (UTC-3).
	job.now = func() time.Time { return time.Date(2026, 11, 17, 1, 30, 0, 0, time.UTC) }

	job.runOnce(t.Context())

	want := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	if len(got) != 1 || !got[0].Equal(want) {
		t.Errorf("DeleteVisitorDataBefore() days = %v, want [%v]", got, want)
	}
}

func TestVisitorDataCleanupHandlesRepositoryError(t *testing.T) {
	called := 0
	job := NewVisitorDataCleanup(visitorDataRepositoryStub(func(context.Context, time.Time) (int64, int64, error) {
		called++
		return 0, 0, errors.New("database unavailable")
	}), time.UTC)

	job.runOnce(t.Context())

	if called != 1 {
		t.Errorf("DeleteVisitorDataBefore() calls = %d, want 1", called)
	}
}
