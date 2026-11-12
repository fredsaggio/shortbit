package messaging

import (
	"time"
	"uuid"
)

// ClickEvent is one effective redirect. EventID is the idempotency key: the
// worker and the synchronous fallback both record it, so a click is counted
// once even if the event is redelivered or its confirmation timed out.
type ClickEvent struct {
	EventID    uuid.UUID `json:"event_id"`
	URLID      int64     `json:"url_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewClickEvent(urlID int64, occurredAt time.Time) ClickEvent {
	return ClickEvent{
		EventID:    uuid.NewV7(),
		URLID:      urlID,
		OccurredAt: occurredAt.UTC(),
	}
}
