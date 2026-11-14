//go:build integration

package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/app"
	"github.com/fredsaggio/url-shortener/internal/db/dbtest"
	"github.com/fredsaggio/url-shortener/internal/server"
)

type accountBody struct {
	Email       string  `json:"email"`
	FullName    *string `json:"full_name"`
	HasPassword bool    `json:"has_password"`
	HasGoogle   bool    `json:"has_google"`
}

func TestAccountIntegration(t *testing.T) {
	const (
		email    = "account-integration@example.com"
		password = "Senha12345!"
	)

	pool := dbtest.Open(t)
	applicationHandlers, _, err := app.CompositionRoot(pool, testConfig(24*time.Hour), unusedGoogleOIDCClient{}, noopHeatTracker{}, noopDestinationCache{}, unavailableClickPublisher{})
	if err != nil {
		t.Fatalf("CompositionRoot() error = %v", err)
	}
	router := server.NewServer(applicationHandlers, pool).NewRouterHTTP()
	createConfirmedPasswordUser(t, pool, email, password)

	login := func(password string) *http.Cookie {
		t.Helper()
		response := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"`+password+`"}`)
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("login status = %d, want 204; body = %q", response.StatusCode, readResponseBody(t, response))
		}
		return requireCookie(t, response.Cookies(), "user_session")
	}

	readAccount := func(cookie *http.Cookie) accountBody {
		t.Helper()
		response := performRequestWithCookie(t, router, http.MethodGet, "/me", cookie)
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("/me status = %d, want 200; body = %q", response.StatusCode, readResponseBody(t, response))
		}
		var account accountBody
		if err := json.NewDecoder(response.Body).Decode(&account); err != nil {
			t.Fatalf("decode /me: %v", err)
		}
		return account
	}

	laptop := login(password)
	phone := login(password)

	t.Run("shows how the account signs in", func(t *testing.T) {
		account := readAccount(laptop)
		if account.Email != email || !account.HasPassword || account.HasGoogle || account.FullName != nil {
			t.Errorf("/me = %+v, want password account without a name yet", account)
		}
	})

	t.Run("updates the full name", func(t *testing.T) {
		response := performJSONRequestWithCookie(t, router, http.MethodPatch, "/me", `{"full_name":"  Ana   Souza "}`, laptop)
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("PATCH /me status = %d, want 200; body = %q", response.StatusCode, readResponseBody(t, response))
		}

		if account := readAccount(phone); account.FullName == nil || *account.FullName != "Ana Souza" {
			t.Errorf("full name = %v, want %q", account.FullName, "Ana Souza")
		}
	})

	t.Run("rejects a wrong current password without changing anything", func(t *testing.T) {
		response := performJSONRequestWithCookie(t, router, http.MethodPost, "/me/password",
			`{"current_password":"errada-123","new_password":"Nova-senha-1","new_password_confirmation":"Nova-senha-1"}`, laptop)
		defer response.Body.Close()
		if response.StatusCode != http.StatusBadRequest || readResponseBody(t, response) != "senha atual incorreta\n" {
			t.Fatalf("status = %d, want 400 senha atual incorreta", response.StatusCode)
		}
		readAccount(phone)
	})

	t.Run("changes the password, signs out other devices and renews this session", func(t *testing.T) {
		response := performJSONRequestWithCookie(t, router, http.MethodPost, "/me/password",
			`{"current_password":"Senha12345!","new_password":"Nova-senha-1","new_password_confirmation":"Nova-senha-1"}`, laptop)
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body = %q", response.StatusCode, readResponseBody(t, response))
		}
		renewed := requireCookie(t, response.Cookies(), "user_session")
		if renewed.Value == laptop.Value {
			t.Error("session ID was not renewed")
		}

		for name, cookie := range map[string]*http.Cookie{"old laptop session": laptop, "phone": phone} {
			response := performRequestWithCookie(t, router, http.MethodGet, "/me", cookie)
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s /me status = %d, want 401", name, response.StatusCode)
			}
		}

		readAccount(renewed)
		login("Nova-senha-1")

		old := performJSONRequest(t, router, http.MethodPost, "/sessions", `{"email":"`+email+`","password":"Senha12345!"}`)
		old.Body.Close()
		if old.StatusCode != http.StatusUnauthorized {
			t.Errorf("login with the old password status = %d, want 401", old.StatusCode)
		}
	})
}
