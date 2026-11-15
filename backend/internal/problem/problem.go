// Package problem writes API errors as RFC 9457 Problem Details: a JSON body
// served as application/problem+json. detail is the Portuguese message shown
// to people; code is a stable, machine-readable name clients can branch on
// without parsing text. type stays "about:blank", so title is the HTTP status
// phrase, as the RFC prescribes for it.
package problem

import (
	"encoding/json"
	"net/http"
)

const ContentType = "application/problem+json"

type Details struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

// Write sends the error with the given status. Like http.Error, it removes
// headers meant for a successful body and stops caching.
func Write(w http.ResponseWriter, status int, code, detail string) {
	h := w.Header()
	h.Del("Content-Length")
	h.Del("Content-Encoding")
	h.Set("Content-Type", ContentType)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	// Encoding a struct of strings and an int cannot fail; a broken
	// connection has nowhere to be reported.
	_ = json.NewEncoder(w).Encode(Details{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	})
}
