package messaging_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/fredsaggio/url-shortener/internal/models"
)

func TestClickEventCarriesVisitDetails(t *testing.T) {
	hash := bytes.Repeat([]byte{7}, models.VisitorHashSize)
	event := messaging.NewClickEvent(42, models.ClickSourceQR, time.Now()).WithDetails(models.VisitDetails{Country: "BR", Visitor: hash})

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded messaging.ClickEvent
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}

	click := decoded.Click()
	if click.Robot || click.Country != "BR" || !bytes.Equal(click.Visitor, hash) || click.Source != models.ClickSourceQR {
		t.Errorf("Click() = %+v, want the QR code click from BR with its visitor", click)
	}
}

// Events published before ADR 0007 have no visit details: they count as a
// person with no country and no visitor.
func TestClickEventWithoutDetailsIsAPerson(t *testing.T) {
	body := []byte(`{"event_id":"0193a0b4-7c3e-7d2a-9b1e-3f4a5b6c7d8e","url_id":42,"occurred_at":"2026-11-16T15:00:00Z","source":"link"}`)
	var event messaging.ClickEvent
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatal(err)
	}

	click := event.Click()
	if click.Robot || click.Country != "" || click.Visitor != nil || !click.VisitDetails.Valid() {
		t.Errorf("Click() = %+v, want a valid person without details", click)
	}
}
