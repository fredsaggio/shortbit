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
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
)

func TestListURLIntegration(t *testing.T) {
	pool := dbtest.Open(t)
	createConfirmedPasswordUser(t, pool, "url-list-owner@example.com", "senha12345")
	createConfirmedPasswordUser(t, pool, "url-list-other@example.com", "senha12345")

	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/urls", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", unauthenticated.Code)
	}

	login := func(email string) *http.Cookie {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/sessions", strings.NewReader(`{"email":"`+email+`","password":"senha12345"}`))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("login status = %d, want 204; body = %q", response.Code, response.Body.String())
		}
		return requireCookie(t, response.Result().Cookies(), "user_session")
	}
	ownerCookie := login("url-list-owner@example.com")
	otherCookie := login("url-list-other@example.com")

	createURL := func(cookie *http.Cookie, originalURL string) string {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/urls", strings.NewReader(`{"url":"`+originalURL+`","visibility":"public"}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("create URL status = %d, want 201; body = %q", response.Code, response.Body.String())
		}
		var body struct {
			ShortCode string `json:"short_code"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.ShortCode == "" {
			t.Fatalf("decode created URL = %+v, error = %v", body, err)
		}
		return body.ShortCode
	}
	firstCode := createURL(ownerCookie, "https://example.com/first")
	secondCode := createURL(ownerCookie, "https://example.com/second")
	otherCode := createURL(otherCookie, "https://example.com/other")

	type listResponse struct {
		URLs       []map[string]any `json:"urls"`
		NextCursor *string          `json:"next_cursor"`
	}
	listURLs := func(cookie *http.Cookie, path string) (int, listResponse) {
		t.Helper()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var body listResponse
		if response.Code == http.StatusOK {
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if strings.Contains(response.Body.String(), "password_hash") {
				t.Fatal("list response exposed password hash field")
			}
		}
		return response.Code, body
	}

	status, firstPage := listURLs(ownerCookie, "/urls?limit=1")
	if status != http.StatusOK || len(firstPage.URLs) != 1 || firstPage.URLs[0]["short_code"] != secondCode || firstPage.NextCursor == nil {
		t.Fatalf("first page = (%d, %+v), want newest owner URL and next cursor", status, firstPage)
	}
	status, secondPage := listURLs(ownerCookie, "/urls?limit=1&cursor="+*firstPage.NextCursor)
	if status != http.StatusOK || len(secondPage.URLs) != 1 || secondPage.URLs[0]["short_code"] != firstCode || secondPage.NextCursor != nil {
		t.Fatalf("second page = (%d, %+v), want oldest owner URL and no next cursor", status, secondPage)
	}
	status, otherPage := listURLs(otherCookie, "/urls")
	if status != http.StatusOK || len(otherPage.URLs) != 1 || otherPage.URLs[0]["short_code"] != otherCode || otherPage.NextCursor != nil {
		t.Fatalf("other user page = (%d, %+v), want only other user's URL", status, otherPage)
	}
	if status, _ := listURLs(ownerCookie, "/urls?limit=21"); status != http.StatusBadRequest {
		t.Errorf("limit above maximum status = %d, want 400", status)
	}
}
