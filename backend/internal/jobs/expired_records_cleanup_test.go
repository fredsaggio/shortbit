package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

type expiredRecordsStub struct {
	batches map[string][]int64
	errs    map[string]error
	calls   map[string]int
	nows    []time.Time
}

func (s *expiredRecordsStub) DeleteExpiredBatch(_ context.Context, table string, now time.Time, limit int) (int64, error) {
	if limit != expiredRecordsBatchSize {
		panic("unexpected batch size")
	}
	s.nows = append(s.nows, now)
	s.calls[table]++
	if err := s.errs[table]; err != nil {
		return 0, err
	}
	batches := s.batches[table]
	if len(batches) == 0 {
		return 0, nil
	}
	s.batches[table] = batches[1:]
	return batches[0], nil
}

func TestExpiredRecordsCleanupDeletesInBatchesUntilClean(t *testing.T) {
	repo := &expiredRecordsStub{
		batches: map[string][]int64{
			"user_sessions":        {expiredRecordsBatchSize, expiredRecordsBatchSize, 3},
			"link_access_sessions": {0},
		},
		errs:  map[string]error{},
		calls: map[string]int{},
	}
	fixed := time.Date(2026, 11, 15, 12, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	job := NewExpiredRecordsCleanup(repo, []string{"user_sessions", "link_access_sessions"})
	job.now = func() time.Time { return fixed }

	job.RunOnce(t.Context())

	if repo.calls["user_sessions"] != 3 {
		t.Errorf("user_sessions batches = %d, want 3 (two full, one short)", repo.calls["user_sessions"])
	}
	if repo.calls["link_access_sessions"] != 1 {
		t.Errorf("link_access_sessions batches = %d, want 1", repo.calls["link_access_sessions"])
	}
	for _, now := range repo.nows {
		if !now.Equal(fixed) || now.Location() != time.UTC {
			t.Errorf("cutoff = %v, want the same instant in UTC for every batch", now)
		}
	}
}

func TestExpiredRecordsCleanupKeepsGoingAfterAFailure(t *testing.T) {
	repo := &expiredRecordsStub{
		batches: map[string][]int64{"link_access_sessions": {2}},
		errs:    map[string]error{"user_sessions": errors.New("database unavailable")},
		calls:   map[string]int{},
	}
	job := NewExpiredRecordsCleanup(repo, []string{"user_sessions", "link_access_sessions"})

	job.RunOnce(t.Context())

	if repo.calls["user_sessions"] != 1 || repo.calls["link_access_sessions"] != 1 {
		t.Errorf("calls = %v, want one per table despite the failure", repo.calls)
	}
}

func TestExpiredRecordsCleanupStopsWhenCanceled(t *testing.T) {
	repo := &expiredRecordsStub{batches: map[string][]int64{}, errs: map[string]error{}, calls: map[string]int{}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	NewExpiredRecordsCleanup(repo, []string{"user_sessions"}).RunOnce(ctx)

	if repo.calls["user_sessions"] != 0 {
		t.Error("cleanup ran after cancellation")
	}
}
