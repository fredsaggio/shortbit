package messaging_test

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/messaging"
)

func TestNewClickEvent(t *testing.T) {
	occurredAt := time.Date(2026, 11, 12, 11, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	event := messaging.NewClickEvent(42, occurredAt)

	if event.URLID != 42 {
		t.Errorf("URLID = %d, want 42", event.URLID)
	}

	if !event.OccurredAt.Equal(occurredAt) || event.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt = %v, want %v in UTC", event.OccurredAt, occurredAt)
	}

	if version := event.EventID[6] >> 4; version != 7 {
		t.Errorf("EventID version = %d, want 7", version)
	}

	if other := messaging.NewClickEvent(42, occurredAt); other.EventID == event.EventID {
		t.Error("NewClickEvent() repeated the EventID")
	}

	if event.EventID == uuid.Nil() {
		t.Error("EventID is nil")
	}
}

func TestClickPublisherPublishWithoutConnection(t *testing.T) {
	publisher := messaging.NewClickPublisher("amqp://user:password@localhost:5672/", time.Second)

	err := publisher.Publish(t.Context(), messaging.NewClickEvent(1, time.Now()))

	if !errors.Is(err, messaging.ErrPublisherUnavailable) {
		t.Fatalf("Publish() error = %v, want %v", err, messaging.ErrPublisherUnavailable)
	}
}
