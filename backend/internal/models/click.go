package models

import (
	"time"
	"uuid"
)

// ClickSource tells how a visitor reached a short link.
type ClickSource string

const (
	// ClickSourceLink is someone opening the short link.
	ClickSourceLink ClickSource = "link"
	// ClickSourceQR is someone scanning the link's QR code, which carries
	// the ?qr=1 marker.
	ClickSourceQR ClickSource = "qr"
)

func (s ClickSource) Valid() bool {
	return s == ClickSourceLink || s == ClickSourceQR
}

// Click is one effective redirect to count. EventID is the idempotency key.
type Click struct {
	EventID    uuid.UUID
	URLID      int64
	Source     ClickSource
	OccurredAt time.Time
}

// DailyClickCount is the number of clicks of one source on one day.
type DailyClickCount struct {
	Day    time.Time
	Source ClickSource
	Clicks int64
}
