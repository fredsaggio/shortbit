package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type linkAccessSessionServiceStub struct {
	createFunc func(context.Context, string, string) (services.LinkAccessSessionResult, error)
}

func (s linkAccessSessionServiceStub) CreateSession(ctx context.Context, shortCode, password string) (services.LinkAccessSessionResult, error) {
	return s.createFunc(ctx, shortCode, password)
}

func newUnlockRequest(t *testing.T, shortCode, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+shortCode+"/unlock", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetPathValue("code", shortCode)
	return request
}

func TestLinkAccessSessionHandlerRedirectPrivateLink(t *testing.T) {
	expiresAt := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	called := false
	service := linkAccessSessionServiceStub{createFunc: func(gotCtx context.Context, gotCode, gotPassword string) (services.LinkAccessSessionResult, error) {
		called = true
		if gotCtx.Value(contextKey{}) != "request-context" || gotCode != "Ab3dX9" || gotPassword != "senha do link" {
			t.Errorf("CreateSession() arguments = (%v, %q, %q), want request context, shortcode and form password", gotCtx, gotCode, gotPassword)
		}
		return services.LinkAccessSessionResult{Token: "raw-link-token", ExpiresAt: expiresAt}, nil
	}}
	handler := handlers.NewLinkAccessSessionHandler(service, true)
	request := newUnlockRequest(t, "Ab3dX9", url.Values{"password": {"senha do link"}}.Encode()).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.RedirectPrivateLink(response, request)

	if !called {
		t.Fatal("CreateSession() was not called")
	}
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/Ab3dX9" {
		t.Errorf("redirect = (%d, %q), want (303, /Ab3dX9)", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want exactly one link access cookie", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != "link_access_session" || cookie.Value != "raw-link-token" || cookie.Path != "/Ab3dX9" ||
		!cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || !cookie.Expires.Equal(expiresAt) || cookie.MaxAge <= 0 {
		t.Errorf("cookie = %+v, want scoped, secure, HttpOnly, Lax cookie expiring at %v", cookie, expiresAt)
	}
}

func TestLinkAccessSessionHandlerScopesCookieToEachLink(t *testing.T) {
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	service := linkAccessSessionServiceStub{createFunc: func(_ context.Context, code, _ string) (services.LinkAccessSessionResult, error) {
		return services.LinkAccessSessionResult{Token: "token-for-" + code, ExpiresAt: expiresAt}, nil
	}}
	handler := handlers.NewLinkAccessSessionHandler(service, false)

	for _, code := range []string{"Ab3dX9", "Qz7Rt2"} {
		response := httptest.NewRecorder()
		handler.RedirectPrivateLink(response, newUnlockRequest(t, code, "password=senha-do-link"))
		cookies := response.Result().Cookies()
		if response.Code != http.StatusSeeOther || len(cookies) != 1 {
			t.Fatalf("unlock %q = status %d, cookies %v; want 303 and one cookie", code, response.Code, cookies)
		}
		if cookies[0].Name != "link_access_session" || cookies[0].Path != "/"+code || cookies[0].Value != "token-for-"+code || cookies[0].Secure {
			t.Errorf("unlock %q cookie = %+v, want token and path specific to link with Secure=false", code, cookies[0])
		}
	}
}

func TestLinkAccessSessionHandlerRejectsInvalidForm(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty body", body: "", want: "senha do link obrigatória\n"},
		{name: "missing password", body: "other=value", want: "senha do link obrigatória\n"},
		{name: "empty password", body: "password=", want: "senha do link obrigatória\n"},
		{name: "malformed form", body: "password=%zz", want: "corpo da requisição inválido\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := linkAccessSessionServiceStub{createFunc: func(context.Context, string, string) (services.LinkAccessSessionResult, error) {
				t.Fatal("CreateSession() called for invalid form")
				return services.LinkAccessSessionResult{}, nil
			}}
			response := httptest.NewRecorder()
			handlers.NewLinkAccessSessionHandler(service, false).RedirectPrivateLink(response, newUnlockRequest(t, "Ab3dX9", tt.body))
			if response.Code != http.StatusBadRequest || response.Body.String() != tt.want {
				t.Errorf("response = (%d, %q), want (400, %q)", response.Code, response.Body.String(), tt.want)
			}
			if response.Header().Get("Set-Cookie") != "" || response.Header().Get("Location") != "" {
				t.Error("invalid form created a cookie or redirect")
			}
		})
	}
}

func TestLinkAccessSessionHandlerMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{name: "link not found", serviceErr: services.ErrURLNotFound, wantStatus: http.StatusNotFound, wantBody: "url não encontrada\n"},
		{name: "wrapped link not found", serviceErr: errors.Join(errors.New("context"), services.ErrURLNotFound), wantStatus: http.StatusNotFound, wantBody: "url não encontrada\n"},
		{name: "incorrect password", serviceErr: services.ErrIncorrectLinkPassword, wantStatus: http.StatusUnauthorized, wantBody: "senha incorreta\n"},
		{name: "internal error", serviceErr: errors.New("secret database details"), wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := linkAccessSessionServiceStub{createFunc: func(_ context.Context, code, password string) (services.LinkAccessSessionResult, error) {
				if code != "Ab3dX9" || password != "senha-do-link" {
					t.Errorf("CreateSession() arguments = (%q, %q), want shortcode and submitted password", code, password)
				}
				return services.LinkAccessSessionResult{}, tt.serviceErr
			}}
			response := httptest.NewRecorder()
			handlers.NewLinkAccessSessionHandler(service, false).RedirectPrivateLink(response, newUnlockRequest(t, "Ab3dX9", "password=senha-do-link"))
			if response.Code != tt.wantStatus || response.Body.String() != tt.wantBody {
				t.Errorf("response = (%d, %q), want (%d, %q)", response.Code, response.Body.String(), tt.wantStatus, tt.wantBody)
			}
			if response.Header().Get("Set-Cookie") != "" || response.Header().Get("Location") != "" {
				t.Error("failed unlock created a cookie or redirect")
			}
		})
	}
}
