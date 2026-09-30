//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/argon2"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
)

func TestCreateURLIntegration(t *testing.T) {
	const (
		email    = "url-create-integration@example.com"
		password = "senha12345"
	)

	pool := dbtest.Open(t)
	userID := createConfirmedPasswordUser(t, pool, email, password)
	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	t.Run("rejects creation without a session", func(t *testing.T) {
		response := performJSONRequest(t, router, http.MethodPost, "/urls", `{"url":"https://example.com/public","visibility":"public"}`)
		defer response.Body.Close()

		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
		}

		var count int
		if err := pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM urls").Scan(&count); err != nil {
			t.Fatalf("count URLs after unauthenticated request: %v", err)
		}
		if count != 0 {
			t.Errorf("stored URLs = %d, want 0", count)
		}
	})

	login := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"url-create-integration@example.com","password":"senha12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d; body = %q", login.StatusCode, http.StatusNoContent, readResponseBody(t, login))
	}
	sessionCookie := requireCookie(t, login.Cookies(), "user_session")

	tests := []struct {
		name       string
		body       string
		original   string
		visibility string
		password   string
	}{
		{
			name:       "public URL",
			body:       `{"url":"https://example.com/public","visibility":"public"}`,
			original:   "https://example.com/public",
			visibility: "public",
		},
		{
			name:       "private URL",
			body:       `{"url":"https://example.com/private","visibility":"private","password":"senha-do-link"}`,
			original:   "https://example.com/private",
			visibility: "private",
			password:   "senha-do-link",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/urls", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(sessionCookie)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusCreated, response.Body.String())
			}

			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			code, ok := body["short_code"].(string)
			if !ok || len(code) < 6 {
				t.Fatalf("short_code = %v, want string with at least six characters", body["short_code"])
			}
			if got := body["short_url"]; got != "http://localhost:8080/"+code {
				t.Errorf("short_url = %v, want URL ending in %q", got, code)
			}
			if len(body) != 2 {
				t.Errorf("response fields = %v, want only short_code and short_url", body)
			}

			var (
				storedUserID     string
				storedOriginal   string
				storedVisibility string
				storedHash       *string
			)
			err := pool.QueryRow(t.Context(), `
				SELECT user_id::text, original_url, visibility::text, password_hash
				FROM urls WHERE short_code = $1
			`, code).Scan(&storedUserID, &storedOriginal, &storedVisibility, &storedHash)
			if err != nil {
				t.Fatalf("find created URL: %v", err)
			}
			if storedUserID != userID.String() || storedOriginal != tt.original || storedVisibility != tt.visibility {
				t.Errorf("stored URL = (%s, %q, %q), want (%s, %q, %q)",
					storedUserID, storedOriginal, storedVisibility, userID, tt.original, tt.visibility)
			}

			if tt.password == "" {
				if storedHash != nil {
					t.Error("public URL has a password hash")
				}
			} else {
				if storedHash == nil || *storedHash == tt.password {
					t.Fatal("private URL has no password hash or stored the plaintext password")
				}
				matches, err := (argon2.Argon2id{}).Compare(tt.password, *storedHash)
				if err != nil || !matches {
					t.Errorf("stored password hash does not match link password: match = %v, error = %v", matches, err)
				}
			}
		})
	}
}
