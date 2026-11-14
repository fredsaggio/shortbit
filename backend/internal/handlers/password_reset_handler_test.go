package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type passwordResetStarterStub struct {
	startFunc   func(context.Context, string, string) (services.PasswordResetStartResult, error)
	confirmFunc func(context.Context, string, string, string) error
}

func (s passwordResetStarterStub) Start(ctx context.Context, email, currentToken string) (services.PasswordResetStartResult, error) {
	return s.startFunc(ctx, email, currentToken)
}

func (s passwordResetStarterStub) Confirm(ctx context.Context, token, code, password string) error {
	return s.confirmFunc(ctx, token, code, password)
}

func TestPasswordResetHandlerStartSetsCookieAndReturnsGenericAccepted(t *testing.T) {
	service := passwordResetStarterStub{startFunc: func(ctx context.Context, email, currentToken string) (services.PasswordResetStartResult, error) {
		if ctx == nil || email != "user@example.com" || currentToken != "previous-token" {
			t.Errorf("Start() received context=%v email=%q token=%q", ctx, email, currentToken)
		}
		return services.PasswordResetStartResult{Token: "new-token"}, nil
	}}
	handler := handlers.NewPasswordResetHandler(service, registrationRateLimiterStub{allowFunc: func(key string) (bool, int) {
		if key != "user@example.com" {
			t.Errorf("rate limit key = %q", key)
		}
		return true, 0
	}}, true)
	request := httptest.NewRequest(http.MethodPost, "/password-resets", strings.NewReader(`{"email":"  USER@Example.COM  "}`))
	request.AddCookie(&http.Cookie{Name: "password_reset", Value: "previous-token"})
	response := httptest.NewRecorder()
	handler.Start(response, request)

	if response.Code != http.StatusAccepted || response.Body.Len() != 0 {
		t.Fatalf("response = (%d, %q), want (202, empty body)", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "password_reset" || cookies[0].Value != "new-token" {
		t.Fatalf("cookies = %+v, want password_reset=new-token", cookies)
	}
	cookie := cookies[0]
	if cookie.Path != "/password-resets" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie attributes = %+v", cookie)
	}
}

func TestPasswordResetHandlerStartUsesEmptyTokenWhenCookieIsAbsent(t *testing.T) {
	service := passwordResetStarterStub{startFunc: func(_ context.Context, email, currentToken string) (services.PasswordResetStartResult, error) {
		if email != "user@example.com" || currentToken != "" {
			t.Errorf("Start() = (%q, %q), want normalized email and empty token", email, currentToken)
		}
		return services.PasswordResetStartResult{Token: "dummy-or-real-token"}, nil
	}}
	handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	request := httptest.NewRequest(http.MethodPost, "/password-resets", strings.NewReader(`{"email":"user@example.com"}`))
	response := httptest.NewRecorder()
	handler.Start(response, request)
	if response.Code != http.StatusAccepted || len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].Secure {
		t.Fatalf("response = (%d, %+v), want 202 and insecure local cookie", response.Code, response.Result().Cookies())
	}
}

func TestPasswordResetHandlerStartRejectsInvalidBodyAndRateLimit(t *testing.T) {
	service := passwordResetStarterStub{startFunc: func(context.Context, string, string) (services.PasswordResetStartResult, error) {
		t.Fatal("service must not be called")
		return services.PasswordResetStartResult{}, nil
	}}
	for _, body := range []string{`{`, `{"email":"user@example.com","unexpected":true}`, `{"email":"user@example.com"}{}`} {
		handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, false)
		response := httptest.NewRecorder()
		handler.Start(response, httptest.NewRequest(http.MethodPost, "/password-resets", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Errorf("body %q returned %d, want 400", body, response.Code)
		}
	}

	handler := handlers.NewPasswordResetHandler(service, registrationRateLimiterStub{allowFunc: func(string) (bool, int) { return false, 42 }}, false)
	response := httptest.NewRecorder()
	handler.Start(response, httptest.NewRequest(http.MethodPost, "/password-resets", strings.NewReader(`{"email":"user@example.com"}`)))
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "42" {
		t.Errorf("rate-limited response = (%d, %q)", response.Code, response.Header().Get("Retry-After"))
	}
}

func TestPasswordResetHandlerStartMapsServiceErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid email", err: services.ErrInvalidEmail, status: http.StatusBadRequest},
		{name: "infrastructure error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := passwordResetStarterStub{startFunc: func(context.Context, string, string) (services.PasswordResetStartResult, error) {
				return services.PasswordResetStartResult{}, tt.err
			}}
			handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			response := httptest.NewRecorder()
			handler.Start(response, httptest.NewRequest(http.MethodPost, "/password-resets", strings.NewReader(`{"email":"user@example.com"}`)))
			if response.Code != tt.status || len(response.Result().Cookies()) != 0 {
				t.Errorf("response = (%d, %+v), want status %d without cookie", response.Code, response.Result().Cookies(), tt.status)
			}
		})
	}
}

func TestPasswordResetHandlerConfirmConsumesCookieAndReturnsNoContent(t *testing.T) {
	service := passwordResetStarterStub{confirmFunc: func(ctx context.Context, token, code, password string) error {
		if ctx == nil || token != "reset-token" || code != "00123456" || password != "nova-senha-segura" {
			t.Errorf("Confirm() = (%q, %q, %q)", token, code, password)
		}
		return nil
	}}
	handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	request := httptest.NewRequest(http.MethodPost, "/password-resets/confirm", strings.NewReader(`{"code":"00123456","password":"nova-senha-segura","password_confirmation":"nova-senha-segura"}`))
	request.AddCookie(&http.Cookie{Name: "password_reset", Value: "reset-token"})
	response := httptest.NewRecorder()
	handler.Confirm(response, request)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Errorf("response = (%d, %q), want 204 with no body", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "password_reset" || cookies[0].MaxAge != -1 || cookies[0].Path != "/password-resets" || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Errorf("cookie clearing = %+v", cookies)
	}
}

func TestPasswordResetHandlerConfirmRejectsInvalidBodyAndMissingCookie(t *testing.T) {
	service := passwordResetStarterStub{confirmFunc: func(context.Context, string, string, string) error {
		t.Fatal("service must not be called")
		return nil
	}}
	handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	for _, body := range []string{`{`, `{"code":"12345678","password":"new-password","password_confirmation":"different"}`, `{"code":"12345678","password":"new-password","password_confirmation":"new-password"}{}`} {
		response := httptest.NewRecorder()
		handler.Confirm(response, httptest.NewRequest(http.MethodPost, "/password-resets/confirm", strings.NewReader(body)))
		if response.Code != http.StatusBadRequest {
			t.Errorf("body %q status = %d, want 400", body, response.Code)
		}
	}
	response := httptest.NewRecorder()
	handler.Confirm(response, httptest.NewRequest(http.MethodPost, "/password-resets/confirm", strings.NewReader(`{"code":"12345678","password":"new-password","password_confirmation":"new-password"}`)))
	if response.Code != http.StatusGone || len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].MaxAge != -1 {
		t.Errorf("missing cookie response = (%d, %+v), want 410 and cleared cookie", response.Code, response.Result().Cookies())
	}
}

func TestPasswordResetHandlerConfirmMapsServiceErrors(t *testing.T) {
	for _, tt := range []struct {
		name        string
		err         error
		status      int
		clearCookie bool
	}{
		{name: "unavailable", err: services.ErrPasswordResetAttemptUnavailable, status: http.StatusGone, clearCookie: true},
		{name: "locked", err: services.ErrPasswordResetAttemptLocked, status: http.StatusTooManyRequests},
		{name: "code expired", err: services.ErrPasswordResetCodeExpired, status: http.StatusBadRequest},
		{name: "invalid code", err: services.ErrPasswordResetCodeInvalid, status: http.StatusBadRequest},
		{name: "short password", err: services.ErrPasswordTooShort, status: http.StatusBadRequest},
		{name: "long password", err: services.ErrPasswordTooLong, status: http.StatusBadRequest},
		{name: "infrastructure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := passwordResetStarterStub{confirmFunc: func(context.Context, string, string, string) error { return tt.err }}
			handler := handlers.NewPasswordResetHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			request := httptest.NewRequest(http.MethodPost, "/password-resets/confirm", strings.NewReader(`{"code":"12345678","password":"new-password","password_confirmation":"new-password"}`))
			request.AddCookie(&http.Cookie{Name: "password_reset", Value: "reset-token"})
			response := httptest.NewRecorder()
			handler.Confirm(response, request)
			if response.Code != tt.status {
				t.Errorf("status = %d, want %d", response.Code, tt.status)
			}
			if got := len(response.Result().Cookies()) > 0; got != tt.clearCookie {
				t.Errorf("cookie cleared = %t, want %t", got, tt.clearCookie)
			}
		})
	}
}
