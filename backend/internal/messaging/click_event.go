package messaging

import (
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/models"
)

// ClickEvent is one effective redirect. EventID is the idempotency key: the
// worker and the synchronous fallback both record it, so a click is counted
// once even if the event is redelivered or its confirmation timed out.
type ClickEvent struct {
	EventID    uuid.UUID          `json:"event_id"`
	URLID      int64              `json:"url_id"`
	OccurredAt time.Time          `json:"occurred_at"`
	Source     models.ClickSource `json:"source,omitempty"`
	// Robot, Country and Visitor are absent from events published before
	// ADR 0007; such an event counts as a person with no country.
	Robot   bool   `json:"robot,omitempty"`
	Country string `json:"country,omitempty"`
	Visitor []byte `json:"visitor,omitempty"`
}

func NewClickEvent(urlID int64, source models.ClickSource, occurredAt time.Time) ClickEvent {
	return ClickEvent{
		EventID:    uuid.NewV7(),
		URLID:      urlID,
		OccurredAt: occurredAt.UTC(),
		Source:     source,
	}
}

// WithDetails returns the event carrying what was kept from the visit.
func (e ClickEvent) WithDetails(details models.VisitDetails) ClickEvent {
	e.Robot = details.Robot
	e.Country = details.Country
	e.Visitor = details.Visitor
	return e
}

// Click is the event as a click to count. Events published before the
// source existed have none and count as the short link.
func (e ClickEvent) Click() models.Click {
	source := e.Source
	if source == "" {
		source = models.ClickSourceLink
	}
	return models.Click{
		EventID:      e.EventID,
		URLID:        e.URLID,
		Source:       source,
		OccurredAt:   e.OccurredAt,
		VisitDetails: models.VisitDetails{Robot: e.Robot, Country: e.Country, Visitor: e.Visitor},
	}
}
