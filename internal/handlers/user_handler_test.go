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
	startPasswordRegistrationFunc func(context.Context, string, string) (services.PasswordRegistrationResult, error)
	getByIDFunc                   func(context.Context, uuid.UUID) (models.User, error)
}

func (s userServiceStub) StartPasswordRegistration(ctx context.Context, email, password string) (services.PasswordRegistrationResult, error) {
	return s.startPasswordRegistrationFunc(ctx, email, password)
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

	handler := handlers.NewUserHandler(service, true)
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
	handler := handlers.NewUserHandler(service, false)
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
			handler := handlers.NewUserHandler(service, false)
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
			handler := handlers.NewUserHandler(service, false)
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

	handler := handlers.NewUserHandler(service, false)
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
	handler := handlers.NewUserHandler(service, false)
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
	handler := handlers.NewUserHandler(service, false)
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
