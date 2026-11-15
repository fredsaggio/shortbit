package middleware

import "net/http"

// NoSniff stops browsers from guessing a response's type and running, say, a
// JSON body or an error page as script or style: they must trust the declared
// Content-Type (OWASP HTTP Headers Cheat Sheet). It is safe on every response.
func NoSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
