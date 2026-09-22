package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

type passwordRegistrationAttemptRepositoryStub struct {
	deleteObsoleteFunc func(context.Context, time.Time) (int64, error)
}

func (s passwordRegistrationAttemptRepositoryStub) DeleteObsoletePasswordRegistrationAttempts(ctx context.Context, now time.Time) (int64, error) {
	return s.deleteObsoleteFunc(ctx, now)
}

func TestPasswordRegistrationCleanupRunOnce(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "cleanup-context")
	before := time.Now().UTC()
	called := 0

	repository := passwordRegistrationAttemptRepositoryStub{deleteObsoleteFunc: func(gotCtx context.Context, now time.Time) (int64, error) {
		called++
		if got := gotCtx.Value(contextKey{}); got != "cleanup-context" {
			t.Errorf("DeleteObsoletePasswordRegistrationAttempts() context value = %v, want %q", got, "cleanup-context")
		}
		if now.Location() != time.UTC {
			t.Errorf("cleanup time location = %v, want UTC", now.Location())
		}
		if now.Before(before) {
			t.Errorf("cleanup time = %v, want at or after %v", now, before)
		}
		return 2, nil
	}}

	job := NewPasswordRegistrationCleanup(repository)
	job.runOnce(ctx)

	if called != 1 {
		t.Errorf("DeleteObsoletePasswordRegistrationAttempts() calls = %d, want 1", called)
	}
}

func TestPasswordRegistrationCleanupRunOnceHandlesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	called := 0
	repository := passwordRegistrationAttemptRepositoryStub{deleteObsoleteFunc: func(context.Context, time.Time) (int64, error) {
		called++
		return 0, wantErr
	}}

	job := NewPasswordRegistrationCleanup(repository)
	job.runOnce(context.Background())

	if called != 1 {
		t.Errorf("DeleteObsoletePasswordRegistrationAttempts() calls = %d, want 1", called)
	}
}

func TestPasswordRegistrationCleanupRunExecutesImmediatelyAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := make(chan struct{}, 1)
	repository := passwordRegistrationAttemptRepositoryStub{deleteObsoleteFunc: func(context.Context, time.Time) (int64, error) {
		called <- struct{}{}
		cancel()
		return 0, nil
	}}

	job := NewPasswordRegistrationCleanup(repository)
	done := make(chan struct{})
	go func() {
		defer close(done)
		job.Run(ctx)
	}()

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("Run() did not execute cleanup immediately")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}
