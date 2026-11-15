package handlers

import (
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/models"
)

// qrMarker is the query the QR code adds to the short link. It only labels
// the click; the destination never changes.
const qrMarker = "qr=1"

func clickSourceOf(r *http.Request) models.ClickSource {
	if r.URL.Query().Get("qr") == "1" {
		return models.ClickSourceQR
	}
	return models.ClickSourceLink
}

// keepClickSource appends the QR marker to path when the request had it, so
// the unlock form of a private link does not lose where the visitor came
// from.
func keepClickSource(r *http.Request, path string) string {
	if clickSourceOf(r) == models.ClickSourceQR {
		return path + "?" + qrMarker
	}
	return path
}
