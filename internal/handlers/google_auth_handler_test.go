package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type googleAuthServiceStub struct {
	authorizationURLFunc func(state, nonce, codeVerifier string) string
	completeLoginFunc    func(ctx context.Context, code, expectedNonce, codeVerifier string) (services.LoginResult, error)
}

func (s googleAuthServiceStub) AuthorizationURL(state, nonce, codeVerifier string) string {
	return s.authorizationURLFunc(state, nonce, codeVerifier)
}

func (s googleAuthServiceStub) CompleteLogin(ctx context.Context, code, expectedNonce, codeVerifier string) (services.LoginResult, error) {
	return s.completeLoginFunc(ctx, code, expectedNonce, codeVerifier)
}

func TestGoogleAuthHandlerStart(t *testing.T) {
	const (
		state            = "fixed-state"
		nonce            = "fixed-nonce"
		codeVerifier     = "fixed-code-verifier"
		authorizationURL = "https://accounts.google.com/o/oauth2/v2/auth?state=fixed-state"
	)

	for _, cookieSecure := range []bool{false, true} {
		name := "insecure cookies"
		if cookieSecure {
			name = "secure cookies"
		}

		t.Run(name, func(t *testing.T) {
			generatorCalls := 0
			generator := func() (googleoidc.AuthorizationValues, error) {
				generatorCalls++
				return googleoidc.AuthorizationValues{State: state, Nonce: nonce, CodeVerifier: codeVerifier}, nil
			}

			googleAuth := googleAuthServiceStub{
				authorizationURLFunc: func(gotState, gotNonce, gotCodeVerifier string) string {
					if gotState != state {
						t.Errorf("AuthorizationURL() state = %q, want %q", gotState, state)
					}
					if gotNonce != nonce {
						t.Errorf("AuthorizationURL() nonce = %q, want %q", gotNonce, nonce)
					}
					if gotCodeVerifier != codeVerifier {
						t.Errorf("AuthorizationURL() code verifier = %q, want %q", gotCodeVerifier, codeVerifier)
					}

					return authorizationURL
				},
				completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
					t.Fatal("CompleteLogin() should not be called when starting Google authentication")
					return services.LoginResult{}, nil
				},
			}

			handler := handlers.NewGoogleAuthHandler(googleAuth, generator, cookieSecure)
			request := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
			response := httptest.NewRecorder()
			beforeRequest := time.Now().UTC()

			handler.Start(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != http.StatusFound {
				t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusFound)
			}
			if location := result.Header.Get("Location"); location != authorizationURL {
				t.Errorf("Location = %q, want %q", location, authorizationURL)
			}
			if generatorCalls != 1 {
				t.Errorf("authorization values generator calls = %d, want 1", generatorCalls)
			}

			wantCookies := map[string]string{
				"google_auth_state":         state,
				"google_auth_nonce":         nonce,
				"google_auth_code_verifier": codeVerifier,
			}
			if len(result.Cookies()) != len(wantCookies) {
				t.Fatalf("cookie count = %d, want %d", len(result.Cookies()), len(wantCookies))
			}

			for cookieName, wantValue := range wantCookies {
				cookie := findCookie(t, result.Cookies(), cookieName)
				if cookie.Value != wantValue {
					t.Errorf("cookie %q value = %q, want %q", cookieName, cookie.Value, wantValue)
				}
				if cookie.Path != "/auth/google/callback" {
					t.Errorf("cookie %q path = %q, want %q", cookieName, cookie.Path, "/auth/google/callback")
				}
				if cookie.MaxAge != 300 {
					t.Errorf("cookie %q MaxAge = %d, want 300", cookieName, cookie.MaxAge)
				}
				if cookie.Expires.Before(beforeRequest.Add(4*time.Minute+59*time.Second)) || cookie.Expires.After(time.Now().Add(5*time.Minute+time.Second)) {
					t.Errorf("cookie %q expiration = %v, want approximately five minutes", cookieName, cookie.Expires)
				}
				if !cookie.HttpOnly {
					t.Errorf("cookie %q HttpOnly = false, want true", cookieName)
				}
				if cookie.Secure != cookieSecure {
					t.Errorf("cookie %q Secure = %v, want %v", cookieName, cookie.Secure, cookieSecure)
				}
				if cookie.SameSite != http.SameSiteLaxMode {
					t.Errorf("cookie %q SameSite = %v, want %v", cookieName, cookie.SameSite, http.SameSiteLaxMode)
				}
			}
		})
	}
}

func TestGoogleAuthHandlerStartHandlesGeneratorError(t *testing.T) {
	wantErr := errors.New("random source unavailable")
	googleAuth := googleAuthServiceStub{
		authorizationURLFunc: func(string, string, string) string {
			t.Fatal("AuthorizationURL() should not be called after a generator error")
			return ""
		},
		completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
			t.Fatal("CompleteLogin() should not be called after a generator error")
			return services.LoginResult{}, nil
		},
	}
	generator := func() (googleoidc.AuthorizationValues, error) {
		return googleoidc.AuthorizationValues{}, wantErr
	}

	handler := handlers.NewGoogleAuthHandler(googleAuth, generator, true)
	request := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	response := httptest.NewRecorder()

	handler.Start(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusInternalServerError)
	}
	if len(result.Cookies()) != 0 {
		t.Errorf("cookie count = %d, want 0", len(result.Cookies()))
	}
	if location := result.Header.Get("Location"); location != "" {
		t.Errorf("Location = %q, want empty", location)
	}
}

func TestGoogleAuthHandlerCallback(t *testing.T) {
	const (
		code         = "authorization-code"
		state        = "stored-state"
		nonce        = "stored-nonce"
		codeVerifier = "stored-code-verifier"
		sessionToken = "plain-session-token"
	)

	type contextKey string
	const requestContextKey contextKey = "request-context"
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	for _, cookieSecure := range []bool{false, true} {
		name := "insecure cookies"
		if cookieSecure {
			name = "secure cookies"
		}

		t.Run(name, func(t *testing.T) {
			googleAuth := googleAuthServiceStub{
				authorizationURLFunc: func(string, string, string) string {
					t.Fatal("AuthorizationURL() should not be called in the callback")
					return ""
				},
				completeLoginFunc: func(ctx context.Context, gotCode, gotNonce, gotCodeVerifier string) (services.LoginResult, error) {
					if ctx.Value(requestContextKey) != "preserved" {
						t.Error("CompleteLogin() did not receive the request context")
					}
					if gotCode != code {
						t.Errorf("CompleteLogin() code = %q, want %q", gotCode, code)
					}
					if gotNonce != nonce {
						t.Errorf("CompleteLogin() nonce = %q, want %q", gotNonce, nonce)
					}
					if gotCodeVerifier != codeVerifier {
						t.Errorf("CompleteLogin() code verifier = %q, want %q", gotCodeVerifier, codeVerifier)
					}

					return services.LoginResult{Token: sessionToken, ExpiresAt: expiresAt}, nil
				},
			}

			handler := newGoogleCallbackHandler(t, googleAuth, cookieSecure)
			request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code="+code+"&state="+state, nil)
			request = request.WithContext(context.WithValue(request.Context(), requestContextKey, "preserved"))
			addGoogleAuthorizationCookies(request, state, nonce, codeVerifier)
			response := httptest.NewRecorder()

			handler.Callback(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != http.StatusNoContent {
				t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusNoContent)
			}
			if len(result.Cookies()) != 4 {
				t.Fatalf("cookie count = %d, want 4", len(result.Cookies()))
			}

			assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), cookieSecure)

			sessionCookie := findCookie(t, result.Cookies(), "user_session")
			if sessionCookie.Value != sessionToken {
				t.Errorf("session cookie value = %q, want %q", sessionCookie.Value, sessionToken)
			}
			if sessionCookie.Path != "/" {
				t.Errorf("session cookie path = %q, want %q", sessionCookie.Path, "/")
			}
			if !sessionCookie.Expires.Equal(expiresAt) {
				t.Errorf("session cookie expiration = %v, want %v", sessionCookie.Expires, expiresAt)
			}
			if !sessionCookie.HttpOnly {
				t.Error("session cookie HttpOnly = false, want true")
			}
			if sessionCookie.Secure != cookieSecure {
				t.Errorf("session cookie Secure = %v, want %v", sessionCookie.Secure, cookieSecure)
			}
			if sessionCookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("session cookie SameSite = %v, want %v", sessionCookie.SameSite, http.SameSiteLaxMode)
			}
		})
	}
}

func TestGoogleAuthHandlerCallbackRejectsProviderError(t *testing.T) {
	googleAuth := googleAuthServiceStub{
		authorizationURLFunc: func(string, string, string) string {
			t.Fatal("AuthorizationURL() should not be called in the callback")
			return ""
		},
		completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
			t.Fatal("CompleteLogin() should not be called when Google returns an error")
			return services.LoginResult{}, nil
		},
	}
	handler := newGoogleCallbackHandler(t, googleAuth, true)
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?error=access_denied", nil)
	response := httptest.NewRecorder()

	handler.Callback(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusBadRequest)
	}
	if len(result.Cookies()) != 3 {
		t.Fatalf("cookie count = %d, want 3", len(result.Cookies()))
	}
	assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), true)
}

func TestGoogleAuthHandlerCallbackRejectsMissingQueryValues(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "missing code and state", target: "/auth/google/callback"},
		{name: "missing state", target: "/auth/google/callback?code=authorization-code"},
		{name: "missing code", target: "/auth/google/callback?state=stored-state"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			googleAuth := googleAuthServiceStub{
				authorizationURLFunc: func(string, string, string) string {
					t.Fatal("AuthorizationURL() should not be called in the callback")
					return ""
				},
				completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
					t.Fatal("CompleteLogin() should not be called with an incomplete callback query")
					return services.LoginResult{}, nil
				},
			}
			handler := newGoogleCallbackHandler(t, googleAuth, false)
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			addGoogleAuthorizationCookies(request, "stored-state", "stored-nonce", "stored-code-verifier")
			response := httptest.NewRecorder()

			handler.Callback(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusBadRequest)
			}
			if len(result.Cookies()) != 3 {
				t.Fatalf("cookie count = %d, want 3", len(result.Cookies()))
			}
			assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), false)
		})
	}
}

func TestGoogleAuthHandlerCallbackRejectsInvalidAuthorizationCookies(t *testing.T) {
	const (
		state        = "stored-state"
		nonce        = "stored-nonce"
		codeVerifier = "stored-code-verifier"
	)

	tests := []struct {
		name         string
		state        string
		nonce        string
		codeVerifier string
	}{
		{name: "missing state", nonce: nonce, codeVerifier: codeVerifier},
		{name: "missing nonce", state: state, codeVerifier: codeVerifier},
		{name: "missing code verifier", state: state, nonce: nonce},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			googleAuth := googleAuthServiceStub{
				authorizationURLFunc: func(string, string, string) string {
					t.Fatal("AuthorizationURL() should not be called in the callback")
					return ""
				},
				completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
					t.Fatal("CompleteLogin() should not be called without all authorization cookies")
					return services.LoginResult{}, nil
				},
			}
			handler := newGoogleCallbackHandler(t, googleAuth, true)
			request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authorization-code&state="+state, nil)
			addGoogleAuthorizationCookies(request, test.state, test.nonce, test.codeVerifier)
			response := httptest.NewRecorder()

			handler.Callback(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusBadRequest)
			}
			if len(result.Cookies()) != 3 {
				t.Fatalf("cookie count = %d, want 3", len(result.Cookies()))
			}
			assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), true)
		})
	}
}

func TestGoogleAuthHandlerCallbackRejectsStateMismatch(t *testing.T) {
	googleAuth := googleAuthServiceStub{
		authorizationURLFunc: func(string, string, string) string {
			t.Fatal("AuthorizationURL() should not be called in the callback")
			return ""
		},
		completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
			t.Fatal("CompleteLogin() should not be called when state does not match")
			return services.LoginResult{}, nil
		},
	}
	handler := newGoogleCallbackHandler(t, googleAuth, false)
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authorization-code&state=received-state", nil)
	addGoogleAuthorizationCookies(request, "stored-state", "stored-nonce", "stored-code-verifier")
	response := httptest.NewRecorder()

	handler.Callback(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusBadRequest)
	}
	if len(result.Cookies()) != 3 {
		t.Fatalf("cookie count = %d, want 3", len(result.Cookies()))
	}
	assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), false)
}

func TestGoogleAuthHandlerCallbackHandlesServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid Google authentication", err: services.ErrGoogleAuthenticationFailed, wantStatus: http.StatusUnauthorized},
		{name: "email already exists", err: services.ErrEmailAlreadyExists, wantStatus: http.StatusConflict},
		{name: "unexpected error", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			googleAuth := googleAuthServiceStub{
				authorizationURLFunc: func(string, string, string) string {
					t.Fatal("AuthorizationURL() should not be called in the callback")
					return ""
				},
				completeLoginFunc: func(context.Context, string, string, string) (services.LoginResult, error) {
					return services.LoginResult{}, test.err
				},
			}
			handler := newGoogleCallbackHandler(t, googleAuth, true)
			request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=authorization-code&state=stored-state", nil)
			addGoogleAuthorizationCookies(request, "stored-state", "stored-nonce", "stored-code-verifier")
			response := httptest.NewRecorder()

			handler.Callback(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != test.wantStatus {
				t.Errorf("status code = %d, want %d", result.StatusCode, test.wantStatus)
			}
			if len(result.Cookies()) != 3 {
				t.Fatalf("cookie count = %d, want 3", len(result.Cookies()))
			}
			assertGoogleAuthorizationCookiesRemoved(t, result.Cookies(), true)
		})
	}
}

func newGoogleCallbackHandler(t *testing.T, googleAuth googleAuthServiceStub, cookieSecure bool) *handlers.GoogleAuthHandler {
	t.Helper()

	generator := func() (googleoidc.AuthorizationValues, error) {
		t.Fatal("authorization values generator should not be called in the callback")
		return googleoidc.AuthorizationValues{}, nil
	}

	return handlers.NewGoogleAuthHandler(googleAuth, generator, cookieSecure)
}

func addGoogleAuthorizationCookies(request *http.Request, state, nonce, codeVerifier string) {
	if state != "" {
		request.AddCookie(&http.Cookie{Name: "google_auth_state", Value: state})
	}
	if nonce != "" {
		request.AddCookie(&http.Cookie{Name: "google_auth_nonce", Value: nonce})
	}
	if codeVerifier != "" {
		request.AddCookie(&http.Cookie{Name: "google_auth_code_verifier", Value: codeVerifier})
	}
}

func assertGoogleAuthorizationCookiesRemoved(t *testing.T, cookies []*http.Cookie, wantSecure bool) {
	t.Helper()

	for _, name := range []string{"google_auth_state", "google_auth_nonce", "google_auth_code_verifier"} {
		cookie := findCookie(t, cookies, name)
		if cookie.Value != "" {
			t.Errorf("removed cookie %q value = %q, want empty", name, cookie.Value)
		}
		if cookie.Path != "/auth/google/callback" {
			t.Errorf("removed cookie %q path = %q, want %q", name, cookie.Path, "/auth/google/callback")
		}
		if cookie.MaxAge != -1 {
			t.Errorf("removed cookie %q MaxAge = %d, want -1", name, cookie.MaxAge)
		}
		if !cookie.Expires.Before(time.Now()) {
			t.Errorf("removed cookie %q expiration = %v, want a past time", name, cookie.Expires)
		}
		if !cookie.HttpOnly {
			t.Errorf("removed cookie %q HttpOnly = false, want true", name)
		}
		if cookie.Secure != wantSecure {
			t.Errorf("removed cookie %q Secure = %v, want %v", name, cookie.Secure, wantSecure)
		}
		if cookie.SameSite != http.SameSiteLaxMode {
			t.Errorf("removed cookie %q SameSite = %v, want %v", name, cookie.SameSite, http.SameSiteLaxMode)
		}
	}
}
