package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/services"
)

func TestAccountHandlerDeleteClearsTheSession(t *testing.T) {
	service := &accountServiceStub{}
	handler := handlers.NewAccountHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	response := httptest.NewRecorder()

	handler.Delete(response, authenticatedRequest(http.MethodDelete, "/me", `{"password":"Senha-atual1"}`))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %q", response.Code, response.Body.String())
	}
	if got := service.gotDeleteInput; len(got) != 2 || got[0] != "Senha-atual1" || got[1] != "" {
		t.Errorf("DeleteAccount input = %v", got)
	}
	cleared := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "user_session" && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("user_session cookie not cleared")
	}
}

func TestAccountHandlerDeleteMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		err    error
		status int
		want   string
	}{
		{name: "wrong password", body: `{"password":"x"}`, err: services.ErrIncorrectCurrentPassword, status: http.StatusBadRequest, want: "senha incorreta"},
		{name: "wrong email", body: `{"email":"x@example.com"}`, err: services.ErrAccountEmailMismatch, status: http.StatusBadRequest, want: "email não confere"},
		{name: "unknown field", body: `{"confirm":true}`, status: http.StatusBadRequest, want: "corpo da requisição inválido"},
		{name: "infrastructure", body: `{"password":"x"}`, err: errors.New("database unavailable"), status: http.StatusInternalServerError, want: "erro interno do servidor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAccountHandler(&accountServiceStub{err: tt.err}, allowAllRegistrationRateLimiterStub{}, false)
			response := httptest.NewRecorder()

			handler.Delete(response, authenticatedRequest(http.MethodDelete, "/me", tt.body))

			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.want) {
				t.Errorf("got %d %q, want %d containing %q", response.Code, response.Body.String(), tt.status, tt.want)
			}
			for _, cookie := range response.Result().Cookies() {
				if cookie.Name == "user_session" {
					t.Error("failed deletion touched the session cookie")
				}
			}
		})
	}
}

func TestAccountHandlerDeleteIsRateLimitedPerAccount(t *testing.T) {
	service := &accountServiceStub{}
	var key string
	limiter := registrationRateLimiterStub{allowFunc: func(k string) (bool, int) {
		key = k
		return false, 120
	}}
	handler := handlers.NewAccountHandler(service, limiter, false)
	response := httptest.NewRecorder()

	handler.Delete(response, authenticatedRequest(http.MethodDelete, "/me", `{"password":"Senha-atual1"}`))

	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "120" {
		t.Fatalf("got %d Retry-After %q, want 429 and 120", response.Code, response.Header().Get("Retry-After"))
	}
	if key != accountTestUserID.String() {
		t.Errorf("rate limit key = %q, want the account ID", key)
	}
	if service.gotDeleteInput != nil {
		t.Error("service called while rate limited")
	}
}
