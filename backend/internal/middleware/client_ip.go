package middleware

import (
	"context"
	"net"
	"net/http"
)

type clientIPContextKey struct{}

// ClientIP resolves the client address once per request, for the rate
// limiters. With an empty header it is the TCP peer (RemoteAddr), which is
// right when clients connect directly.
//
// Behind a proxy every request comes from the proxy, so all clients would
// share one rate limit bucket. Name a header that the proxy always overwrites
// with the client address (on Render, Cloudflare's True-Client-IP); its value
// is used when it parses as an IP, and RemoteAddr otherwise. A header the
// client can set unchecked, such as X-Forwarded-For on Render, would let anyone
// pick their own bucket, so config refuses it.
func ClientIP(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if header == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := net.ParseIP(r.Header.Get(header)); ip != nil {
				r = r.WithContext(context.WithValue(r.Context(), clientIPContextKey{}, ip.String()))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPContextKey{}).(string); ok {
		return ip
	}

	return peerIP(r)
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	if ip := net.ParseIP(r.RemoteAddr); ip != nil {
		return ip.String()
	}

	return "unknown"
}
