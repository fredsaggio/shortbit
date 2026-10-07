//go:build integration

package integration_test

import (
	"encoding/json"
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

func TestPublicURLRedirectIntegration(t *testing.T) {
	const email = "url-redirect-owner@example.com"
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
	if err := redisClient.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	cfg := testConfig(24 * time.Hour)
	cfg.Redis.Timeout = 2 * time.Second
	heatTracker := cache.NewRedisHeatTracker(redisClient, cfg.Redis.Timeout, time.Minute)
	applicationHandlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, heatTracker)
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	login := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"url-redirect-owner@example.com","password":"senha12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusNoContent {
		t.Fatalf("login status = %d, want 204; body = %q", login.StatusCode, readResponseBody(t, login))
	}
	sessionCookie := requireCookie(t, login.Cookies(), "user_session")

	createURL := func(body string) string {
		t.Helper()
		response := performJSONRequestWithCookie(t, router, http.MethodPost, "/urls", body, sessionCookie)
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create URL status = %d, want 201; body = %q", response.StatusCode, readResponseBody(t, response))
		}
		var result struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.ShortCode == "" {
			t.Fatalf("decode created URL = %+v, error = %v", result, err)
		}
		return result.ShortCode
	}
	publicCode := createURL(`{"url":"https://example.com/public","visibility":"public"}`)
	privateCode := createURL(`{"url":"https://example.com/private","visibility":"private","password":"senha-do-link"}`)
	publicHeatKey := "url:heat:" + publicCode

	clickCount := func(code string) int64 {
		t.Helper()
		var count int64
		if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE short_code = $1", code).Scan(&count); err != nil {
			t.Fatalf("read click count for %q: %v", code, err)
		}
		return count
	}

	for visit := int64(1); visit <= 2; visit++ {
		response := performRequestWithCookie(t, router, http.MethodGet, "/"+publicCode, nil)
		if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "https://example.com/public" || response.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("public redirect %d = (status %d, Location %q, Cache-Control %q), want (302, destination, no-store)",
				visit, response.StatusCode, response.Header.Get("Location"), response.Header.Get("Cache-Control"))
		}
		response.Body.Close()
		if got := clickCount(publicCode); got != visit {
			t.Errorf("public click count after visit %d = %d, want %d", visit, got, visit)
		}
		if got, err := redisClient.Get(t.Context(), publicHeatKey).Int64(); err != nil || got != visit {
			t.Errorf("public Redis heat after visit %d = (%d, %v), want (%d, nil)", visit, got, err, visit)
		}
	}
	if ttl, err := redisClient.PTTL(t.Context(), publicHeatKey).Result(); err != nil || ttl <= 0 || ttl > time.Minute {
		t.Errorf("public Redis heat TTL = (%v, %v), want between 0 and 1m", ttl, err)
	}

	privateResponse := performRequestWithCookie(t, router, http.MethodGet, "/"+privateCode, nil)
	defer privateResponse.Body.Close()
	missingResponse := performRequestWithCookie(t, router, http.MethodGet, "/NoSuchCode123", nil)
	defer missingResponse.Body.Close()
	if privateResponse.StatusCode != http.StatusOK || missingResponse.StatusCode != http.StatusNotFound {
		t.Errorf("private/missing status = (%d, %d), want (200, 404)", privateResponse.StatusCode, missingResponse.StatusCode)
	}
	if privateResponse.Header.Get("Location") != "" || missingResponse.Header.Get("Location") != "" {
		t.Error("private or missing URL returned a redirect location")
	}
	if got := clickCount(privateCode); got != 0 {
		t.Errorf("private click count = %d, want 0", got)
	}
	if exists, err := redisClient.Exists(t.Context(), "url:heat:"+privateCode).Result(); err != nil || exists != 0 {
		t.Errorf("private Redis heat key exists = (%d, %v), want (0, nil)", exists, err)
	}
	if got := clickCount(publicCode); got != 2 {
		t.Errorf("public click count after failed redirects = %d, want 2", got)
	}
}
