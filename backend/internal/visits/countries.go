package visits

import (
	"fmt"
	"net/netip"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/oschwald/maxminddb-golang/v2"
)

// CountryDB finds the country of an IP in a local MMDB file with a
// country.iso_code field, such as DB-IP's IP to Country Lite (ADR 0007). The
// lookup runs in memory: no IP leaves the process.
type CountryDB struct {
	reader *maxminddb.Reader
}

func OpenCountryDB(path string) (*CountryDB, error) {
	reader, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open country database: %w", err)
	}
	return &CountryDB{reader: reader}, nil
}

// Country returns the ISO 3166-1 alpha-2 code of the IP, or "" when the IP
// does not parse, is private, or the database does not know it.
func (db *CountryDB) Country(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return ""
	}

	var code string
	if err := db.reader.Lookup(addr).DecodePath(&code, "country", "iso_code"); err != nil {
		return ""
	}
	if !models.ValidCountry(code) {
		return ""
	}
	return code
}

func (db *CountryDB) Close() error {
	return db.reader.Close()
}
