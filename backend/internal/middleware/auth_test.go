package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type authServiceStub struct {
	authenticateFunc func(ctx context.Context, token string) (uuid.UUID, error)
}

func (s authServiceStub) Authenticate(ctx context.Context, token string) (uuid.UUID, error) {
	return s.authenticateFunc(ctx, token)
}

func TestAuthenticatorAddsUserIDToRequestContext(t *testing.T) {
	const token = "raw-session-token"
	wantUserID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")

	auth := authServiceStub{
		authenticateFunc: func(_ context.Context, gotToken string) (uuid.UUID, error) {
			if gotToken != token {
				t.Errorf("Authenticate() token = %q, want %q", gotToken, token)
			}
			return wantUserID, nil
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		userID, ok := ctxval.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("user ID not found in request context")
		}
		if userID != wantUserID {
			t.Errorf("context user ID = %s, want %s", userID, wantUserID)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: token})
	response := httptest.NewRecorder()
	middleware.Authenticator(auth)(next).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusNoContent)
	}
	if !nextCalled {
		t.Error("next handler was not called")
	}
}

func TestAuthenticatorRejectsRequestWithoutCookie(t *testing.T) {
	auth := authServiceStub{
		authenticateFunc: func(context.Context, string) (uuid.UUID, error) {
			t.Fatal("Authenticate() should not be called")
			return uuid.Nil(), nil
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true })
	response := httptest.NewRecorder()
	middleware.Authenticator(auth)(next).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/me", nil))

	if response.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Error("next handler was called")
	}
}

func TestAuthenticatorRejectsUnauthenticatedSession(t *testing.T) {
	auth := authServiceStub{
		authenticateFunc: func(context.Context, string) (uuid.UUID, error) {
			return uuid.Nil(), services.ErrUnauthenticated
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true })
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: "invalid-session-token"})
	response := httptest.NewRecorder()
	middleware.Authenticator(auth)(next).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if nextCalled {
		t.Error("next handler was called")
	}
}

func TestAuthenticatorReturnsInternalServerErrorForAuthenticationFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	auth := authServiceStub{
		authenticateFunc: func(context.Context, string) (uuid.UUID, error) {
			return uuid.Nil(), wantErr
		},
	}

	nextCalled := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { nextCalled = true })
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: "session-token"})
	response := httptest.NewRecorder()
	middleware.Authenticator(auth)(next).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if nextCalled {
		t.Error("next handler was called")
	}
}
