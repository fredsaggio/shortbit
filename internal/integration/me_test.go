//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
)

func TestMeIntegration(t *testing.T) {
	const (
		email      = "me-integration@example.com"
		password   = "senha12345"
		sessionTTL = 24 * time.Hour
	)

	pool := dbtest.Open(t)
	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(sessionTTL), unusedGoogleOIDCClient{}, noopHeatTracker{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	t.Run("rejects request without a session cookie", func(t *testing.T) {
		response := performRequestWithCookie(t, router, http.MethodGet, "/me", nil)
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusUnauthorized, readResponseBody(t, response))
		}
	})

	t.Run("rejects an unknown session token", func(t *testing.T) {
		cookie := &http.Cookie{Name: "user_session", Value: "unknown-session-token"}
		response := performRequestWithCookie(t, router, http.MethodGet, "/me", cookie)
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusUnauthorized, readResponseBody(t, response))
		}
	})

	registeredUserID := createConfirmedPasswordUser(t, pool, email, password)
	registeredUser := struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}{ID: registeredUserID.String(), Email: email}

	t.Run("rejects an expired session", func(t *testing.T) {
		const expiredToken = "expired-session-token"
		now := time.Now().UTC()

		_, err := pool.Exec(
			t.Context(),
			`INSERT INTO user_sessions(token_hash, user_id, created_at, expires_at) VALUES ($1, $2, $3, $4)`,
			sessiontoken.Hash(expiredToken),
			uuid.MustParse(registeredUser.ID),
			now.Add(-2*time.Hour),
			now.Add(-time.Hour),
		)
		if err != nil {
			t.Fatalf("insert expired session: %v", err)
		}

		cookie := &http.Cookie{Name: "user_session", Value: expiredToken}
		response := performRequestWithCookie(t, router, http.MethodGet, "/me", cookie)
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusUnauthorized, readResponseBody(t, response))
		}
	})

	loginResponse := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"me-integration@example.com","password":"senha12345"}`)
	defer loginResponse.Body.Close()

	if loginResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("login status code = %d, want %d; body = %q", loginResponse.StatusCode, http.StatusNoContent, readResponseBody(t, loginResponse))
	}

	sessionCookie := requireCookie(t, loginResponse.Cookies(), "user_session")

	t.Run("returns the authenticated user", func(t *testing.T) {
		response := performRequestWithCookie(t, router, http.MethodGet, "/me", sessionCookie)
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusOK, readResponseBody(t, response))
		}

		if got := response.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}

		var gotUser struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}
		if err := json.NewDecoder(response.Body).Decode(&gotUser); err != nil {
			t.Fatalf("decode /me response: %v", err)
		}

		if gotUser != registeredUser {
			t.Errorf("/me user = %+v, want %+v", gotUser, registeredUser)
		}
	})
}

func performRequestWithCookie(t *testing.T, handler http.Handler, method, target string, cookie *http.Cookie) *http.Response {
	t.Helper()

	request := httptest.NewRequest(method, target, http.NoBody)
	if cookie != nil {
		request.AddCookie(cookie)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Result()
}
