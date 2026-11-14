package jobs

import (
	"context"
	"log/slog"
	"time"
)

const passwordRegistrationCleanupInterval = 5 * time.Minute

type PasswordRegistrationAttemptRepository interface {
	DeleteObsoletePasswordRegistrationAttempts(ctx context.Context, now time.Time) (int64, error)
}

type PasswordRegistrationCleanup struct {
	repo PasswordRegistrationAttemptRepository
}

func NewPasswordRegistrationCleanup(repo PasswordRegistrationAttemptRepository) *PasswordRegistrationCleanup {
	return &PasswordRegistrationCleanup{
		repo: repo,
	}
}

func (j *PasswordRegistrationCleanup) Run(ctx context.Context) {
	j.runOnce(ctx)

	ticker := time.NewTicker(passwordRegistrationCleanupInterval)
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

func (j *PasswordRegistrationCleanup) runOnce(ctx context.Context) {
	deleted, err := j.repo.DeleteObsoletePasswordRegistrationAttempts(ctx, time.Now().UTC())

	if err != nil {
		slog.ErrorContext(ctx, "password registration cleanup failed", "error", err)
		return
	}

	if deleted > 0 {
		slog.InfoContext(ctx, "obsolete password registration attempts deleted", "deleted_count", deleted)
	}
}
