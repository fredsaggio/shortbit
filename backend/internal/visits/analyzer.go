package visits

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
)

// SaltSize is the length of the daily visitor salt, in bytes.
const SaltSize = 32

// saltRetryAfter spaces out reads of the salt after one fails, so a database
// outage does not add a failing query to every redirect.
const saltRetryAfter = 30 * time.Second

// CountryLocator returns the country code of an IP, or "" when unknown.
type CountryLocator interface {
	Country(ip string) string
}

// SaltStore keeps one random salt per day, shared by every API instance.
// DailySalt stores candidate as the salt of day unless the day already has
// one, and returns the salt that the day ends up with.
type SaltStore interface {
	DailySalt(ctx context.Context, day time.Time, candidate []byte) ([]byte, error)
}

// Analyzer turns a visit into what may be kept about it (ADR 0007).
type Analyzer struct {
	countries CountryLocator
	salts     SaltStore
	location  *time.Location

	now func() time.Time

	mu         sync.Mutex
	saltDay    string
	salt       []byte
	retryAfter time.Time
}

// NewAnalyzer analyzes visits in the day of location. A nil countries keeps
// no country; a nil salts keeps no visitor hash.
func NewAnalyzer(countries CountryLocator, salts SaltStore, location *time.Location) *Analyzer {
	return &Analyzer{countries: countries, salts: salts, location: location, now: time.Now}
}

func (a *Analyzer) Analyze(ctx context.Context, urlID int64, visit models.Visit, at time.Time) models.VisitDetails {
	if visit.Head || IsRobot(visit.UserAgent) {
		return models.VisitDetails{Robot: true}
	}

	var details models.VisitDetails
	if a.countries != nil {
		details.Country = a.countries.Country(visit.IP)
	}
	if salt := a.dailySalt(ctx, at); salt != nil {
		details.Visitor = VisitorHash(salt, urlID, visit.IP, visit.UserAgent)
	}
	return details
}

// dailySalt returns the salt of the day of at, or nil when it cannot be
// read: the click still counts, only not as a visitor. The salt stays in
// memory until the day turns.
func (a *Analyzer) dailySalt(ctx context.Context, at time.Time) []byte {
	if a.salts == nil {
		return nil
	}

	local := at.In(a.location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	key := day.Format(time.DateOnly)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.saltDay == key {
		return a.salt
	}
	if a.now().Before(a.retryAfter) {
		return nil
	}

	candidate := make([]byte, SaltSize)
	rand.Read(candidate)
	salt, err := a.salts.DailySalt(ctx, day, candidate)
	if err == nil && len(salt) != SaltSize {
		err = fmt.Errorf("stored salt has %d bytes, want %d", len(salt), SaltSize)
	}
	if err != nil {
		slog.WarnContext(ctx, "read visitor salt failed; clicks counted without visitor for a while", "error", err)
		a.retryAfter = a.now().Add(saltRetryAfter)
		return nil
	}

	a.saltDay, a.salt = key, salt
	return salt
}

// VisitorHash identifies a visitor of one link on one day. Without the salt,
// which is deleted when the day ends, the hash cannot be rebuilt from an IP.
func VisitorHash(salt []byte, urlID int64, ip, userAgent string) []byte {
	mac := hmac.New(sha256.New, salt)
	var id [8]byte
	binary.BigEndian.PutUint64(id[:], uint64(urlID))
	mac.Write(id[:])
	mac.Write([]byte(ip))
	// The separator keeps "1.2.3.4" + "5x" apart from "1.2.3.45" + "x".
	mac.Write([]byte{0})
	mac.Write([]byte(userAgent))
	return mac.Sum(nil)[:models.VisitorHashSize]
}
