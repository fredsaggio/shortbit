package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIPFromHeader(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "uses the TCP peer without a configured header",
			remoteAddr: "203.0.113.7:51234",
			headers:    map[string]string{"True-Client-IP": "198.51.100.1"},
			want:       "203.0.113.7",
		},
		{
			name:       "uses the configured header behind a proxy",
			header:     "True-Client-Ip",
			remoteAddr: "10.0.0.2:443",
			headers:    map[string]string{"True-Client-IP": "198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "normalizes an IPv6 address from the header",
			header:     "True-Client-Ip",
			remoteAddr: "10.0.0.2:443",
			headers:    map[string]string{"True-Client-IP": "2001:DB8::1"},
			want:       "2001:db8::1",
		},
		{
			name:       "falls back to the peer when the header is missing",
			header:     "True-Client-Ip",
			remoteAddr: "10.0.0.2:443",
			want:       "10.0.0.2",
		},
		{
			name:       "falls back to the peer when the header is not an IP",
			header:     "True-Client-Ip",
			remoteAddr: "10.0.0.2:443",
			headers:    map[string]string{"True-Client-IP": "198.51.100.1, 10.0.0.9"},
			want:       "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			handler := ClientIP(tt.header)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = clientIP(r)
			}))

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				request.Header.Set(key, value)
			}
			handler.ServeHTTP(httptest.NewRecorder(), request)

			if got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimiterUsesResolvedClientIP(t *testing.T) {
	// Frozen clock: no tokens refill between requests.
	limiter, _ := newTestRateLimiter(1, 1, 100, time.Minute)
	handler := ClientIP("True-Client-Ip")(limiter.MiddlewareByIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	send := func(client string) int {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "10.0.0.2:443"
		request.Header.Set("True-Client-IP", client)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}

	if code := send("198.51.100.1"); code != http.StatusNoContent {
		t.Fatalf("first client first request status = %d, want %d", code, http.StatusNoContent)
	}
	if code := send("198.51.100.2"); code != http.StatusNoContent {
		t.Fatalf("second client behind the same proxy status = %d, want %d", code, http.StatusNoContent)
	}
	if code := send("198.51.100.1"); code != http.StatusTooManyRequests {
		t.Fatalf("first client second request status = %d, want %d", code, http.StatusTooManyRequests)
	}
}
