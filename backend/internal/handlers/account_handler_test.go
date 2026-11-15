package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem/problemtest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

var accountTestUserID = uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c318")

type accountServiceStub struct {
	account        models.Account
	err            error
	changeResult   services.PasswordChangeResult
	gotFullName    string
	gotChangeInput []string
	gotDeleteInput []string
}

func (s *accountServiceStub) DeleteAccount(_ context.Context, _ uuid.UUID, password, email string) error {
	s.gotDeleteInput = []string{password, email}
	return s.err
}

func (s *accountServiceStub) GetAccount(context.Context, uuid.UUID) (models.Account, error) {
	return s.account, s.err
}

func (s *accountServiceStub) UpdateFullName(_ context.Context, _ uuid.UUID, fullName string) (models.Account, error) {
	s.gotFullName = fullName
	return s.account, s.err
}

func (s *accountServiceStub) ChangePassword(_ context.Context, _ uuid.UUID, token, current, newPassword, confirmation string) (services.PasswordChangeResult, error) {
	s.gotChangeInput = []string{token, current, newPassword, confirmation}
	return s.changeResult, s.err
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	return request.WithContext(ctxval.ContextWithUserID(request.Context(), accountTestUserID))
}

func TestAccountHandlerGetReturnsAccount(t *testing.T) {
	name := "Ana Souza"
	created := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	service := &accountServiceStub{account: models.Account{
		User:        models.User{ID: accountTestUserID, Email: "ana@example.com", FullName: &name, CreatedAt: created},
		HasPassword: true,
	}}
	handler := handlers.NewAccountHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	response := httptest.NewRecorder()

	handler.Get(response, authenticatedRequest(http.MethodGet, "/me", ""))

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]any{
		"id": accountTestUserID.String(), "email": "ana@example.com", "full_name": "Ana Souza",
		"has_password": true, "has_google": false, "created_at": "2026-11-14T12:00:00Z",
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("%s = %v, want %v", key, body[key], value)
		}
	}
	if len(body) != len(want) {
		t.Errorf("response has %d fields, want %d: %v", len(body), len(want), body)
	}
}

func TestAccountHandlerUpdateMapsNameErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "required", err: services.ErrFullNameRequired, wantStatus: http.StatusBadRequest, wantBody: "informe o nome completo\n"},
		{name: "too long", err: services.ErrFullNameTooLong, wantStatus: http.StatusBadRequest, wantBody: "nome completo muito longo; use até 100 caracteres\n"},
		{name: "internal", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &accountServiceStub{err: tt.err}
			handler := handlers.NewAccountHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			response := httptest.NewRecorder()

			handler.Update(response, authenticatedRequest(http.MethodPatch, "/me", `{"full_name":"Ana"}`))

			if response.Code != tt.wantStatus || problemtest.Text(response.Body.String()) != tt.wantBody {
				t.Errorf("response = (%d, %q), want (%d, %q)", response.Code, problemtest.Text(response.Body.String()), tt.wantStatus, tt.wantBody)
			}
			if service.gotFullName != "Ana" {
				t.Errorf("service full name = %q, want Ana", service.gotFullName)
			}
		})
	}
}

func TestAccountHandlerChangePasswordRenewsSessionCookie(t *testing.T) {
	expires := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Second)
	service := &accountServiceStub{changeResult: services.PasswordChangeResult{
		Session:    services.LoginResult{Token: "new-session-token", ExpiresAt: expires},
		RememberMe: true,
	}}
	handler := handlers.NewAccountHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	request := authenticatedRequest(http.MethodPost, "/me/password", `{"current_password":"Senha-atual1","new_password":"Senha-nova-123","new_password_confirmation":"Senha-nova-123"}`)
	request.AddCookie(&http.Cookie{Name: "user_session", Value: "current-session-token"})
	response := httptest.NewRecorder()

	handler.ChangePassword(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %q", response.Code, response.Body.String())
	}
	wantInput := []string{"current-session-token", "Senha-atual1", "Senha-nova-123", "Senha-nova-123"}
	for i := range wantInput {
		if service.gotChangeInput[i] != wantInput[i] {
			t.Errorf("service input = %v, want %v", service.gotChangeInput, wantInput)
			break
		}
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "user_session" || cookies[0].Value != "new-session-token" || !cookies[0].Secure || !cookies[0].Expires.Equal(expires) {
		t.Errorf("cookies = %+v, want a renewed persistent secure user_session", cookies)
	}
}

func TestAccountHandlerChangePasswordMapsErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{name: "mismatch", err: services.ErrPasswordConfirmationMismatch, wantStatus: http.StatusBadRequest, wantBody: "as senhas não coincidem\n"},
		{name: "too short", err: services.ErrPasswordTooShort, wantStatus: http.StatusBadRequest, wantBody: "senha muito curta\n"},
		{name: "no uppercase", err: services.ErrPasswordMissingUppercase, wantStatus: http.StatusBadRequest, wantBody: "a senha precisa ter pelo menos uma letra maiúscula\n"},
		{name: "wrong current", err: services.ErrIncorrectCurrentPassword, wantStatus: http.StatusBadRequest, wantBody: "senha atual incorreta\n"},
		{name: "same password", err: services.ErrSamePassword, wantStatus: http.StatusBadRequest, wantBody: "a nova senha deve ser diferente da atual\n"},
		{name: "Google-only", err: services.ErrAccountHasNoPassword, wantStatus: http.StatusConflict, wantBody: "esta conta entra com Google e não tem senha\n"},
		{name: "stale session", err: services.ErrUnauthenticated, wantStatus: http.StatusUnauthorized, wantBody: "autenticação necessária\n"},
		{name: "internal", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAccountHandler(&accountServiceStub{err: tt.err}, allowAllRegistrationRateLimiterStub{}, false)
			request := authenticatedRequest(http.MethodPost, "/me/password", `{"current_password":"a","new_password":"b","new_password_confirmation":"b"}`)
			request.AddCookie(&http.Cookie{Name: "user_session", Value: "token"})
			response := httptest.NewRecorder()

			handler.ChangePassword(response, request)

			if response.Code != tt.wantStatus || problemtest.Text(response.Body.String()) != tt.wantBody {
				t.Errorf("response = (%d, %q), want (%d, %q)", response.Code, problemtest.Text(response.Body.String()), tt.wantStatus, tt.wantBody)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Error("failed password change set a cookie")
			}
		})
	}
}

func TestAccountHandlerChangePasswordIsRateLimitedPerAccount(t *testing.T) {
	var gotKey string
	limiter := registrationRateLimiterStub{allowFunc: func(key string) (bool, int) {
		gotKey = key
		return false, 90
	}}
	service := &accountServiceStub{}
	handler := handlers.NewAccountHandler(service, limiter, false)
	response := httptest.NewRecorder()

	handler.ChangePassword(response, authenticatedRequest(http.MethodPost, "/me/password", `{}`))

	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "90" {
		t.Errorf("response = (%d, Retry-After %q), want (429, 90)", response.Code, response.Header().Get("Retry-After"))
	}
	if gotKey != accountTestUserID.String() {
		t.Errorf("rate limit key = %q, want the user ID", gotKey)
	}
	if service.gotChangeInput != nil {
		t.Error("service called despite the rate limit")
	}
}
