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
	loginFunc func(ctx context.Context, email, password string) (services.LoginResult, error)
}

func (s authServiceStub) Login(ctx context.Context, email, password string) (services.LoginResult, error) {
	return s.loginFunc(ctx, email, password)
}

func TestSessionHandlerLogin(t *testing.T) {
	const (
		email        = "USER@example.com"
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

	handler := handlers.NewSessionHandler(service, true)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"USER@example.com","password":"senha-segura"}`)).WithContext(ctx)
	response := httptest.NewRecorder()

	handler.Login(response, request)

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

	handler := handlers.NewSessionHandler(service, false)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
	response := httptest.NewRecorder()

	handler.Login(response, request)

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

			handler := handlers.NewSessionHandler(service, false)
			request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			handler.Login(response, request)

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
			wantBody:   "email or password is incorrect\n",
		},
		{
			name:       "unexpected error",
			serviceErr: unexpectedErr,
			wantStatus: http.StatusInternalServerError,
			wantBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := authServiceStub{
				loginFunc: func(context.Context, string, string) (services.LoginResult, error) {
					return services.LoginResult{}, tt.serviceErr
				},
			}

			handler := handlers.NewSessionHandler(service, false)
			request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
			response := httptest.NewRecorder()

			handler.Login(response, request)

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
