package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/telemetry"
)

type ClickEventStore interface {
	CountClickOnce(ctx context.Context, click models.Click) (bool, error)
}

type ClickCountService struct {
	store ClickEventStore
}

func NewClickCountService(store ClickEventStore) *ClickCountService {
	return &ClickCountService{store: store}
}

func (s *ClickCountService) HandleClick(ctx context.Context, event messaging.ClickEvent) error {
	counted, err := s.store.CountClickOnce(ctx, event.Click())
	if err != nil {
		// Retrying an event for a missing URL would never succeed.
		if errors.Is(err, repositories.ErrURLNotFound) {
			slog.WarnContext(ctx, "click event for missing URL discarded", "event_id", event.EventID, "url_id", event.URLID)
			telemetry.RecordClickProcessed(ctx, telemetry.ClickDiscarded, event.OccurredAt)
			return nil
		}
		telemetry.RecordClickProcessed(ctx, telemetry.ClickFailed, event.OccurredAt)
		return fmt.Errorf("count click event: %w", err)
	}

	if !counted {
		slog.InfoContext(ctx, "duplicate click event ignored", "event_id", event.EventID, "url_id", event.URLID)
		telemetry.RecordClickProcessed(ctx, telemetry.ClickDuplicate, event.OccurredAt)
		return nil
	}

	telemetry.RecordClickProcessed(ctx, telemetry.ClickCounted, event.OccurredAt)
	return nil
}
