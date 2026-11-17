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

// VisitorHashSize is the length of a visitor hash, in bytes (ADR 0007).
const VisitorHashSize = 16

// Visit is what the redirect knows about who opened a link. IP and
// UserAgent are read during the request only: they never reach the queue,
// the database or the logs.
type Visit struct {
	Source    ClickSource
	IP        string
	UserAgent string
}

// VisitDetails is what is kept from a visit (ADR 0007). A robot carries no
// country and no visitor hash. Country is an ISO 3166-1 alpha-2 code, empty
// when unknown. Visitor is the day's salted hash of the visitor, empty when
// the salt was unavailable.
type VisitDetails struct {
	Robot   bool
	Country string
	Visitor []byte
}

// Click is one effective redirect to count. EventID is the idempotency key.
type Click struct {
	EventID    uuid.UUID
	URLID      int64
	Source     ClickSource
	OccurredAt time.Time
	VisitDetails
}

// ValidCountry reports whether code is empty or two uppercase ASCII letters.
func ValidCountry(code string) bool {
	if code == "" {
		return true
	}
	return len(code) == 2 && code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z'
}

// Valid reports whether the details can be stored as they are.
func (d VisitDetails) Valid() bool {
	if d.Robot {
		return d.Country == "" && len(d.Visitor) == 0
	}
	return ValidCountry(d.Country) && (len(d.Visitor) == 0 || len(d.Visitor) == VisitorHashSize)
}

// DailyClickCount is the number of clicks of one source on one day.
type DailyClickCount struct {
	Day    time.Time
	Source ClickSource
	Clicks int64
}

// DailyAudience is the unique visitors and robot visits of one day.
type DailyAudience struct {
	Day            time.Time
	UniqueVisitors int64
	RobotVisits    int64
}

// CountryClicks is the number of clicks from one country.
type CountryClicks struct {
	Country string
	Clicks  int64
}
