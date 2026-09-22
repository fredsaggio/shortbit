package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
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

type userServiceStub struct {
	startPasswordRegistrationFunc      func(context.Context, string, string) (services.PasswordRegistrationResult, error)
	confirmPasswordRegistrationFunc    func(context.Context, string, string) (models.User, error)
	resendPasswordRegistrationCodeFunc func(context.Context, string) error
	getByIDFunc                        func(context.Context, uuid.UUID) (models.User, error)
}

type allowAllRegistrationRateLimiterStub struct{}

func (allowAllRegistrationRateLimiterStub) Allow(string) (bool, int) {
	return true, 0
}

type registrationRateLimiterStub struct {
	allowFunc func(string) (bool, int)
}

func (s registrationRateLimiterStub) Allow(key string) (bool, int) {
	return s.allowFunc(key)
}

func (s userServiceStub) StartPasswordRegistration(ctx context.Context, email, password string) (services.PasswordRegistrationResult, error) {
	return s.startPasswordRegistrationFunc(ctx, email, password)
}

func (s userServiceStub) ConfirmPasswordRegistration(ctx context.Context, token, code string) (models.User, error) {
	return s.confirmPasswordRegistrationFunc(ctx, token, code)
}

func (s userServiceStub) ResendPasswordRegistrationCode(ctx context.Context, token string) error {
	return s.resendPasswordRegistrationCodeFunc(ctx, token)
}

func (s userServiceStub) GetByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	return s.getByIDFunc(ctx, userID)
}

func TestUserHandlerStartPasswordRegistration(t *testing.T) {
	const (
		token    = "password-registration-token"
		email    = "USER@example.com"
		password = "senha-segura"
	)
	expiresAt := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)

	service := userServiceStub{startPasswordRegistrationFunc: func(ctx context.Context, gotEmail, gotPassword string) (services.PasswordRegistrationResult, error) {
		if ctx == nil {
			t.Fatal("StartPasswordRegistration() received a nil context")
		}
		if gotEmail != email {
			t.Errorf("StartPasswordRegistration() email = %q, want %q", gotEmail, email)
		}
		if gotPassword != password {
			t.Errorf("StartPasswordRegistration() password = %q, want %q", gotPassword, password)
		}
		return services.PasswordRegistrationResult{Token: token, ExpiresAt: expiresAt}, nil
	}}

	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	request := httptest.NewRequest(http.MethodPost, "/registrations/password", strings.NewReader(`{"email":"USER@example.com","password":"senha-segura"}`))
	response := httptest.NewRecorder()
	handler.StartPasswordRegistration(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", result.StatusCode, http.StatusAccepted)
	}
	if response.Body.Len() != 0 {
		t.Errorf("response body = %q, want empty", response.Body.String())
	}

	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "password_registration" {
		t.Errorf("cookie name = %q, want %q", cookie.Name, "password_registration")
	}
	if cookie.Value != token {
		t.Errorf("cookie value = %q, want %q", cookie.Value, token)
	}
	if cookie.Path != "/registrations/password" {
		t.Errorf("cookie path = %q, want %q", cookie.Path, "/registrations/password")
	}
	if !cookie.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}
	if !cookie.Secure {
		t.Error("cookie Secure = false, want true")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
	if !cookie.Expires.Equal(expiresAt) {
		t.Errorf("cookie expiration = %v, want %v", cookie.Expires, expiresAt)
	}
	if cookie.MaxAge <= 0 || cookie.MaxAge > int((30*time.Minute).Seconds()) {
		t.Errorf("cookie MaxAge = %d, want between 1 and 1800", cookie.MaxAge)
	}
}

func TestUserHandlerStartPasswordRegistrationHonorsInsecureCookieConfiguration(t *testing.T) {
	service := userServiceStub{startPasswordRegistrationFunc: func(context.Context, string, string) (services.PasswordRegistrationResult, error) {
		return services.PasswordRegistrationResult{Token: "registration-token", ExpiresAt: time.Now().UTC().Add(30 * time.Minute)}, nil
	}}
	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	response := httptest.NewRecorder()
	handler.StartPasswordRegistration(response, httptest.NewRequest(http.MethodPost, "/registrations/password", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`)))

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	if cookies[0].Secure {
		t.Error("cookie Secure = true, want false")
	}
}

func TestUserHandlerStartPasswordRegistrationRateLimitsNormalizedEmail(t *testing.T) {
	const retryAfter = 37

	service := userServiceStub{startPasswordRegistrationFunc: func(context.Context, string, string) (services.PasswordRegistrationResult, error) {
		t.Fatal("StartPasswordRegistration() should not be called after rate limit")
		return services.PasswordRegistrationResult{}, nil
	}}

	limiter := registrationRateLimiterStub{allowFunc: func(key string) (bool, int) {
		if key != "user@example.com" {
			t.Errorf("rate limit key = %q, want %q", key, "user@example.com")
		}
		return false, retryAfter
	}}

	handler := handlers.NewUserHandler(service, limiter, false)
	request := httptest.NewRequest(http.MethodPost, "/registrations/password", strings.NewReader(`{"email":"  USER@Example.COM  ","password":"senha-segura"}`))
	response := httptest.NewRecorder()
	handler.StartPasswordRegistration(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response.Header().Get("Retry-After") != "37" {
		t.Errorf("Retry-After = %q, want %q", response.Header().Get("Retry-After"), "37")
	}
	if response.Body.String() != "muitas solicitações para este email, tente novamente mais tarde\n" {
		t.Errorf("response body = %q", response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Error("rate-limited registration created a cookie")
	}
}

func TestUserHandlerStartPasswordRegistrationRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed JSON", body: `{"email":`},
		{name: "unknown field", body: `{"email":"user@example.com","password":"senha-segura","name":"Fred"}`},
		{name: "multiple JSON objects", body: `{"email":"user@example.com","password":"senha-segura"} {"email":"other@example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{startPasswordRegistrationFunc: func(context.Context, string, string) (services.PasswordRegistrationResult, error) {
				t.Fatal("StartPasswordRegistration() should not be called")
				return services.PasswordRegistrationResult{}, nil
			}}
			handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			response := httptest.NewRecorder()
			handler.StartPasswordRegistration(response, httptest.NewRequest(http.MethodPost, "/registrations/password", strings.NewReader(tt.body)))

			if response.Code != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Error("invalid request created a registration cookie")
			}
		})
	}
}

func TestUserHandlerStartPasswordRegistrationMapsServiceErrors(t *testing.T) {
	unexpectedErr := errors.New("database unavailable")
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{name: "invalid email", serviceErr: services.ErrInvalidEmail, wantStatus: http.StatusBadRequest, wantBody: "email inválido\n"},
		{name: "password too short", serviceErr: services.ErrPasswordTooShort, wantStatus: http.StatusBadRequest, wantBody: "senha muito curta\n"},
		{name: "password too long", serviceErr: services.ErrPasswordTooLong, wantStatus: http.StatusBadRequest, wantBody: "senha muito longa\n"},
		{name: "email already exists", serviceErr: services.ErrEmailAlreadyExists, wantStatus: http.StatusConflict, wantBody: "email já está em uso\n"},
		{name: "unexpected error", serviceErr: unexpectedErr, wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{startPasswordRegistrationFunc: func(context.Context, string, string) (services.PasswordRegistrationResult, error) {
				return services.PasswordRegistrationResult{}, tt.serviceErr
			}}
			handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			request := httptest.NewRequest(http.MethodPost, "/registrations/password", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
			response := httptest.NewRecorder()
			handler.StartPasswordRegistration(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Body.String() != tt.wantBody {
				t.Errorf("response body = %q, want %q", response.Body.String(), tt.wantBody)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Error("failed registration created a cookie")
			}
			if errors.Is(tt.serviceErr, unexpectedErr) && strings.Contains(response.Body.String(), unexpectedErr.Error()) {
				t.Error("response body exposes an internal error")
			}
		})
	}
}

func TestUserHandlerConfirmPasswordRegistration(t *testing.T) {
	const (
		token = "password-registration-token"
		code  = "123456"
	)
	wantUser := models.User{
		ID:    uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c318"),
		Email: "user@example.com",
	}

	service := userServiceStub{confirmPasswordRegistrationFunc: func(ctx context.Context, gotToken, gotCode string) (models.User, error) {
		if ctx == nil {
			t.Fatal("ConfirmPasswordRegistration() received a nil context")
		}
		if gotToken != token {
			t.Errorf("ConfirmPasswordRegistration() token = %q, want %q", gotToken, token)
		}
		if gotCode != code {
			t.Errorf("ConfirmPasswordRegistration() code = %q, want %q", gotCode, code)
		}
		return wantUser, nil
	}}

	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	request := httptest.NewRequest(http.MethodPost, "/registrations/password/confirm", strings.NewReader(`{"code":"123456"}`))
	request.AddCookie(&http.Cookie{Name: "password_registration", Value: token, Path: "/registrations/password"})
	response := httptest.NewRecorder()
	handler.ConfirmPasswordRegistration(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusCreated {
		t.Fatalf("status code = %d, want %d; body = %q", result.StatusCode, http.StatusCreated, response.Body.String())
	}
	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var gotUser struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(result.Body).Decode(&gotUser); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if gotUser.ID != wantUser.ID.String() {
		t.Errorf("response ID = %q, want %q", gotUser.ID, wantUser.ID.String())
	}
	if gotUser.Email != wantUser.Email {
		t.Errorf("response email = %q, want %q", gotUser.Email, wantUser.Email)
	}

	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	assertClearedPasswordRegistrationCookie(t, cookies[0], true)
}

func TestUserHandlerConfirmPasswordRegistrationRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed JSON", body: `{"code":`},
		{name: "unknown field", body: `{"code":"123456","email":"user@example.com"}`},
		{name: "multiple JSON objects", body: `{"code":"123456"} {"code":"654321"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{confirmPasswordRegistrationFunc: func(context.Context, string, string) (models.User, error) {
				t.Fatal("ConfirmPasswordRegistration() should not be called")
				return models.User{}, nil
			}}
			handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			request := httptest.NewRequest(http.MethodPost, "/registrations/password/confirm", strings.NewReader(tt.body))
			request.AddCookie(&http.Cookie{Name: "password_registration", Value: "registration-token"})
			response := httptest.NewRecorder()
			handler.ConfirmPasswordRegistration(response, request)

			if response.Code != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if len(response.Result().Cookies()) != 0 {
				t.Error("invalid JSON changed the registration cookie")
			}
		})
	}
}

func TestUserHandlerConfirmPasswordRegistrationRejectsMissingCookie(t *testing.T) {
	service := userServiceStub{confirmPasswordRegistrationFunc: func(context.Context, string, string) (models.User, error) {
		t.Fatal("ConfirmPasswordRegistration() should not be called")
		return models.User{}, nil
	}}
	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	response := httptest.NewRecorder()
	handler.ConfirmPasswordRegistration(response, httptest.NewRequest(http.MethodPost, "/registrations/password/confirm", strings.NewReader(`{"code":"123456"}`)))

	if response.Code != http.StatusGone {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusGone)
	}
	if response.Body.String() != "tentativa de cadastro ausente ou expirada\n" {
		t.Errorf("response body = %q, want %q", response.Body.String(), "tentativa de cadastro ausente ou expirada\n")
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	assertClearedPasswordRegistrationCookie(t, cookies[0], false)
}

func TestUserHandlerConfirmPasswordRegistrationMapsServiceErrors(t *testing.T) {
	unexpectedErr := errors.New("database unavailable")
	tests := []struct {
		name         string
		serviceErr   error
		wantStatus   int
		wantBody     string
		clearsCookie bool
	}{
		{name: "attempt unavailable", serviceErr: services.ErrRegistrationAttemptUnavailable, wantStatus: http.StatusGone, wantBody: "tentativa de cadastro ausente ou expirada\n", clearsCookie: true},
		{name: "attempt locked", serviceErr: services.ErrRegistrationAttemptLocked, wantStatus: http.StatusTooManyRequests, wantBody: "aguarde antes de tentar novamente\n"},
		{name: "verification code expired", serviceErr: services.ErrVerificationCodeExpired, wantStatus: http.StatusBadRequest, wantBody: "código expirado; solicite um novo código\n"},
		{name: "verification code invalid", serviceErr: services.ErrVerificationCodeInvalid, wantStatus: http.StatusBadRequest, wantBody: "código de confirmação inválido\n"},
		{name: "email already exists", serviceErr: services.ErrEmailAlreadyExists, wantStatus: http.StatusConflict, wantBody: "email já está em uso; faça login\n", clearsCookie: true},
		{name: "unexpected error", serviceErr: unexpectedErr, wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{confirmPasswordRegistrationFunc: func(context.Context, string, string) (models.User, error) {
				return models.User{}, tt.serviceErr
			}}
			handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			request := httptest.NewRequest(http.MethodPost, "/registrations/password/confirm", strings.NewReader(`{"code":"123456"}`))
			request.AddCookie(&http.Cookie{Name: "password_registration", Value: "registration-token"})
			response := httptest.NewRecorder()
			handler.ConfirmPasswordRegistration(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Body.String() != tt.wantBody {
				t.Errorf("response body = %q, want %q", response.Body.String(), tt.wantBody)
			}

			cookies := response.Result().Cookies()
			if tt.clearsCookie {
				if len(cookies) != 1 {
					t.Fatalf("response cookies = %d, want 1", len(cookies))
				}
				assertClearedPasswordRegistrationCookie(t, cookies[0], false)
			} else if len(cookies) != 0 {
				t.Error("recoverable confirmation error changed the registration cookie")
			}

			if errors.Is(tt.serviceErr, unexpectedErr) && strings.Contains(response.Body.String(), unexpectedErr.Error()) {
				t.Error("response body exposes an internal error")
			}
		})
	}
}

func TestUserHandlerResendPasswordRegistrationCode(t *testing.T) {
	const token = "password-registration-token"

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	service := userServiceStub{resendPasswordRegistrationCodeFunc: func(gotCtx context.Context, gotToken string) error {
		if got := gotCtx.Value(contextKey{}); got != "request-context" {
			t.Errorf("ResendPasswordRegistrationCode() context value = %v, want %q", got, "request-context")
		}
		if gotToken != token {
			t.Errorf("ResendPasswordRegistrationCode() token = %q, want %q", gotToken, token)
		}
		return nil
	}}

	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	request := httptest.NewRequest(http.MethodPost, "/registrations/password/resend", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "password_registration", Value: token, Path: "/registrations/password"})
	response := httptest.NewRecorder()

	handler.ResendPasswordRegistrationCode(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", result.StatusCode, http.StatusAccepted)
	}
	if response.Body.Len() != 0 {
		t.Errorf("response body = %q, want empty", response.Body.String())
	}
	if cookies := result.Cookies(); len(cookies) != 0 {
		t.Errorf("response cookies = %d, want 0", len(cookies))
	}
}

func TestUserHandlerResendPasswordRegistrationCodeRejectsMissingCookie(t *testing.T) {
	serviceCalled := false
	service := userServiceStub{resendPasswordRegistrationCodeFunc: func(context.Context, string) error {
		serviceCalled = true
		return nil
	}}
	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, true)
	response := httptest.NewRecorder()

	handler.ResendPasswordRegistrationCode(
		response,
		httptest.NewRequest(http.MethodPost, "/registrations/password/resend", nil),
	)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusGone {
		t.Errorf("status code = %d, want %d", result.StatusCode, http.StatusGone)
	}
	if serviceCalled {
		t.Error("ResendPasswordRegistrationCode() was called without a registration cookie")
	}

	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response cookies = %d, want 1", len(cookies))
	}
	assertClearedPasswordRegistrationCookie(t, cookies[0], true)
}

func TestUserHandlerResendPasswordRegistrationCodeMapsServiceErrors(t *testing.T) {
	const token = "password-registration-token"
	wantInternalErr := errors.New("email provider unavailable")

	tests := []struct {
		name         string
		serviceErr   error
		wantStatus   int
		wantMessage  string
		clearsCookie bool
	}{
		{
			name:         "unavailable attempt",
			serviceErr:   services.ErrRegistrationAttemptUnavailable,
			wantStatus:   http.StatusGone,
			wantMessage:  "tentativa de cadastro ausente ou expirada",
			clearsCookie: true,
		},
		{
			name:        "locked attempt",
			serviceErr:  services.ErrRegistrationAttemptLocked,
			wantStatus:  http.StatusTooManyRequests,
			wantMessage: "aguarde antes de solicitar outro código",
		},
		{
			name:        "resend cooldown",
			serviceErr:  services.ErrRegistrationCodeResendTooSoon,
			wantStatus:  http.StatusTooManyRequests,
			wantMessage: "aguarde 30 segundos antes de solicitar outro código",
		},
		{
			name:        "internal error",
			serviceErr:  wantInternalErr,
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "erro interno do servidor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{resendPasswordRegistrationCodeFunc: func(_ context.Context, gotToken string) error {
				if gotToken != token {
					t.Errorf("ResendPasswordRegistrationCode() token = %q, want %q", gotToken, token)
				}
				return tt.serviceErr
			}}
			handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
			request := httptest.NewRequest(http.MethodPost, "/registrations/password/resend", nil)
			request.AddCookie(&http.Cookie{Name: "password_registration", Value: token, Path: "/registrations/password"})
			response := httptest.NewRecorder()

			handler.ResendPasswordRegistrationCode(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != tt.wantStatus {
				t.Errorf("status code = %d, want %d", result.StatusCode, tt.wantStatus)
			}
			if !strings.Contains(response.Body.String(), tt.wantMessage) {
				t.Errorf("response body = %q, want message containing %q", response.Body.String(), tt.wantMessage)
			}
			if strings.Contains(response.Body.String(), wantInternalErr.Error()) {
				t.Error("response body exposes an internal error")
			}

			cookies := result.Cookies()
			if tt.clearsCookie {
				if len(cookies) != 1 {
					t.Fatalf("response cookies = %d, want 1", len(cookies))
				}
				assertClearedPasswordRegistrationCookie(t, cookies[0], false)
			} else if len(cookies) != 0 {
				t.Errorf("response cookies = %d, want 0", len(cookies))
			}
		})
	}
}

func TestUserHandlerGetUserInfo(t *testing.T) {
	wantUser := models.User{ID: uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"), Email: "user@example.com"}
	service := userServiceStub{getByIDFunc: func(ctx context.Context, userID uuid.UUID) (models.User, error) {
		if ctx == nil {
			t.Fatal("GetByID() received a nil context")
		}
		if userID != wantUser.ID {
			t.Errorf("GetByID() user ID = %s, want %s", userID, wantUser.ID)
		}
		return wantUser, nil
	}}

	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request = request.WithContext(ctxval.ContextWithUserID(request.Context(), wantUser.ID))
	response := httptest.NewRecorder()
	handler.GetUserInfo(response, request)

	result := response.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", result.StatusCode, http.StatusOK)
	}
	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	var gotResponse struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(result.Body).Decode(&gotResponse); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if gotResponse.ID != wantUser.ID.String() {
		t.Errorf("response ID = %q, want %q", gotResponse.ID, wantUser.ID.String())
	}
	if gotResponse.Email != wantUser.Email {
		t.Errorf("response email = %q, want %q", gotResponse.Email, wantUser.Email)
	}
}

func TestUserHandlerGetUserInfoRejectsMissingUserID(t *testing.T) {
	serviceCalled := false
	service := userServiceStub{getByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
		serviceCalled = true
		return models.User{}, errors.New("unexpected GetByID call")
	}}
	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	response := httptest.NewRecorder()
	handler.GetUserInfo(response, httptest.NewRequest(http.MethodGet, "/me", nil))

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if serviceCalled {
		t.Error("GetByID() was called without a user ID in the request context")
	}
}

func TestUserHandlerGetUserInfoHandlesServiceError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317")
	service := userServiceStub{getByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
		return models.User{}, wantErr
	}}
	handler := handlers.NewUserHandler(service, allowAllRegistrationRateLimiterStub{}, false)
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request = request.WithContext(ctxval.ContextWithUserID(request.Context(), userID))
	response := httptest.NewRecorder()
	handler.GetUserInfo(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(response.Body.String(), wantErr.Error()) {
		t.Error("response body exposes an internal error")
	}
}

func assertClearedPasswordRegistrationCookie(t *testing.T, cookie *http.Cookie, secure bool) {
	t.Helper()

	if cookie.Name != "password_registration" {
		t.Errorf("cookie name = %q, want %q", cookie.Name, "password_registration")
	}
	if cookie.Value != "" {
		t.Errorf("cookie value = %q, want empty", cookie.Value)
	}
	if cookie.Path != "/registrations/password" {
		t.Errorf("cookie path = %q, want %q", cookie.Path, "/registrations/password")
	}
	if cookie.MaxAge != -1 {
		t.Errorf("cookie MaxAge = %d, want -1", cookie.MaxAge)
	}
	if !cookie.Expires.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("cookie expiration = %v, want %v", cookie.Expires, time.Unix(0, 0).UTC())
	}
	if !cookie.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}
	if cookie.Secure != secure {
		t.Errorf("cookie Secure = %t, want %t", cookie.Secure, secure)
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want %v", cookie.SameSite, http.SameSiteStrictMode)
	}
}
