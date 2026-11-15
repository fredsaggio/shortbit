//go:build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/jackc/pgx/v5/pgxpool"
)

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(t.Context(), query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestDeleteAccountIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	ownerID := createConfirmedPasswordUser(t, pool, "delete-owner@example.com", "senha12345")
	otherID := createConfirmedPasswordUser(t, pool, "delete-other@example.com", "senha12345")

	cfg := testConfig(24 * time.Hour)
	cfg.LinkAccessSession.TTL = 30 * time.Minute
	handlers, _, err := app.CompositionRoot(pool, cfg, unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()

	owner := loginAs(t, router, "delete-owner@example.com")
	secondDevice := loginAs(t, router, "delete-owner@example.com")
	other := loginAs(t, router, "delete-other@example.com")

	publicCode := createPublicURL(t, router, owner)
	private := performJSONRequestWithCookie(t, router, http.MethodPost, "/urls", `{"url":"https://example.com/p","visibility":"private","password":"senha-do-link"}`, owner)
	private.Body.Close()
	otherCode := createPublicURL(t, router, other)
	unlockCode := ""
	if err := pool.QueryRow(t.Context(), "SELECT short_code FROM urls WHERE user_id = $1 AND visibility = 'private'", ownerID).Scan(&unlockCode); err != nil {
		t.Fatalf("find private link: %v", err)
	}
	unlockLink(t, router, unlockCode, "senha-do-link")

	// A wrong password deletes nothing.
	wrong := performJSONRequestWithCookie(t, router, http.MethodDelete, "/me", `{"password":"Senha-errada1"}`, owner)
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusBadRequest || countRows(t, pool, "SELECT COUNT(*) FROM users WHERE id = $1", ownerID) != 1 {
		t.Fatalf("wrong password status = %d, want 400 and the account intact", wrong.StatusCode)
	}

	deleted := performJSONRequestWithCookie(t, router, http.MethodDelete, "/me", `{"password":"senha12345"}`, owner)
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /me status = %d, want 204", deleted.StatusCode)
	}

	for name, query := range map[string]string{
		"users":                "SELECT COUNT(*) FROM users WHERE id = $1",
		"user_sessions":        "SELECT COUNT(*) FROM user_sessions WHERE user_id = $1",
		"password_credentials": "SELECT COUNT(*) FROM password_credentials WHERE user_id = $1",
		"urls":                 "SELECT COUNT(*) FROM urls WHERE user_id = $1",
	} {
		if n := countRows(t, pool, query, ownerID); n != 0 {
			t.Errorf("%s left for the deleted account = %d, want 0", name, n)
		}
	}
	if n := countRows(t, pool, "SELECT COUNT(*) FROM link_access_sessions"); n != 0 {
		t.Errorf("link access sessions left = %d, want 0", n)
	}

	// Every device is signed out, and the links stop working.
	for name, cookie := range map[string]*http.Cookie{"this device": owner, "other device": secondDevice} {
		me := performRequestWithCookie(t, router, http.MethodGet, "/me", cookie)
		me.Body.Close()
		if me.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET /me from %s = %d, want 401", name, me.StatusCode)
		}
	}
	for _, code := range []string{publicCode, unlockCode} {
		gone := openLink(t, router, code, nil)
		gone.Body.Close()
		if gone.StatusCode != http.StatusNotFound {
			t.Errorf("link %s after account deletion = %d, want 404", code, gone.StatusCode)
		}
	}

	// Someone else's account and links are untouched.
	if countRows(t, pool, "SELECT COUNT(*) FROM users WHERE id = $1", otherID) != 1 || countRows(t, pool, "SELECT COUNT(*) FROM urls WHERE user_id = $1", otherID) != 1 {
		t.Fatal("deleting one account touched another")
	}
	stillThere := openLink(t, router, otherCode, nil)
	stillThere.Body.Close()
	if stillThere.StatusCode != http.StatusFound {
		t.Errorf("other user's link = %d, want 302", stillThere.StatusCode)
	}
}

func TestDeleteGoogleOnlyAccountIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	userID := createConfirmedPasswordUser(t, pool, "google-only@example.com", "unused")
	if _, err := pool.Exec(t.Context(), "DELETE FROM password_credentials WHERE user_id = $1", userID); err != nil {
		t.Fatalf("remove password: %v", err)
	}
	if _, err := pool.Exec(t.Context(), "INSERT INTO auth_identities (user_id, provider, provider_user_id) VALUES ($1, 'google', 'google-subject-1')", userID); err != nil {
		t.Fatalf("add Google identity: %v", err)
	}
	token, tokenHash, err := sessiontoken.Generate()
	if err != nil {
		t.Fatalf("generate session: %v", err)
	}
	if _, err := pool.Exec(t.Context(), "INSERT INTO user_sessions (token_hash, user_id, expires_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour')", tokenHash, userID); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	session := &http.Cookie{Name: "user_session", Value: token}

	handlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(handlers, pool).NewRouterHTTP()

	wrong := performJSONRequestWithCookie(t, router, http.MethodDelete, "/me", `{"email":"someone@example.com"}`, session)
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong email status = %d, want 400", wrong.StatusCode)
	}

	deleted := performJSONRequestWithCookie(t, router, http.MethodDelete, "/me", `{"email":"Google-Only@example.com"}`, session)
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /me status = %d, want 204", deleted.StatusCode)
	}
	if countRows(t, pool, "SELECT COUNT(*) FROM users WHERE id = $1", userID) != 0 ||
		countRows(t, pool, "SELECT COUNT(*) FROM auth_identities WHERE user_id = $1", userID) != 0 {
		t.Fatal("Google-only account or its identity survived")
	}
}
