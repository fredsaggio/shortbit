package visits

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/models"
)

const browserAgent = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36"

type countriesStub map[string]string

func (c countriesStub) Country(ip string) string { return c[ip] }

type saltStoreStub struct {
	err   error
	days  []time.Time
	salts map[string][]byte
}

func (s *saltStoreStub) DailySalt(_ context.Context, day time.Time, candidate []byte) ([]byte, error) {
	s.days = append(s.days, day)
	if s.err != nil {
		return nil, s.err
	}
	key := day.Format(time.DateOnly)
	if s.salts == nil {
		s.salts = map[string][]byte{}
	}
	if _, ok := s.salts[key]; !ok {
		s.salts[key] = candidate
	}
	return s.salts[key], nil
}

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func TestAnalyzeRobotKeepsNothingElse(t *testing.T) {
	salts := &saltStoreStub{}
	analyzer := NewAnalyzer(countriesStub{"203.0.113.7": "BR"}, salts, saoPaulo(t))

	got := analyzer.Analyze(t.Context(), 42, models.Visit{IP: "203.0.113.7", UserAgent: "WhatsApp/2.23.20.0 A"}, time.Now())

	if !got.Robot || got.Country != "" || got.Visitor != nil {
		t.Errorf("Analyze() = %+v, want a robot without country or visitor", got)
	}
	if len(salts.days) != 0 {
		t.Errorf("salt read for a robot")
	}
}

func TestAnalyzePersonGetsCountryAndVisitor(t *testing.T) {
	analyzer := NewAnalyzer(countriesStub{"203.0.113.7": "BR"}, &saltStoreStub{}, saoPaulo(t))
	visit := models.Visit{IP: "203.0.113.7", UserAgent: browserAgent}
	at := time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)

	first := analyzer.Analyze(t.Context(), 42, visit, at)
	again := analyzer.Analyze(t.Context(), 42, visit, at.Add(time.Hour))
	otherLink := analyzer.Analyze(t.Context(), 43, visit, at)
	otherBrowser := analyzer.Analyze(t.Context(), 42, models.Visit{IP: visit.IP, UserAgent: browserAgent + " Firefox"}, at)

	if first.Robot || first.Country != "BR" || len(first.Visitor) != models.VisitorHashSize {
		t.Fatalf("Analyze() = %+v, want a person from BR with a visitor hash", first)
	}
	if !bytes.Equal(first.Visitor, again.Visitor) {
		t.Error("the same visitor on the same day got different hashes")
	}
	if bytes.Equal(first.Visitor, otherLink.Visitor) {
		t.Error("the same visitor got the same hash on two links")
	}
	if bytes.Equal(first.Visitor, otherBrowser.Visitor) {
		t.Error("two browsers got the same hash")
	}
}

func TestAnalyzeUsesANewSaltEachLocalDay(t *testing.T) {
	salts := &saltStoreStub{}
	analyzer := NewAnalyzer(nil, salts, saoPaulo(t))
	visit := models.Visit{IP: "203.0.113.7", UserAgent: browserAgent}

	// 23:30 and 23:50 in São Paulo on November 16, then 00:10 on November 17.
	evening := analyzer.Analyze(t.Context(), 42, visit, time.Date(2026, 11, 17, 2, 30, 0, 0, time.UTC))
	later := analyzer.Analyze(t.Context(), 42, visit, time.Date(2026, 11, 17, 2, 50, 0, 0, time.UTC))
	nextDay := analyzer.Analyze(t.Context(), 42, visit, time.Date(2026, 11, 17, 3, 10, 0, 0, time.UTC))

	if evening.Country != "" {
		t.Errorf("country = %q without a database, want empty", evening.Country)
	}
	if !bytes.Equal(evening.Visitor, later.Visitor) {
		t.Error("the same local day got different hashes")
	}
	if bytes.Equal(evening.Visitor, nextDay.Visitor) {
		t.Error("the next local day kept the same hash")
	}
	wantDays := []time.Time{time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)}
	if len(salts.days) != 2 || !salts.days[0].Equal(wantDays[0]) || !salts.days[1].Equal(wantDays[1]) {
		t.Errorf("salt days = %v, want %v (one read per day)", salts.days, wantDays)
	}
}

func TestAnalyzeWithoutSaltCountsWithoutVisitorAndWaitsToRetry(t *testing.T) {
	salts := &saltStoreStub{err: errors.New("database unavailable")}
	analyzer := NewAnalyzer(countriesStub{"203.0.113.7": "BR"}, salts, saoPaulo(t))
	now := time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC)
	analyzer.now = func() time.Time { return now }
	visit := models.Visit{IP: "203.0.113.7", UserAgent: browserAgent}

	first := analyzer.Analyze(t.Context(), 42, visit, now)
	analyzer.Analyze(t.Context(), 42, visit, now)

	if first.Robot || first.Country != "BR" || first.Visitor != nil {
		t.Errorf("Analyze() = %+v, want a person from BR without visitor", first)
	}
	if len(salts.days) != 1 {
		t.Errorf("salt reads = %d right after a failure, want 1", len(salts.days))
	}

	salts.err = nil
	now = now.Add(saltRetryAfter)
	if got := analyzer.Analyze(t.Context(), 42, visit, now); got.Visitor == nil {
		t.Error("no visitor after the retry delay")
	}
}

func TestAnalyzeRejectsASaltOfTheWrongSize(t *testing.T) {
	salts := &saltStoreStub{salts: map[string][]byte{"2026-11-16": []byte("short")}}
	analyzer := NewAnalyzer(nil, salts, saoPaulo(t))

	got := analyzer.Analyze(t.Context(), 42, models.Visit{IP: "203.0.113.7", UserAgent: browserAgent}, time.Date(2026, 11, 16, 15, 0, 0, 0, time.UTC))

	if got.Visitor != nil {
		t.Errorf("visitor = %x with a short salt, want none", got.Visitor)
	}
}
