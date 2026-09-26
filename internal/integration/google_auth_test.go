//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/services"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/jackc/pgx/v5/pgxpool"
)

type googleOIDCExchange struct {
	code         string
	nonce        string
	codeVerifier string
}

type googleOIDCAuthorization struct {
	state        string
	nonce        string
	codeVerifier string
}

type googleOIDCClientFake struct {
	identitiesByCode      map[string]services.GoogleIdentity
	authorizationRequests []googleOIDCAuthorization
	exchanges             []googleOIDCExchange
}

func (c *googleOIDCClientFake) AuthorizationURL(state, nonce, codeVerifier string) string {
	c.authorizationRequests = append(c.authorizationRequests, googleOIDCAuthorization{
		state:        state,
		nonce:        nonce,
		codeVerifier: codeVerifier,
	})

	return "https://accounts.google.test/authorize?" + url.Values{"state": {state}}.Encode()
}

func (c *googleOIDCClientFake) ExchangeAndVerify(_ context.Context, code, expectedNonce, codeVerifier string) (services.GoogleIdentity, error) {
	c.exchanges = append(c.exchanges, googleOIDCExchange{
		code:         code,
		nonce:        expectedNonce,
		codeVerifier: codeVerifier,
	})

	identity, ok := c.identitiesByCode[code]
	if !ok {
		return services.GoogleIdentity{}, errors.New("unknown authorization code")
	}

	return identity, nil
}

func TestGoogleLoginIntegration(t *testing.T) {
	const (
		sessionTTL       = 24 * time.Hour
		googleEmail      = "google-integration@example.com"
		googleSubject    = "google-subject-new-user"
		passwordEmail    = "linked-integration@example.com"
		passwordSubject  = "google-subject-password-user"
		password         = "senha12345"
		newUserCode      = "new-user-authorization-code"
		repeatedUserCode = "repeated-user-authorization-code"
		linkedUserCode   = "linked-user-authorization-code"
	)

	pool := dbtest.Open(t)
	googleClient := &googleOIDCClientFake{
		identitiesByCode: map[string]services.GoogleIdentity{
			newUserCode: {
				Email:   googleEmail,
				Subject: googleSubject,
			},
			repeatedUserCode: {
				Email:   "changed-google-email@example.com",
				Subject: googleSubject,
			},
			linkedUserCode: {
				Email:   passwordEmail,
				Subject: passwordSubject,
			},
		},
	}
	applicationHandlers, _ := app.CompositionRoot(pool, testConfig(sessionTTL), googleClient)
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	var (
		googleUserID        uuid.UUID
		googleSessionCookie *http.Cookie
	)

	t.Run("creates a Google-only user and a local session", func(t *testing.T) {
		beforeLogin := time.Now().UTC()
		googleSessionCookie = performGoogleLogin(t, router, googleClient, newUserCode)
		afterLogin := time.Now().UTC()
		googleUserID = findUserIDByEmail(t, pool, googleEmail)

		assertGoogleIdentity(t, pool, googleUserID, googleSubject)
		assertPasswordCredentialExists(t, pool, googleUserID, false)
		assertSessionBelongsToUser(t, pool, googleSessionCookie.Value, googleUserID)

		var expiresAt time.Time
		if err := pool.QueryRow(t.Context(), "SELECT expires_at FROM user_sessions WHERE token_hash = $1", sessiontoken.Hash(googleSessionCookie.Value)).Scan(&expiresAt); err != nil {
			t.Fatalf("find Google session expiration: %v", err)
		}
		rememberedTTL := 30 * 24 * time.Hour
		if expiresAt.Before(beforeLogin.Add(rememberedTTL)) || expiresAt.After(afterLogin.Add(rememberedTTL)) {
			t.Errorf("Google session expiration = %v, want 30 days after login", expiresAt)
		}
		if !googleSessionCookie.Expires.Equal(expiresAt.Truncate(time.Second)) {
			t.Errorf("Google cookie expiration = %v, stored expiration = %v", googleSessionCookie.Expires, expiresAt)
		}
	})

	t.Run("uses the subject to find the same Google user", func(t *testing.T) {
		sessionCookie := performGoogleLogin(t, router, googleClient, repeatedUserCode)

		if got := countUsersByEmail(t, pool, googleEmail); got != 1 {
			t.Errorf("users with original Google email = %d, want 1", got)
		}
		if got := countUsersByEmail(t, pool, "changed-google-email@example.com"); got != 0 {
			t.Errorf("users with changed Google email = %d, want 0", got)
		}
		if got := countGoogleIdentitiesBySubject(t, pool, googleSubject); got != 1 {
			t.Errorf("Google identities for subject = %d, want 1", got)
		}

		assertSessionBelongsToUser(t, pool, sessionCookie.Value, googleUserID)
	})

	t.Run("links Google to an existing confirmed password user", func(t *testing.T) {
		passwordUserID := createConfirmedPasswordUser(t, pool, passwordEmail, password)
		sessionCookie := performGoogleLogin(t, router, googleClient, linkedUserCode)

		if got := findUserIDByEmail(t, pool, passwordEmail); got != passwordUserID {
			t.Errorf("user ID after linking = %s, want %s", got, passwordUserID)
		}
		if got := countUsersByEmail(t, pool, passwordEmail); got != 1 {
			t.Errorf("users with password account email = %d, want 1", got)
		}

		assertGoogleIdentity(t, pool, passwordUserID, passwordSubject)
		assertPasswordCredentialExists(t, pool, passwordUserID, true)
		assertSessionBelongsToUser(t, pool, sessionCookie.Value, passwordUserID)
	})

	t.Run("logs out a session created by Google", func(t *testing.T) {
		response := performRequestWithCookie(t, router, http.MethodDelete, "/sessions/current", googleSessionCookie)
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("logout status code = %d, want %d; body = %q", response.StatusCode, http.StatusNoContent, readResponseBody(t, response))
		}
		if !sessionCookieWasRemoved(t, response.Cookies()) {
			t.Error("logout response did not remove the Google session cookie")
		}
		if sessionHashExists(t, pool, sessiontoken.Hash(googleSessionCookie.Value)) {
			t.Error("Google session hash still exists after logout")
		}
	})
}

func performGoogleLogin(t *testing.T, router http.Handler, googleClient *googleOIDCClientFake, code string) *http.Cookie {
	t.Helper()

	startRequest := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	startRecorder := httptest.NewRecorder()
	authorizationsBeforeStart := len(googleClient.authorizationRequests)
	router.ServeHTTP(startRecorder, startRequest)
	startResponse := startRecorder.Result()
	defer startResponse.Body.Close()

	if startResponse.StatusCode != http.StatusFound {
		t.Fatalf("Google login start status code = %d, want %d; body = %q", startResponse.StatusCode, http.StatusFound, readResponseBody(t, startResponse))
	}

	stateCookie := requireCookie(t, startResponse.Cookies(), "google_auth_state")
	nonceCookie := requireCookie(t, startResponse.Cookies(), "google_auth_nonce")
	codeVerifierCookie := requireCookie(t, startResponse.Cookies(), "google_auth_code_verifier")
	if len(googleClient.authorizationRequests) != authorizationsBeforeStart+1 {
		t.Fatalf("OIDC authorization requests after start = %d, want %d", len(googleClient.authorizationRequests), authorizationsBeforeStart+1)
	}

	authorization := googleClient.authorizationRequests[len(googleClient.authorizationRequests)-1]
	if authorization.state != stateCookie.Value {
		t.Errorf("authorization state = %q, want cookie state %q", authorization.state, stateCookie.Value)
	}
	if authorization.nonce != nonceCookie.Value {
		t.Errorf("authorization nonce = %q, want cookie nonce %q", authorization.nonce, nonceCookie.Value)
	}
	if authorization.codeVerifier != codeVerifierCookie.Value {
		t.Errorf("authorization code verifier = %q, want cookie code verifier %q", authorization.codeVerifier, codeVerifierCookie.Value)
	}

	authorizationURL, err := url.Parse(startResponse.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse Google authorization URL: %v", err)
	}
	if got := authorizationURL.Query().Get("state"); got != stateCookie.Value {
		t.Fatalf("authorization URL state = %q, want cookie state %q", got, stateCookie.Value)
	}

	callbackTarget := "/auth/google/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(stateCookie.Value)
	callbackRequest := httptest.NewRequest(http.MethodGet, callbackTarget, nil)
	callbackRequest.AddCookie(stateCookie)
	callbackRequest.AddCookie(nonceCookie)
	callbackRequest.AddCookie(codeVerifierCookie)
	callbackRecorder := httptest.NewRecorder()
	exchangesBeforeCallback := len(googleClient.exchanges)

	router.ServeHTTP(callbackRecorder, callbackRequest)
	callbackResponse := callbackRecorder.Result()
	defer callbackResponse.Body.Close()

	if callbackResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("Google callback status code = %d, want %d; body = %q", callbackResponse.StatusCode, http.StatusNoContent, readResponseBody(t, callbackResponse))
	}
	if len(googleClient.exchanges) != exchangesBeforeCallback+1 {
		t.Fatalf("OIDC exchanges after callback = %d, want %d", len(googleClient.exchanges), exchangesBeforeCallback+1)
	}

	exchange := googleClient.exchanges[len(googleClient.exchanges)-1]
	if exchange.code != code {
		t.Errorf("exchanged authorization code = %q, want %q", exchange.code, code)
	}
	if exchange.nonce != nonceCookie.Value {
		t.Errorf("exchanged nonce = %q, want cookie nonce %q", exchange.nonce, nonceCookie.Value)
	}
	if exchange.codeVerifier != codeVerifierCookie.Value {
		t.Errorf("exchanged code verifier = %q, want cookie code verifier %q", exchange.codeVerifier, codeVerifierCookie.Value)
	}

	assertGoogleAuthorizationCookiesRemoved(t, callbackResponse.Cookies())
	return requireCookie(t, callbackResponse.Cookies(), "user_session")
}

func assertGoogleAuthorizationCookiesRemoved(t *testing.T, cookies []*http.Cookie) {
	t.Helper()

	for _, name := range []string{"google_auth_state", "google_auth_nonce", "google_auth_code_verifier"} {
		cookie := requireCookie(t, cookies, name)
		if cookie.Value != "" || cookie.Path != "/auth/google/callback" || cookie.MaxAge != -1 {
			t.Errorf("authorization cookie %q was not removed correctly", name)
		}
	}
}

func findUserIDByEmail(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := pool.QueryRow(t.Context(), "SELECT id FROM users WHERE email = $1", email).Scan(&userID); err != nil {
		t.Fatalf("find user by email: %v", err)
	}

	return userID
}

func countUsersByEmail(t *testing.T, pool *pgxpool.Pool, email string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count); err != nil {
		t.Fatalf("count users by email: %v", err)
	}

	return count
}

func countGoogleIdentitiesBySubject(t *testing.T, pool *pgxpool.Pool, subject string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM auth_identities WHERE provider = 'google' AND provider_user_id = $1", subject).Scan(&count); err != nil {
		t.Fatalf("count Google identities by subject: %v", err)
	}

	return count
}

func assertGoogleIdentity(t *testing.T, pool *pgxpool.Pool, wantUserID uuid.UUID, subject string) {
	t.Helper()

	var userID uuid.UUID
	if err := pool.QueryRow(
		t.Context(),
		"SELECT user_id FROM auth_identities WHERE provider = 'google' AND provider_user_id = $1",
		subject,
	).Scan(&userID); err != nil {
		t.Fatalf("find Google identity: %v", err)
	}
	if userID != wantUserID {
		t.Errorf("Google identity user ID = %s, want %s", userID, wantUserID)
	}
}

func assertPasswordCredentialExists(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, want bool) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM password_credentials WHERE user_id = $1)", userID).Scan(&exists); err != nil {
		t.Fatalf("check password credential: %v", err)
	}
	if exists != want {
		t.Errorf("password credential exists = %v, want %v", exists, want)
	}
}

func assertSessionBelongsToUser(t *testing.T, pool *pgxpool.Pool, token string, wantUserID uuid.UUID) {
	t.Helper()

	var userID uuid.UUID
	if err := pool.QueryRow(t.Context(), "SELECT user_id FROM user_sessions WHERE token_hash = $1", sessiontoken.Hash(token)).Scan(&userID); err != nil {
		t.Fatalf("find local session: %v", err)
	}
	if userID != wantUserID {
		t.Errorf("session user ID = %s, want %s", userID, wantUserID)
	}
}
