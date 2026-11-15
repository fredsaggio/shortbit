// Package problemtest helps tests read RFC 9457 error bodies.
package problemtest

import (
	"bytes"
	"encoding/json"

	"github.com/fredsaggio/url-shortener/internal/problem"
)

// Text returns "detail\n" when body is a problem document and body unchanged
// otherwise, so tests written against plain-text errors keep asserting the
// message people see.
func Text(body string) string {
	details, ok := Decode(body)
	if !ok {
		return body
	}
	return details.Detail + "\n"
}

// Decode parses body as a problem document. It accepts only the exact
// fields the API writes.
func Decode(body string) (problem.Details, bool) {
	var details problem.Details
	decoder := json.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&details); err != nil || details.Type == "" || details.Code == "" || details.Status == 0 {
		return problem.Details{}, false
	}
	return details, true
}
