package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type authServiceStub struct {
	loginFunc  func(ctx context.Context, email, password string) (services.LoginResult, error)
	logoutFunc func(ctx context.Context, token string) error
}

type loginRateLimiterStub struct {
	allowFunc func(key string) (bool, int)
}

func (s loginRateLimiterStub) Allow(key string) (bool, int) {
	return s.allowFunc(key)
}

func (s authServiceStub) LoginWithPassword(ctx context.Context, email, password string) (services.LoginResult, error) {
	return s.loginFunc(ctx, email, password)
}

func (s authServiceStub) Logout(ctx context.Context, token string) error {
	return s.logoutFunc(ctx, token)
}

func TestSessionHandlerLogin(t *testing.T) {
	const (
		email        = "user@example.com"
		password     = "senha-segura"
		token        = "raw-session-token"
		contextValue = "request-context"
	)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	service := authServiceStub{
		loginFunc: func(gotCtx context.Context, gotEmail, gotPassword string) (services.LoginResult, error) {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("Login() context value = %v, want %q", got, contextValue)
			}

			if gotEmail != email {
				t.Errorf("Login() email = %q, want %q", gotEmail, email)
			}

			if gotPassword != password {
				t.Errorf("Login() password = %q, want %q", gotPassword, password)
			}

			return services.LoginResult{Token: token, ExpiresAt: expiresAt}, nil
		},
	}

	handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), true)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"USER@example.com","password":"senha-segura"}`)).WithContext(ctx)
	response := httptest.NewRecorder()

	handler.LoginWithPassword(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", result.StatusCode, http.StatusNoContent)
	}

	if response.Body.Len() != 0 {
		t.Errorf("response body = %q, want empty body", response.Body.String())
	}

	cookie := findCookie(t, result.Cookies(), "user_session")

	if cookie.Value != token {
		t.Errorf("cookie value = %q, want %q", cookie.Value, token)
	}

	if cookie.Path != "/" {
		t.Errorf("cookie path = %q, want %q", cookie.Path, "/")
	}

	if !cookie.Expires.Equal(expiresAt) {
		t.Errorf("cookie expiration = %v, want %v", cookie.Expires, expiresAt)
	}

	maxAgeLimit := int((24 * time.Hour) / time.Second)
	if cookie.MaxAge < maxAgeLimit-1 || cookie.MaxAge > maxAgeLimit {
		t.Errorf("cookie MaxAge = %d, want between %d and %d", cookie.MaxAge, maxAgeLimit-1, maxAgeLimit)
	}

	if !cookie.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}

	if !cookie.Secure {
		t.Error("cookie Secure = false, want true")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}
}

func TestSessionHandlerLoginUsesCookieSecureConfiguration(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	service := authServiceStub{
		loginFunc: func(context.Context, string, string) (services.LoginResult, error) {
			return services.LoginResult{Token: "raw-session-token", ExpiresAt: expiresAt}, nil
		},
	}

	handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), false)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
	response := httptest.NewRecorder()

	handler.LoginWithPassword(response, request)

	cookie := findCookie(t, response.Result().Cookies(), "user_session")
	if cookie.Secure {
		t.Error("cookie Secure = true, want false")
	}
}

func TestSessionHandlerLoginRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed JSON", body: `{"email":`},
		{name: "unknown field", body: `{"email":"user@example.com","password":"senha-segura","remember":true}`},
		{name: "multiple JSON objects", body: `{"email":"user@example.com","password":"senha-segura"} {"email":"other@example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authServiceStub{
				loginFunc: func(context.Context, string, string) (services.LoginResult, error) {
					t.Fatal("Login() should not be called")
					return services.LoginResult{}, nil
				},
			}

			handler := handlers.NewSessionHandler(service, unexpectedLoginRateLimiter(t), false)
			request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			handler.LoginWithPassword(response, request)

			if response.Code != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", response.Code, http.StatusBadRequest)
			}

			if len(response.Result().Cookies()) != 0 {
				t.Error("response contains a session cookie for an invalid request")
			}
		})
	}
}

func TestSessionHandlerLoginMapsServiceErrors(t *testing.T) {
	unexpectedErr := errors.New("database unavailable")

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "incorrect credentials",
			serviceErr: services.ErrIncorrectEmailOrPassword,
			wantStatus: http.StatusUnauthorized,
			wantBody:   "email ou senha incorretos\n",
		},
		{
			name:       "unexpected error",
			serviceErr: unexpectedErr,
			wantStatus: http.StatusInternalServerError,
			wantBody:   "erro interno do servidor\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authServiceStub{
				loginFunc: func(context.Context, string, string) (services.LoginResult, error) {
					return services.LoginResult{}, tt.serviceErr
				},
			}

			handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), false)
			request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
			response := httptest.NewRecorder()

			handler.LoginWithPassword(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", response.Code, tt.wantStatus)
			}

			if response.Body.String() != tt.wantBody {
				t.Errorf("response body = %q, want %q", response.Body.String(), tt.wantBody)
			}

			if len(response.Result().Cookies()) != 0 {
				t.Error("response contains a session cookie after login failure")
			}

			if errors.Is(tt.serviceErr, unexpectedErr) && strings.Contains(response.Body.String(), unexpectedErr.Error()) {
				t.Error("response body exposes an internal error")
			}
		})
	}
}

func TestSessionHandlerLoginRateLimitsNormalizedEmail(t *testing.T) {
	const inputEmail = "  USER@Example.COM  "
	wantEmail := "user@example.com"

	service := authServiceStub{
		loginFunc: func(context.Context, string, string) (services.LoginResult, error) {
			t.Fatal("Login() should not be called after the email rate limit is reached")
			return services.LoginResult{}, nil
		},
	}

	limiter := loginRateLimiterStub{
		allowFunc: func(key string) (bool, int) {
			if key != wantEmail {
				t.Errorf("Allow() key = %q, want %q", key, wantEmail)
			}
			return false, 30
		},
	}

	handler := handlers.NewSessionHandler(service, limiter, false)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"  USER@Example.COM  ","password":"senha-segura"}`))
	response := httptest.NewRecorder()
	handler.LoginWithPassword(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusTooManyRequests)
	}

	if got := response.Header().Get("Retry-After"); got != "30" {
		t.Errorf("Retry-After = %q, want %q", got, "30")
	}

	if got := response.Body.String(); got != "muitas tentativas de login, tente novamente mais tarde\n" {
		t.Errorf("response body = %q, want rate limit message", got)
	}
}

func TestSessionHandlerLogout(t *testing.T) {
	const (
		token        = "raw-session-token"
		contextValue = "request-context"
	)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, contextValue)
	service := authServiceStub{
		logoutFunc: func(gotCtx context.Context, gotToken string) error {
			if got := gotCtx.Value(contextKey{}); got != contextValue {
				t.Errorf("Logout() context value = %v, want %q", got, contextValue)
			}

			if gotToken != token {
				t.Errorf("Logout() token = %q, want %q", gotToken, token)
			}

			return nil
		},
	}

	handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), true)
	request := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: token})
	response := httptest.NewRecorder()

	handler.Logout(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusNoContent)
	}

	if response.Body.Len() != 0 {
		t.Errorf("response body = %q, want empty body", response.Body.String())
	}

	assertRemovedSessionCookie(t, response.Result().Cookies(), true)
}

func TestSessionHandlerLogoutWithoutCookie(t *testing.T) {
	service := authServiceStub{
		logoutFunc: func(_ context.Context, token string) error {
			if token != "" {
				t.Errorf("Logout() token = %q, want empty", token)
			}

			return nil
		},
	}

	handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), false)
	request := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)
	response := httptest.NewRecorder()

	handler.Logout(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", response.Code, http.StatusNoContent)
	}

	assertRemovedSessionCookie(t, response.Result().Cookies(), false)
}

func TestSessionHandlerLogoutMapsServiceError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	service := authServiceStub{
		logoutFunc: func(context.Context, string) error {
			return wantErr
		},
	}

	handler := handlers.NewSessionHandler(service, allowAllLoginRateLimiter(), false)
	request := httptest.NewRequest(http.MethodDelete, "/sessions/current", nil)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: "raw-session-token"})
	response := httptest.NewRecorder()

	handler.Logout(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}

	if got := response.Body.String(); got != "erro interno do servidor\n" {
		t.Errorf("response body = %q, want generic internal error", got)
	}

	if strings.Contains(response.Body.String(), wantErr.Error()) {
		t.Error("response body exposes an internal error")
	}

	if len(response.Result().Cookies()) != 0 {
		t.Error("response removes the cookie even though session revocation failed")
	}
}

func allowAllLoginRateLimiter() loginRateLimiterStub {
	return loginRateLimiterStub{
		allowFunc: func(string) (bool, int) {
			return true, 0
		},
	}
}

func unexpectedLoginRateLimiter(t *testing.T) loginRateLimiterStub {
	t.Helper()

	return loginRateLimiterStub{
		allowFunc: func(string) (bool, int) {
			t.Fatal("Allow() should not be called for invalid JSON")
			return false, 0
		},
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}

func assertRemovedSessionCookie(t *testing.T, cookies []*http.Cookie, wantSecure bool) {
	t.Helper()

	cookie := findCookie(t, cookies, "user_session")

	if cookie.Value != "" {
		t.Errorf("cookie value = %q, want empty", cookie.Value)
	}

	if cookie.Path != "/" {
		t.Errorf("cookie path = %q, want %q", cookie.Path, "/")
	}

	if cookie.MaxAge != -1 {
		t.Errorf("cookie MaxAge = %d, want -1", cookie.MaxAge)
	}

	if !cookie.Expires.Before(time.Now()) {
		t.Errorf("cookie expiration = %v, want a time in the past", cookie.Expires)
	}

	if !cookie.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}

	if cookie.Secure != wantSecure {
		t.Errorf("cookie Secure = %t, want %t", cookie.Secure, wantSecure)
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteLaxMode)
	}
}
