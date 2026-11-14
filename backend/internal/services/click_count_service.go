package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
)

type ClickEventStore interface {
	CountClickOnce(ctx context.Context, eventID uuid.UUID, urlID int64) (bool, error)
}

type ClickCountService struct {
	store ClickEventStore
}

func NewClickCountService(store ClickEventStore) *ClickCountService {
	return &ClickCountService{store: store}
}

func (s *ClickCountService) HandleClick(ctx context.Context, event messaging.ClickEvent) error {
	counted, err := s.store.CountClickOnce(ctx, event.EventID, event.URLID)
	if err != nil {
		// Retrying an event for a missing URL would never succeed.
		if errors.Is(err, repositories.ErrURLNotFound) {
			slog.WarnContext(ctx, "click event for missing URL discarded", "event_id", event.EventID, "url_id", event.URLID)
			return nil
		}
		return fmt.Errorf("count click event: %w", err)
	}

	if !counted {
		slog.InfoContext(ctx, "duplicate click event ignored", "event_id", event.EventID, "url_id", event.URLID)
	}

	return nil
}
