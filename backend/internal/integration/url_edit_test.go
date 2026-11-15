//go:build integration

package integration_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func loginAs(t *testing.T, router http.Handler, email string) *http.Cookie {
	t.Helper()
	response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"senha12345"}`)
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("login %s status = %d, want 204; body = %q", email, response.StatusCode, readResponseBody(t, response))
	}
	return requireCookie(t, response.Cookies(), "user_session")
}

func patchURL(t *testing.T, router http.Handler, code, body string, cookie *http.Cookie) (int, map[string]any) {
	t.Helper()
	response := performJSONRequestWithCookie(t, router, http.MethodPatch, "/urls/"+code, body, cookie)
	defer response.Body.Close()
	var decoded map[string]any
	if response.StatusCode == http.StatusOK {
		if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
			t.Fatalf("decode PATCH response: %v", err)
		}
	}
	return response.StatusCode, decoded
}

// openLink opens a short link as a visitor, with an optional unlock cookie.
func openLink(t *testing.T, router http.Handler, code string, cookie *http.Cookie) *http.Response {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+code, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder.Result()
}

func unlockLink(t *testing.T, router http.Handler, code, password string) *http.Cookie {
	t.Helper()
	form := url.Values{"password": {password}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+code+"/access", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("unlock status = %d, want 303", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.MaxAge >= 0 && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatal("unlock did not set an access cookie")
	return nil
}

func TestEditAndDeleteURLIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, "edit-owner@example.com", "senha12345")
	createConfirmedPasswordUser(t, pool, "edit-other@example.com", "senha12345")

	cfg := testConfig(24 * time.Hour)
	cfg.LinkAccessSession.TTL = 30 * time.Minute
	handlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()
	owner := loginAs(t, router, "edit-owner@example.com")
	other := loginAs(t, router, "edit-other@example.com")
	code := createPublicURL(t, router, owner)

	// Another user can neither edit nor delete it, and learns nothing.
	if status, _ := patchURL(t, router, code, `{"url":"https://example.com/hijack"}`, other); status != http.StatusNotFound {
		t.Fatalf("PATCH by another user status = %d, want 404", status)
	}
	otherDelete := performRequestWithCookie(t, router, http.MethodDelete, "/urls/"+code, other)
	otherDelete.Body.Close()
	if otherDelete.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE by another user status = %d, want 404", otherDelete.StatusCode)
	}

	// New destination.
	status, body := patchURL(t, router, code, `{"url":"https://example.com/new"}`, owner)
	if status != http.StatusOK || body["original_url"] != "https://example.com/new" || body["visibility"] != "public" {
		t.Fatalf("PATCH url = %d %v, want the new destination", status, body)
	}
	redirect := openLink(t, router, code, nil)
	redirect.Body.Close()
	if redirect.StatusCode != http.StatusFound || redirect.Header.Get("Location") != "https://example.com/new" {
		t.Fatalf("redirect = %d %q, want 302 to the new destination", redirect.StatusCode, redirect.Header.Get("Location"))
	}

	// Private needs a password; with one, visitors get the password page.
	if status, _ := patchURL(t, router, code, `{"visibility":"private"}`, owner); status != http.StatusBadRequest {
		t.Fatalf("PATCH to private without password status = %d, want 400", status)
	}
	if status, body := patchURL(t, router, code, `{"visibility":"private","password":"senha-antiga-link"}`, owner); status != http.StatusOK || body["visibility"] != "private" {
		t.Fatalf("PATCH to private = %d %v", status, body)
	}
	locked := openLink(t, router, code, nil)
	locked.Body.Close()
	if locked.StatusCode != http.StatusOK || !strings.HasPrefix(locked.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("private link without access = %d %q, want the password page", locked.StatusCode, locked.Header.Get("Content-Type"))
	}

	oldAccess := unlockLink(t, router, code, "senha-antiga-link")
	unlocked := openLink(t, router, code, oldAccess)
	unlocked.Body.Close()
	if unlocked.StatusCode != http.StatusFound {
		t.Fatalf("unlocked link status = %d, want 302", unlocked.StatusCode)
	}

	// Editing only the destination keeps the password and the access.
	if status, _ := patchURL(t, router, code, `{"url":"https://example.com/moved"}`, owner); status != http.StatusOK {
		t.Fatalf("PATCH destination of private link status = %d", status)
	}
	stillUnlocked := openLink(t, router, code, oldAccess)
	stillUnlocked.Body.Close()
	if stillUnlocked.StatusCode != http.StatusFound || stillUnlocked.Header.Get("Location") != "https://example.com/moved" {
		t.Fatalf("access after destination change = %d %q, want 302 to the moved destination", stillUnlocked.StatusCode, stillUnlocked.Header.Get("Location"))
	}

	// A new password ends access granted with the old one.
	if status, _ := patchURL(t, router, code, `{"password":"senha-nova-do-link"}`, owner); status != http.StatusOK {
		t.Fatalf("PATCH password status = %d", status)
	}
	revoked := openLink(t, router, code, oldAccess)
	revoked.Body.Close()
	if revoked.StatusCode != http.StatusOK {
		t.Fatalf("old access after password change = %d, want the password page", revoked.StatusCode)
	}
	var sessions int
	if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM link_access_sessions s JOIN urls u ON u.id = s.link_id WHERE u.short_code = $1", code).Scan(&sessions); err != nil {
		t.Fatalf("count access sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("access sessions after password change = %d, want 0", sessions)
	}

	// Back to public clears the password.
	if status, body := patchURL(t, router, code, `{"visibility":"public"}`, owner); status != http.StatusOK || body["visibility"] != "public" {
		t.Fatalf("PATCH to public = %d %v", status, body)
	}
	var hasPassword bool
	if err := pool.QueryRow(t.Context(), "SELECT password_hash IS NOT NULL FROM urls WHERE short_code = $1", code).Scan(&hasPassword); err != nil || hasPassword {
		t.Fatalf("password hash after going public = %v, error = %v; want none", hasPassword, err)
	}

	// Delete: gone for the owner and for visitors.
	deleted := performRequestWithCookie(t, router, http.MethodDelete, "/urls/"+code, owner)
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", deleted.StatusCode)
	}
	gone := openLink(t, router, code, nil)
	gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted link status = %d, want 404", gone.StatusCode)
	}
	again := performRequestWithCookie(t, router, http.MethodDelete, "/urls/"+code, owner)
	again.Body.Close()
	if again.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE status = %d, want 404", again.StatusCode)
	}
}

func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	container, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start Redis container: %v", err)
	}
	host, err := container.Host(t.Context())
	if err != nil {
		t.Fatalf("get Redis host: %v", err)
	}
	port, err := container.MappedPort(t.Context(), "6379/tcp")
	if err != nil {
		t.Fatalf("get Redis port: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(host, port.Port()), ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// A cached destination must not outlive an edit: a public link turned private
// would otherwise keep redirecting without a password until the entry expired.
func TestEditDropsCachedDestinationIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, "edit-cache-owner@example.com", "senha12345")
	redisClient := startRedis(t)

	cfg := testConfig(24 * time.Hour)
	cfg.Redis.Timeout = 2 * time.Second
	cfg.HotURL.Threshold = 1
	cfg.HotURL.Window = time.Minute
	cfg.HotURL.CacheTTL = time.Hour

	heat := cache.NewRedisHeatTracker(redisClient, cfg.Redis.Timeout, cfg.HotURL.Window)
	destinations := cache.NewRedisDestinationCache(redisClient, cfg.Redis.Timeout, cfg.HotURL.CacheTTL)
	handlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, heat, destinations, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()
	owner := loginAs(t, router, "edit-cache-owner@example.com")
	code := createPublicURL(t, router, owner)
	key := "url:dest:" + code

	warm := openLink(t, router, code, nil)
	warm.Body.Close()
	if exists, _ := redisClient.Exists(t.Context(), key).Result(); exists != 1 {
		t.Fatalf("destination not cached after a hot redirect")
	}

	if status, _ := patchURL(t, router, code, `{"visibility":"private","password":"senha-do-link"}`, owner); status != http.StatusOK {
		t.Fatalf("PATCH to private status = %d", status)
	}
	if exists, _ := redisClient.Exists(t.Context(), key).Result(); exists != 0 {
		t.Fatal("cached destination survived the switch to private")
	}
	locked := openLink(t, router, code, nil)
	locked.Body.Close()
	if locked.StatusCode != http.StatusOK {
		t.Fatalf("private link status = %d, want the password page, not a cached redirect", locked.StatusCode)
	}

	if status, _ := patchURL(t, router, code, `{"visibility":"public"}`, owner); status != http.StatusOK {
		t.Fatalf("PATCH to public status = %d", status)
	}
	rewarm := openLink(t, router, code, nil)
	rewarm.Body.Close()
	if exists, _ := redisClient.Exists(t.Context(), key).Result(); exists != 1 {
		t.Fatal("destination not cached again after going public")
	}

	deleted := performRequestWithCookie(t, router, http.MethodDelete, "/urls/"+code, owner)
	deleted.Body.Close()
	if exists, _ := redisClient.Exists(t.Context(), key).Result(); exists != 0 {
		t.Fatal("cached destination survived the delete")
	}
	gone := openLink(t, router, code, nil)
	gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted link status = %d, want 404, not a cached redirect", gone.StatusCode)
	}
}
