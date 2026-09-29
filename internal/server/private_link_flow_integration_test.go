//go:build integration

package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/fredsaggio/url-shortener/internal/shortcodes"
	"github.com/jackc/pgx/v5"
)

func TestPrivateLinkAccessFlowIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	cfg, err := config.Load(func(key string) string {
		return map[string]string{
			"DATABASE_URL":         "postgres://integration-test/unused",
			"GOOGLE_CLIENT_ID":     "test-client-id",
			"GOOGLE_CLIENT_SECRET": "test-client-secret",
			"GOOGLE_REDIRECT_URL":  "http://localhost:8080/auth/google/callback",
			"EMAIL_FROM":           "noreply@example.com",
			"RESEND_API_KEY":       "test-resend-key",
			"COOKIE_SECURE":        "false",
		}[key]
	})
	if err != nil {
		t.Fatalf("load test config: %v", err)
	}
	if cfg.LinkAccessSession.TTL != 30*time.Minute {
		t.Fatalf("link access session TTL = %v, want 30m", cfg.LinkAccessSession.TTL)
	}

	handlers, _, err := app.CompositionRoot(pool, cfg, nil)
	if err != nil {
		t.Fatalf("build application: %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()
	users := repositories.NewUserRepository(pool)
	urls := repositories.NewURLRepository(pool)
	owner, err := users.CreateWithPassword(t.Context(), "private-flow-owner@example.com", "$argon2id$integration-test-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	id, err := urls.ReserveID(t.Context())
	if err != nil {
		t.Fatalf("reserve URL ID: %v", err)
	}
	generator, err := shortcodes.NewGenerator()
	if err != nil {
		t.Fatalf("create shortcode generator: %v", err)
	}
	code, err := generator.Generate(id)
	if err != nil {
		t.Fatalf("generate shortcode: %v", err)
	}
	passwordHash, err := (argon2.Argon2id{}).Hash("correct-link-password")
	if err != nil {
		t.Fatalf("hash link password: %v", err)
	}
	const destination = "https://example.com/private-destination"
	_, err = urls.Create(t.Context(), models.URL{
		ID: id, ShortCode: code, UserID: owner.ID, OriginalURL: destination,
		Visibility: models.VisibilityPrivate, PasswordHash: &passwordHash,
	})
	if err != nil {
		t.Fatalf("create private URL: %v", err)
	}

	request := func(method, path, form string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(form))
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	clickCount := func() int64 {
		t.Helper()
		var count int64
		if err := pool.QueryRow(t.Context(), "SELECT click_count FROM urls WHERE id = @id", pgx.StrictNamedArgs{"id": id}).Scan(&count); err != nil {
			t.Fatalf("query click count: %v", err)
		}
		return count
	}

	firstGET := request(http.MethodGet, "/"+code, "", nil)
	if firstGET.Code != http.StatusOK || !strings.Contains(firstGET.Body.String(), `action="/`+code+`/access"`) || clickCount() != 0 {
		t.Fatalf("first GET = status %d, count %d; want password form and no click", firstGET.Code, clickCount())
	}

	wrongPassword := request(http.MethodPost, "/"+code+"/access", url.Values{"password": {"wrong-password"}}.Encode(), nil)
	if wrongPassword.Code != http.StatusUnauthorized || !strings.Contains(wrongPassword.Body.String(), "Senha incorreta") || clickCount() != 0 {
		t.Fatalf("wrong password = status %d, count %d; want 401 and no click", wrongPassword.Code, clickCount())
	}
	if len(wrongPassword.Result().Cookies()) != 0 {
		t.Fatal("wrong password created an access cookie")
	}

	correctPassword := request(http.MethodPost, "/"+code+"/access", url.Values{"password": {"correct-link-password"}}.Encode(), nil)
	if correctPassword.Code != http.StatusSeeOther || correctPassword.Header().Get("Location") != "/"+code || clickCount() != 0 {
		t.Fatalf("correct password = status %d, location %q, count %d; want 303 to shortcode and no click yet",
			correctPassword.Code, correctPassword.Header().Get("Location"), clickCount())
	}
	cookies := correctPassword.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "link_access_session" || cookies[0].Path != "/"+code || !cookies[0].HttpOnly {
		t.Fatalf("access cookies = %v, want one scoped HttpOnly cookie", cookies)
	}
	accessCookie := cookies[0]
	if remaining := time.Until(accessCookie.Expires); remaining < 29*time.Minute || remaining > 31*time.Minute {
		t.Errorf("cookie expires in %v, want about 30m", remaining)
	}

	withoutCookie := request(http.MethodGet, "/"+code, "", nil)
	if withoutCookie.Code != http.StatusOK || clickCount() != 0 {
		t.Fatalf("GET without cookie = status %d, count %d; want password form and no click", withoutCookie.Code, clickCount())
	}

	withCookie := request(http.MethodGet, "/"+code, "", accessCookie)
	if withCookie.Code != http.StatusFound || withCookie.Header().Get("Location") != destination || clickCount() != 1 {
		t.Fatalf("GET with cookie = status %d, location %q, count %d; want 302 to destination and one click",
			withCookie.Code, withCookie.Header().Get("Location"), clickCount())
	}

	_, err = pool.Exec(t.Context(), `
		UPDATE link_access_sessions
		SET created_at = @createdAt, expires_at = @expiresAt
		WHERE token_hash = @tokenHash
	`, pgx.StrictNamedArgs{
		"createdAt": time.Now().Add(-2 * time.Hour),
		"expiresAt": time.Now().Add(-time.Hour),
		"tokenHash": sessiontoken.Hash(accessCookie.Value),
	})
	if err != nil {
		t.Fatalf("expire access session: %v", err)
	}
	expiredCookie := request(http.MethodGet, "/"+code, "", accessCookie)
	if expiredCookie.Code != http.StatusOK || !strings.Contains(expiredCookie.Body.String(), `action="/`+code+`/access"`) || clickCount() != 1 {
		t.Fatalf("GET with expired session = status %d, count %d; want password form and unchanged count", expiredCookie.Code, clickCount())
	}
}
