package services_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type fakeClickEventStore struct {
	counted bool
	err     error

	gotEventID uuid.UUID
	gotURLID   int64
}

func (s *fakeClickEventStore) CountClickOnce(_ context.Context, eventID uuid.UUID, urlID int64) (bool, error) {
	s.gotEventID = eventID
	s.gotURLID = urlID
	return s.counted, s.err
}

func TestClickCountServiceHandleClick(t *testing.T) {
	storeErr := errors.New("database unavailable")

	tests := []struct {
		name    string
		store   *fakeClickEventStore
		wantErr error
	}{
		{name: "counts a new event", store: &fakeClickEventStore{counted: true}},
		{name: "accepts a duplicate event", store: &fakeClickEventStore{counted: false}},
		{name: "discards an event for a missing URL", store: &fakeClickEventStore{err: repositories.ErrURLNotFound}},
		{name: "returns a store failure for retry", store: &fakeClickEventStore{err: storeErr}, wantErr: storeErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := messaging.NewClickEvent(42, time.Now())

			err := services.NewClickCountService(tt.store).HandleClick(t.Context(), event)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("HandleClick() error = %v, want %v", err, tt.wantErr)
			}
			if tt.store.gotEventID != event.EventID || tt.store.gotURLID != event.URLID {
				t.Errorf("store received (%v, %d), want (%v, %d)", tt.store.gotEventID, tt.store.gotURLID, event.EventID, event.URLID)
			}
		})
	}
}
