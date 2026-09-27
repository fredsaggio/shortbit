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
)

func TestLogoutIntegration(t *testing.T) {
	const sessionTTL = 24 * time.Hour

	pool := dbtest.Open(t)
	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(sessionTTL), unusedGoogleOIDCClient{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	createConfirmedPasswordUser(t, pool, "logout-integration@example.com", "senha12345")

	loginResponse := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"logout-integration@example.com","password":"senha12345"}`)
	defer loginResponse.Body.Close()

	if loginResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("login status code = %d, want %d; body = %q", loginResponse.StatusCode, http.StatusNoContent, readResponseBody(t, loginResponse))
	}

	sessionCookie := requireCookie(t, loginResponse.Cookies(), "user_session")
	tokenHash := sessiontoken.Hash(sessionCookie.Value)

	if !sessionHashExists(t, pool, tokenHash) {
		t.Fatal("session hash was not stored after login")
	}

	meResponse := performRequestWithCookie(t, router, http.MethodGet, "/me", sessionCookie)
	defer meResponse.Body.Close()

	if meResponse.StatusCode != http.StatusOK {
		t.Fatalf("/me before logout status code = %d, want %d; body = %q", meResponse.StatusCode, http.StatusOK, readResponseBody(t, meResponse))
	}

	logoutResponse := performRequestWithCookie(t, router, http.MethodDelete, "/sessions/current", sessionCookie)
	defer logoutResponse.Body.Close()

	if logoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status code = %d, want %d; body = %q", logoutResponse.StatusCode, http.StatusNoContent, readResponseBody(t, logoutResponse))
	}

	if !sessionCookieWasRemoved(t, logoutResponse.Cookies()) {
		t.Error("logout response did not remove the session cookie")
	}

	if sessionHashExists(t, pool, tokenHash) {
		t.Error("session hash still exists after logout")
	}

	meAfterLogoutResponse := performRequestWithCookie(t, router, http.MethodGet, "/me", sessionCookie)
	defer meAfterLogoutResponse.Body.Close()

	if meAfterLogoutResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/me after logout status code = %d, want %d; body = %q", meAfterLogoutResponse.StatusCode, http.StatusUnauthorized, readResponseBody(t, meAfterLogoutResponse))
	}

	repeatedLogoutResponse := performRequestWithCookie(t, router, http.MethodDelete, "/sessions/current", sessionCookie)
	defer repeatedLogoutResponse.Body.Close()

	if repeatedLogoutResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("repeated logout status code = %d, want %d; body = %q", repeatedLogoutResponse.StatusCode, http.StatusNoContent, readResponseBody(t, repeatedLogoutResponse))
	}

	logoutWithoutCookieResponse := performRequestWithCookie(t, router, http.MethodDelete, "/sessions/current", nil)
	defer logoutWithoutCookieResponse.Body.Close()

	if logoutWithoutCookieResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("logout without cookie status code = %d, want %d; body = %q", logoutWithoutCookieResponse.StatusCode, http.StatusNoContent, readResponseBody(t, logoutWithoutCookieResponse))
	}
}

func sessionCookieWasRemoved(t *testing.T, cookies []*http.Cookie) bool {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name != "user_session" {
			continue
		}

		return cookie.Value == "" && cookie.Path == "/" && cookie.MaxAge == -1 && cookie.Expires.Before(time.Now()) && cookie.HttpOnly && !cookie.Secure && cookie.SameSite == http.SameSiteLaxMode
	}

	return false
}
