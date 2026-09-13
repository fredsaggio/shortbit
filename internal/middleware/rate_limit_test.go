package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIPRateLimiterAllowsBurstAndRefillsTokens(t *testing.T) {
	limiter, now := newTestIPRateLimiter(2, 2, 10, time.Minute)

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		allowed, retryAfter := limiter.allow("192.0.2.1")
		if !allowed {
			t.Fatalf("request %d allowed = false, want true; Retry-After = %d", requestNumber, retryAfter)
		}
	}

	allowed, retryAfter := limiter.allow("192.0.2.1")
	if allowed {
		t.Fatal("request after burst allowed = true, want false")
	}

	if retryAfter != 1 {
		t.Errorf("Retry-After = %d, want 1", retryAfter)
	}

	*now = now.Add(500 * time.Millisecond)
	allowed, retryAfter = limiter.allow("192.0.2.1")
	if !allowed {
		t.Fatalf("request after token refill allowed = false, want true; Retry-After = %d", retryAfter)
	}
}

func TestIPRateLimiterMiddlewareReturnsTooManyRequests(t *testing.T) {
	limiter, now := newTestIPRateLimiter(0.5, 1, 10, time.Minute)
	nextCalls := 0

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	})

	handler := limiter.Middleware(next)

	firstRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	firstRequest.RemoteAddr = "192.0.2.1:1234"
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)

	if firstResponse.Code != http.StatusNoContent {
		t.Fatalf("first status code = %d, want %d", firstResponse.Code, http.StatusNoContent)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secondRequest.RemoteAddr = "192.0.2.1:5678"
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)

	if secondResponse.Code != http.StatusTooManyRequests {
		t.Errorf("second status code = %d, want %d", secondResponse.Code, http.StatusTooManyRequests)
	}

	if got := secondResponse.Header().Get("Retry-After"); got != "2" {
		t.Errorf("Retry-After = %q, want %q", got, "2")
	}

	if got := secondResponse.Body.String(); got != "muitas requisições, tente novamente mais tarde\n" {
		t.Errorf("response body = %q, want rate limit message", got)
	}

	if nextCalls != 1 {
		t.Errorf("next handler calls = %d, want 1", nextCalls)
	}

	*now = now.Add(2 * time.Second)
	thirdRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	thirdRequest.RemoteAddr = "192.0.2.1:9999"
	thirdResponse := httptest.NewRecorder()
	handler.ServeHTTP(thirdResponse, thirdRequest)

	if thirdResponse.Code != http.StatusNoContent {
		t.Errorf("status after Retry-After = %d, want %d", thirdResponse.Code, http.StatusNoContent)
	}

	if nextCalls != 2 {
		t.Errorf("next handler calls after refill = %d, want 2", nextCalls)
	}
}

func TestIPRateLimiterKeepsIndependentBucketsPerIP(t *testing.T) {
	limiter, _ := newTestIPRateLimiter(1, 1, 10, time.Minute)

	if allowed, _ := limiter.allow("192.0.2.1"); !allowed {
		t.Fatal("first IP initial request was rejected")
	}

	if allowed, _ := limiter.allow("192.0.2.1"); allowed {
		t.Fatal("first IP request after burst was allowed")
	}

	if allowed, _ := limiter.allow("192.0.2.2"); !allowed {
		t.Fatal("second IP initial request was rejected")
	}
}

func TestIPRateLimiterRemovesStaleClients(t *testing.T) {
	limiter, now := newTestIPRateLimiter(1, 1, 10, 2*time.Minute)
	limiter.allow("192.0.2.1")

	*now = now.Add(2 * time.Minute)
	limiter.allow("192.0.2.2")

	if _, exists := limiter.clients["192.0.2.1"]; exists {
		t.Error("stale client was not removed")
	}

	if _, exists := limiter.clients["192.0.2.2"]; !exists {
		t.Error("current client was not stored")
	}

	if len(limiter.clients) != 1 {
		t.Errorf("tracked clients = %d, want 1", len(limiter.clients))
	}
}

func TestIPRateLimiterRemovesOldestClientAtCapacity(t *testing.T) {
	limiter, now := newTestIPRateLimiter(1, 1, 2, time.Hour)
	limiter.allow("192.0.2.1")

	*now = now.Add(time.Second)
	limiter.allow("192.0.2.2")

	*now = now.Add(time.Second)
	limiter.allow("192.0.2.3")

	if _, exists := limiter.clients["192.0.2.1"]; exists {
		t.Error("oldest client was not removed")
	}

	if _, exists := limiter.clients["192.0.2.2"]; !exists {
		t.Error("second client was unexpectedly removed")
	}

	if _, exists := limiter.clients["192.0.2.3"]; !exists {
		t.Error("new client was not stored")
	}

	if len(limiter.clients) != 2 {
		t.Errorf("tracked clients = %d, want 2", len(limiter.clients))
	}
}

func TestIPRateLimiterHandlesConcurrentRequests(t *testing.T) {
	const burst = 10

	limiter, _ := newTestIPRateLimiter(1, burst, 10, time.Minute)
	var allowedRequests atomic.Int64
	var waitGroup sync.WaitGroup

	for range 100 {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()

			if allowed, _ := limiter.allow("192.0.2.1"); allowed {
				allowedRequests.Add(1)
			}
		}()
	}

	waitGroup.Wait()

	if got := allowedRequests.Load(); got != burst {
		t.Errorf("allowed concurrent requests = %d, want %d", got, burst)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "IPv4 with port", remoteAddr: "192.0.2.1:8080", want: "192.0.2.1"},
		{name: "IPv6 with port", remoteAddr: "[2001:db8::1]:8080", want: "2001:db8::1"},
		{name: "IPv4 without port", remoteAddr: "192.0.2.1", want: "192.0.2.1"},
		{name: "unknown address", remoteAddr: "not-an-address", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
			request.RemoteAddr = tt.remoteAddr

			if got := clientIP(request); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func newTestIPRateLimiter(requestsPerSecond float64, burst, maxClients int, staleAfter time.Duration) (*IPRateLimiter, *time.Time) {
	limiter := NewIPRateLimiter(requestsPerSecond, burst, maxClients, staleAfter)
	now := limiter.lastCleanup
	limiter.now = func() time.Time { return now }

	return limiter, &now
}
