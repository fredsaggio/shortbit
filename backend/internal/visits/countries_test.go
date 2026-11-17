package visits_test

import (
	"os"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/visits"
)

// The database is not in the repository (make geoip downloads it), so this
// test runs only when GEOIP_DB_PATH points to it.
func TestCountryDB(t *testing.T) {
	path := os.Getenv("GEOIP_DB_PATH")
	if path == "" {
		t.Skip("GEOIP_DB_PATH not set; run make geoip and export it")
	}
	db, err := visits.OpenCountryDB(path)
	if err != nil {
		t.Fatalf("OpenCountryDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tests := map[string]string{
		"8.8.8.8":        "US",
		"::ffff:8.8.8.8": "US",
		"200.160.2.3":    "BR",
		"127.0.0.1":      "",
		"10.0.0.1":       "",
		"192.168.1.1":    "",
		"::1":            "",
		"unknown":        "",
		"":               "",
	}
	for ip, want := range tests {
		if got := db.Country(ip); got != want {
			t.Errorf("Country(%q) = %q, want %q", ip, got, want)
		}
	}

	// The Lite database moves IPv6 blocks between countries over time; it
	// only has to find one.
	if got := db.Country("2001:4860:4860::8888"); got == "" {
		t.Error("Country() of a global IPv6 address = \"\", want a country")
	}
}

func TestOpenCountryDBFailsOnMissingFile(t *testing.T) {
	if _, err := visits.OpenCountryDB(t.TempDir() + "/missing.mmdb"); err == nil {
		t.Error("OpenCountryDB() error = nil for a missing file")
	}
}
