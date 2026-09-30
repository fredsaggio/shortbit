//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
)

func TestGetURLIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, "url-get-owner@example.com", "senha12345")
	createConfirmedPasswordUser(t, pool, "url-get-other@example.com", "senha12345")

	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	login := func(email string) *http.Cookie {
		t.Helper()
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"senha12345"}`)
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("login for %s status = %d, want 204; body = %q", email, response.StatusCode, readResponseBody(t, response))
		}
		return requireCookie(t, response.Cookies(), "user_session")
	}
	ownerCookie := login("url-get-owner@example.com")
	otherCookie := login("url-get-other@example.com")

	created := performJSONRequestWithCookie(t, router, http.MethodPost, "/urls",
		`{"url":"https://example.com/private","visibility":"private","password":"senha-do-link"}`, ownerCookie)
	defer created.Body.Close()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create private URL status = %d, want 201; body = %q", created.StatusCode, readResponseBody(t, created))
	}
	var createBody struct {
		ShortCode string `json:"short_code"`
	}
	if err := json.NewDecoder(created.Body).Decode(&createBody); err != nil || createBody.ShortCode == "" {
		t.Fatalf("decode created URL = %+v, error = %v", createBody, err)
	}

	if _, err := pool.Exec(t.Context(), "UPDATE urls SET click_count = 7 WHERE short_code = $1", createBody.ShortCode); err != nil {
		t.Fatalf("set click count: %v", err)
	}
	path := "/urls/" + createBody.ShortCode

	ownerResponse := performRequestWithCookie(t, router, http.MethodGet, path, ownerCookie)
	defer ownerResponse.Body.Close()
	if ownerResponse.StatusCode != http.StatusOK || !strings.HasPrefix(ownerResponse.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("owner response = (%d, %q), want 200 JSON", ownerResponse.StatusCode, ownerResponse.Header.Get("Content-Type"))
	}
	var metadata map[string]any
	if err := json.NewDecoder(ownerResponse.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode owner response: %v", err)
	}
	if metadata["short_code"] != createBody.ShortCode || metadata["original_url"] != "https://example.com/private" ||
		metadata["visibility"] != "private" || metadata["click_count"] != float64(7) ||
		metadata["created_at"] == "" || metadata["updated_at"] == "" || len(metadata) != 6 {
		t.Errorf("owner metadata = %v, want six public metadata fields with seven clicks", metadata)
	}
	if _, present := metadata["password_hash"]; present {
		t.Error("owner response exposed password hash")
	}

	otherResponse := performRequestWithCookie(t, router, http.MethodGet, path, otherCookie)
	defer otherResponse.Body.Close()
	if otherResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("other user status = %d, want 404", otherResponse.StatusCode)
	}
	otherBody := readResponseBody(t, otherResponse)

	missingResponse := performRequestWithCookie(t, router, http.MethodGet, "/urls/NoSuchCode123", ownerCookie)
	defer missingResponse.Body.Close()
	if missingResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("missing code status = %d, want 404", missingResponse.StatusCode)
	}
	if missingBody := readResponseBody(t, missingResponse); missingBody != otherBody {
		t.Errorf("404 bodies differ: other user = %q, missing code = %q", otherBody, missingBody)
	}

	unauthenticated := performRequestWithCookie(t, router, http.MethodGet, path, nil)
	defer unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Errorf("without cookie status = %d, want 401", unauthenticated.StatusCode)
	}
}
