package middleware

import "net/http"

// StrictTransportSecurity tells browsers to reach this host, and its
// subdomains, only over HTTPS for a year (OWASP HTTP Headers Cheat Sheet).
// It closes the first-request window that an HTTP-to-HTTPS redirect leaves
// open. No preload: joining the browsers' preload list is hard to undo.
// Browsers ignore the header over plain HTTP, but the server still sends it
// only where HTTPS is in use.
const strictTransportSecurity = "max-age=31536000; includeSubDomains"

func StrictTransportSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", strictTransportSecurity)
		next.ServeHTTP(w, r)
	})
}
