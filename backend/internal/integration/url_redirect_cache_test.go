//go:build integration

package integration_test

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/cache"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPublicURLRedirectCacheIntegration(t *testing.T) {
	const email = "url-cache-owner@example.com"
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, email, "senha12345")

	redisContainer, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, redisContainer)
	if err != nil {
		t.Fatalf("start Redis container: %v", err)
	}
	host, err := redisContainer.Host(t.Context())
	if err != nil {
		t.Fatalf("get Redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(t.Context(), "6379/tcp")
	if err != nil {
		t.Fatalf("get Redis port: %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(host, port.Port()), ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { _ = redisClient.Close() })

	cfg := testConfig(24 * time.Hour)
	cfg.Redis.Timeout = 2 * time.Second
	cfg.HotURL.Threshold = 3
	cfg.HotURL.Window = time.Minute
	cfg.HotURL.CacheTTL = time.Minute

	heatTracker := cache.NewRedisHeatTracker(redisClient, cfg.Redis.Timeout, cfg.HotURL.Window)
	destinationCache := cache.NewRedisDestinationCache(redisClient, cfg.Redis.Timeout, cfg.HotURL.CacheTTL)
	applicationHandlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, heatTracker, destinationCache, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	login := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"senha12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusNoContent {
		t.Fatalf("login status = %d, want 204; body = %q", login.StatusCode, readResponseBody(t, login))
	}
	code := createPublicURL(t, router, requireCookie(t, login.Cookies(), "user_session"))
	cacheKey := "url:dest:" + code

	redirect := func() string {
		t.Helper()
		response := performRequestWithCookie(t, router, http.MethodGet, "/"+code, nil)
		defer response.Body.Close()
		if response.StatusCode != http.StatusFound {
			t.Fatalf("redirect status = %d, want 302", response.StatusCode)
		}
		return response.Header.Get("Location")
	}

	cached := func() bool {
		t.Helper()
		exists, err := redisClient.Exists(t.Context(), cacheKey).Result()
		if err != nil {
			t.Fatalf("check cache key: %v", err)
		}
		return exists == 1
	}

	t.Run("keeps a cold link out of the cache", func(t *testing.T) {
		for range cfg.HotURL.Threshold - 1 {
			redirect()
		}
		if cached() {
			t.Error("destination cached before reaching the threshold")
		}
	})

	t.Run("caches the destination when the link gets hot", func(t *testing.T) {
		redirect()

		if !cached() {
			t.Fatal("destination not cached after reaching the threshold")
		}
		ttl, err := redisClient.PTTL(t.Context(), cacheKey).Result()
		if err != nil || ttl <= 0 || ttl > cfg.HotURL.CacheTTL {
			t.Errorf("cache TTL = (%v, %v), want between 0 and %v", ttl, err, cfg.HotURL.CacheTTL)
		}
	})

	t.Run("serves a cache hit without reading the destination from PostgreSQL", func(t *testing.T) {
		if _, err := pool.Exec(t.Context(), `UPDATE urls SET original_url = 'https://example.com/changed' WHERE short_code = $1`, code); err != nil {
			t.Fatalf("change destination in PostgreSQL: %v", err)
		}
		before := clickCountByCode(t, pool, code)

		if got := redirect(); got != "https://example.com/queued" {
			t.Errorf("Location = %q, want the cached destination", got)
		}
		if got := clickCountByCode(t, pool, code); got != before+1 {
			t.Errorf("click count = %d, want %d: a cache hit still counts the click", got, before+1)
		}
	})

	t.Run("reads PostgreSQL again after the entry expires", func(t *testing.T) {
		if err := redisClient.Del(t.Context(), cacheKey).Err(); err != nil {
			t.Fatalf("expire cache key: %v", err)
		}

		if got := redirect(); got != "https://example.com/changed" {
			t.Errorf("Location = %q, want the PostgreSQL destination", got)
		}
	})

	t.Run("keeps redirecting when Redis is down", func(t *testing.T) {
		if err := redisContainer.Stop(t.Context(), nil); err != nil {
			t.Fatalf("stop Redis: %v", err)
		}
		before := clickCountByCode(t, pool, code)

		if got := redirect(); got != "https://example.com/changed" {
			t.Errorf("Location = %q, want the PostgreSQL destination", got)
		}
		if got := clickCountByCode(t, pool, code); got != before+1 {
			t.Errorf("click count = %d, want %d", got, before+1)
		}
	})
}
