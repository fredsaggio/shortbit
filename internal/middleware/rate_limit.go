package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateLimitBucket struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type RateLimiter struct {
	mu              sync.Mutex
	entries         map[string]*rateLimitBucket
	rate            float64
	burst           float64
	maxEntries      int
	staleAfter      time.Duration
	lastCleanup     time.Time
	cleanupInterval time.Duration
	now             func() time.Time
}

func NewRateLimiter(requestsPerSecond float64, burst, maxEntries int, staleAfter time.Duration) *RateLimiter {
	if requestsPerSecond <= 0 {
		panic("requests per second must be greater than zero")
	}

	if burst <= 0 {
		panic("burst must be greater than zero")
	}

	if maxEntries <= 0 {
		panic("max entries must be greater than zero")
	}

	if staleAfter <= 0 {
		panic("stale after must be greater than zero")
	}

	now := time.Now()

	return &RateLimiter{
		entries:         make(map[string]*rateLimitBucket),
		rate:            requestsPerSecond,
		burst:           float64(burst),
		maxEntries:      maxEntries,
		staleAfter:      staleAfter,
		lastCleanup:     now,
		cleanupInterval: time.Minute,
		now:             time.Now,
	}
}

func (l *RateLimiter) MiddlewareByIP(next http.Handler) http.Handler {
	return l.middlewareByKey(next, clientIP)
}

func (l *RateLimiter) MiddlewareByIPAndPathValue(pathValue string, next http.Handler) http.Handler {
	return l.middlewareByKey(next, func(r *http.Request) string {
		return clientIP(r) + "\x00" + r.PathValue(pathValue)
	})
}

func (l *RateLimiter) middlewareByKey(next http.Handler, key func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := l.Allow(key(r))
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, "muitas requisições, tente novamente mais tarde", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) Allow(key string) (bool, int) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= l.cleanupInterval {
		l.cleanup(now)
	}

	bucket, exists := l.entries[key]
	if !exists {
		if len(l.entries) >= l.maxEntries {
			l.removeOldestEntry()
		}

		l.entries[key] = &rateLimitBucket{
			tokens:     l.burst - 1,
			lastRefill: now,
			lastSeen:   now,
		}

		return true, 0
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()

	if elapsed > 0 {
		bucket.tokens = math.Min(l.burst, bucket.tokens+elapsed*l.rate)
		bucket.lastRefill = now
	}

	bucket.lastSeen = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}

	secondsUntilNextToken := (1 - bucket.tokens) / l.rate

	retryAfter := max(1, int(math.Ceil(secondsUntilNextToken)))

	return false, retryAfter
}

func (l *RateLimiter) cleanup(now time.Time) {
	for key, bucket := range l.entries {
		if now.Sub(bucket.lastSeen) >= l.staleAfter {
			delete(l.entries, key)
		}
	}

	l.lastCleanup = now
}

func (l *RateLimiter) removeOldestEntry() {
	var oldestKey string
	var oldestTime time.Time
	found := false

	for key, bucket := range l.entries {
		if !found || bucket.lastSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = bucket.lastSeen
			found = true
		}
	}

	if found {
		delete(l.entries, oldestKey)
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	if ip := net.ParseIP(r.RemoteAddr); ip != nil {
		return ip.String()
	}

	return "unknown"
}
