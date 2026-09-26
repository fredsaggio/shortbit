//go:build integration

package integration_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/config"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
	"github.com/fredsaggio/url-shortener/internal/sessiontoken"
	"github.com/jackc/pgx/v5/pgxpool"
	"uuid"
)

func TestPasswordLoginIntegration(t *testing.T) {
	const (
		email      = "login-integration@example.com"
		password   = "senha12345"
		sessionTTL = 24 * time.Hour
	)

	pool := dbtest.Open(t)
	applicationHandlers, _ := app.CompositionRoot(pool, testConfig(sessionTTL), unusedGoogleOIDCClient{})
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	createConfirmedPasswordUser(t, pool, email, password)

	t.Run("incorrect password does not create session", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"login-integration@example.com","password":"senha-incorreta"}`)
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusUnauthorized, readResponseBody(t, response))
		}

		if len(response.Cookies()) != 0 {
			t.Error("incorrect password response contains a session cookie")
		}

		if got := countUserSessions(t, pool, email); got != 0 {
			t.Errorf("stored sessions = %d, want 0", got)
		}
	})

	var firstToken string

	t.Run("correct password creates cookie and stores only token hash", func(t *testing.T) {
		beforeLogin := time.Now().UTC()
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"login-integration@example.com","password":"senha12345"}`)
		afterLogin := time.Now().UTC()
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusNoContent, readResponseBody(t, response))
		}

		cookie := requireCookie(t, response.Cookies(), "user_session")
		firstToken = cookie.Value

		if firstToken == "" {
			t.Fatal("session cookie has an empty token")
		}

		if !cookie.HttpOnly {
			t.Error("session cookie HttpOnly = false, want true")
		}

		if cookie.Secure {
			t.Error("session cookie Secure = true, want false for local HTTP configuration")
		}

		wantHash := sessiontoken.Hash(firstToken)
		var (
			storedEmail     string
			storedTokenHash []byte
			createdAt       time.Time
			expiresAt       time.Time
		)

		err := pool.QueryRow(
			t.Context(),
			`
				SELECT u.email, us.token_hash, us.created_at, us.expires_at
				FROM user_sessions AS us
				JOIN users AS u ON u.id = us.user_id
				WHERE us.token_hash = $1
			`,
			wantHash,
		).Scan(&storedEmail, &storedTokenHash, &createdAt, &expiresAt)
		if err != nil {
			t.Fatalf("query persisted session: %v", err)
		}

		if storedEmail != email {
			t.Errorf("session email = %q, want %q", storedEmail, email)
		}

		if !bytes.Equal(storedTokenHash, wantHash) {
			t.Errorf("stored token hash = %x, want %x", storedTokenHash, wantHash)
		}

		if bytes.Equal(storedTokenHash, []byte(firstToken)) {
			t.Error("database stored the raw session token")
		}

		if len(storedTokenHash) != 32 {
			t.Errorf("stored token hash length = %d, want 32", len(storedTokenHash))
		}

		if createdAt.IsZero() {
			t.Error("stored session has a zero creation time")
		}

		if expiresAt.Before(beforeLogin.Add(sessionTTL)) || expiresAt.After(afterLogin.Add(sessionTTL)) {
			t.Errorf("stored expiration = %v, want between %v and %v", expiresAt, beforeLogin.Add(sessionTTL), afterLogin.Add(sessionTTL))
		}

		if !cookie.Expires.Equal(expiresAt.Truncate(time.Second)) {
			t.Errorf("cookie expiration = %v, stored expiration = %v", cookie.Expires, expiresAt)
		}

		if got := countUserSessions(t, pool, email); got != 1 {
			t.Errorf("stored sessions = %d, want 1", got)
		}
	})

	t.Run("two logins create independent sessions", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"login-integration@example.com","password":"senha12345"}`)
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusNoContent, readResponseBody(t, response))
		}

		secondToken := requireCookie(t, response.Cookies(), "user_session").Value
		if secondToken == firstToken {
			t.Fatal("two logins returned the same session token")
		}

		if got := countUserSessions(t, pool, email); got != 2 {
			t.Errorf("stored sessions = %d, want 2", got)
		}

		if !sessionHashExists(t, pool, sessiontoken.Hash(firstToken)) {
			t.Error("first session hash was not found in the database")
		}

		if !sessionHashExists(t, pool, sessiontoken.Hash(secondToken)) {
			t.Error("second session hash was not found in the database")
		}
	})

	t.Run("remember me creates a 30-day session", func(t *testing.T) {
		beforeLogin := time.Now().UTC()
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"login-integration@example.com","password":"senha12345","remember_me":true}`)
		afterLogin := time.Now().UTC()
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status code = %d, want %d; body = %q", response.StatusCode, http.StatusNoContent, readResponseBody(t, response))
		}

		cookie := requireCookie(t, response.Cookies(), "user_session")
		var expiresAt time.Time
		if err := pool.QueryRow(t.Context(), "SELECT expires_at FROM user_sessions WHERE token_hash = $1", sessiontoken.Hash(cookie.Value)).Scan(&expiresAt); err != nil {
			t.Fatalf("find remembered session expiration: %v", err)
		}
		rememberedTTL := 30 * 24 * time.Hour
		if expiresAt.Before(beforeLogin.Add(rememberedTTL)) || expiresAt.After(afterLogin.Add(rememberedTTL)) {
			t.Errorf("remembered session expiration = %v, want 30 days after login", expiresAt)
		}
		if !cookie.Expires.Equal(expiresAt.Truncate(time.Second)) {
			t.Errorf("remembered cookie expiration = %v, stored expiration = %v", cookie.Expires, expiresAt)
		}
	})
}

func testConfig(sessionTTL time.Duration) config.Config {
	return config.Config{
		Session: config.SessionConfig{TTL: sessionTTL, RememberedTTL: 720 * time.Hour, CookieSecure: false},
		Email:   config.EmailConfig{From: "noreply@example.com"},
		Resend:  config.ResendConfig{APIKey: "re_test_api_key"},
		PasswordRegistration: config.PasswordRegistrationConfig{
			CodeTTL:    10 * time.Minute,
			AttemptTTL: 30 * time.Minute,
		},
		PasswordReset: config.PasswordResetConfig{CodeTTL: 10 * time.Minute, AttemptTTL: 30 * time.Minute},
	}
}

func createConfirmedPasswordUser(t *testing.T, pool *pgxpool.Pool, email, password string) uuid.UUID {
	t.Helper()

	passwordHash, err := (argon2.Argon2id{}).Hash(password)
	if err != nil {
		t.Fatalf("hash test user password: %v", err)
	}

	var userID uuid.UUID
	err = pool.QueryRow(
		t.Context(),
		`INSERT INTO users (email, email_verified_at) VALUES ($1, NOW()) RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("insert confirmed test user: %v", err)
	}

	if _, err := pool.Exec(
		t.Context(),
		`INSERT INTO password_credentials (user_id, password_hash) VALUES ($1, $2)`,
		userID,
		passwordHash,
	); err != nil {
		t.Fatalf("insert test user password credential: %v", err)
	}

	return userID
}

func performJSONRequest(t *testing.T, handler http.Handler, method, target, body string) *http.Response {
	t.Helper()

	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response.Result()
}

func readResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return string(body)
}

func requireCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}

func countUserSessions(t *testing.T, pool *pgxpool.Pool, email string) int {
	t.Helper()

	var count int
	err := pool.QueryRow(
		t.Context(),
		`
			SELECT COUNT(*)
			FROM user_sessions AS us
			JOIN users AS u ON u.id = us.user_id
			WHERE u.email = $1
		`,
		email,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count user sessions: %v", err)
	}

	return count
}

func sessionHashExists(t *testing.T, pool *pgxpool.Pool, tokenHash []byte) bool {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM user_sessions WHERE token_hash = $1)", tokenHash).Scan(&exists); err != nil {
		t.Fatalf("check session hash: %v", err)
	}

	return exists
}
