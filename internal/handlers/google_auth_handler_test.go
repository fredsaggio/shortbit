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
