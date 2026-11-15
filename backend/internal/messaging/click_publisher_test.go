package messaging_test

import (
	"encoding/json"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/models"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/messaging"
)

func TestNewClickEvent(t *testing.T) {
	occurredAt := time.Date(2026, 11, 12, 11, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	event := messaging.NewClickEvent(42, models.ClickSourceLink, occurredAt)

	if event.URLID != 42 {
		t.Errorf("URLID = %d, want 42", event.URLID)
	}

	if !event.OccurredAt.Equal(occurredAt) || event.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt = %v, want %v in UTC", event.OccurredAt, occurredAt)
	}

	if version := event.EventID[6] >> 4; version != 7 {
		t.Errorf("EventID version = %d, want 7", version)
	}

	if other := messaging.NewClickEvent(42, models.ClickSourceLink, occurredAt); other.EventID == event.EventID {
		t.Error("NewClickEvent() repeated the EventID")
	}

	if event.EventID == uuid.Nil() {
		t.Error("EventID is nil")
	}
}

func TestClickEventSource(t *testing.T) {
	qr := messaging.NewClickEvent(42, models.ClickSourceQR, time.Now())
	if click := qr.Click(); click.Source != models.ClickSourceQR || click.URLID != 42 || click.EventID != qr.EventID {
		t.Errorf("Click() = %+v, want the QR code click of the event", click)
	}

	// An event published before the source existed counts as the short link.
	var old messaging.ClickEvent
	if err := json.Unmarshal([]byte(`{"event_id":"`+qr.EventID.String()+`","url_id":42,"occurred_at":"2026-11-14T12:00:00Z"}`), &old); err != nil {
		t.Fatalf("decode old event: %v", err)
	}
	if got := old.Click().Source; got != models.ClickSourceLink {
		t.Errorf("old event source = %q, want link", got)
	}

	encoded, err := json.Marshal(qr)
	if err != nil || !strings.Contains(string(encoded), `"source":"qr"`) {
		t.Errorf("encoded event = %s (%v), want the source in it", encoded, err)
	}
}

func TestClickPublisherPublishWithoutConnection(t *testing.T) {
	publisher := messaging.NewClickPublisher("amqp://user:password@localhost:5672/", time.Second)

	err := publisher.Publish(t.Context(), messaging.NewClickEvent(1, models.ClickSourceLink, time.Now()))

	if !errors.Is(err, messaging.ErrPublisherUnavailable) {
		t.Fatalf("Publish() error = %v, want %v", err, messaging.ErrPublisherUnavailable)
	}
}
