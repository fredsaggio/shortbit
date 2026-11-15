//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/problem/problemtest"
	"github.com/fredsaggio/url-shortener/internal/server"
)

type clickStatsBody struct {
	Days []struct {
		Date string `json:"date"`
		Link int64  `json:"link"`
		QR   int64  `json:"qr"`
	} `json:"days"`
	Totals struct {
		Link int64 `json:"link"`
		QR   int64 `json:"qr"`
	} `json:"totals"`
}

func TestDailyClicksByLinkAndQRCodeIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, "clicks-owner@example.com", "senha12345")
	createConfirmedPasswordUser(t, pool, "clicks-other@example.com", "senha12345")

	cfg := testConfig(24 * time.Hour)
	cfg.LinkAccessSession.TTL = 30 * time.Minute
	// The unavailable publisher makes every click count synchronously, so the
	// numbers are there as soon as the redirect answers.
	handlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()
	owner := loginAs(t, router, "clicks-owner@example.com")
	other := loginAs(t, router, "clicks-other@example.com")

	public := createPublicURL(t, router, owner)
	for _, path := range []string{public, public, public + "?qr=1", public + "?qr=1", public + "?qr=1"} {
		response := openLink(t, router, path, nil)
		response.Body.Close()
		if response.StatusCode != http.StatusFound {
			t.Fatalf("open %s status = %d, want 302", path, response.StatusCode)
		}
	}

	// A private link scanned from its QR code: the password form and the
	// redirect after it keep the marker, so the click counts as QR.
	private := createPublicURL(t, router, owner)
	if status, _ := patchURL(t, router, private, `{"visibility":"private","password":"senha-do-link"}`, owner); status != http.StatusOK {
		t.Fatalf("make private status = %d", status)
	}
	form := url.Values{"password": {"senha-do-link"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+private+"/access?qr=1", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	unlocked := recorder.Result()
	unlocked.Body.Close()
	if unlocked.StatusCode != http.StatusSeeOther || unlocked.Header.Get("Location") != "/"+private+"?qr=1" {
		t.Fatalf("unlock = %d %q, want 303 back to the link with the QR marker", unlocked.StatusCode, unlocked.Header.Get("Location"))
	}
	cookie := requireCookie(t, unlocked.Cookies(), "link_access_session")
	opened := openLink(t, router, private+"?qr=1", cookie)
	opened.Body.Close()
	if opened.StatusCode != http.StatusFound {
		t.Fatalf("open unlocked private link status = %d, want 302", opened.StatusCode)
	}

	stats := func(code, query string, cookie *http.Cookie) (*http.Response, clickStatsBody) {
		t.Helper()
		response := performRequestWithCookie(t, router, http.MethodGet, "/urls/"+code+"/clicks"+query, cookie)
		var body clickStatsBody
		if response.StatusCode == http.StatusOK {
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode stats: %v", err)
			}
		}
		response.Body.Close()
		return response, body
	}

	today := time.Now().In(mustLoadLocation(t, "America/Sao_Paulo")).Format(time.DateOnly)

	response, body := stats(public, "?days=7", owner)
	if response.StatusCode != http.StatusOK || len(body.Days) != 7 {
		t.Fatalf("stats = %d with %d days, want 200 and 7 days", response.StatusCode, len(body.Days))
	}
	last := body.Days[6]
	if last.Date != today || last.Link != 2 || last.QR != 3 || body.Totals.Link != 2 || body.Totals.QR != 3 {
		t.Errorf("public link stats = %+v, want today %s with 2 by link and 3 by QR code", body, today)
	}

	if _, body := stats(private, "", owner); len(body.Days) != 30 || body.Totals.QR != 1 || body.Totals.Link != 0 {
		t.Errorf("private link stats = %d days, totals %+v; want 30 days and 1 QR code click", len(body.Days), body.Totals)
	}

	// Only the owner sees them, and only for the offered periods.
	if response, _ := stats(public, "?days=7", other); response.StatusCode != http.StatusNotFound {
		t.Errorf("stats by another user status = %d, want 404", response.StatusCode)
	}
	if response, _ := stats(public, "?days=7", nil); response.StatusCode != http.StatusUnauthorized {
		t.Errorf("stats without a session status = %d, want 401", response.StatusCode)
	}
	bad := performRequestWithCookie(t, router, http.MethodGet, "/urls/"+public+"/clicks?days=365", owner)
	details, ok := problemtest.Decode(readResponseBody(t, bad))
	if bad.StatusCode != http.StatusBadRequest || !ok || details.Code != "invalid_click_period" {
		t.Errorf("stats for 365 days = %d %+v, want 400 invalid_click_period", bad.StatusCode, details)
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return location
}
