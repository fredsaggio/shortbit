package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type userServiceStub struct {
	registerWithPasswordFunc func(
		ctx context.Context,
		email string,
		password string,
	) (models.User, error)
	getByIDFunc func(ctx context.Context, userID uuid.UUID) (models.User, error)
}

func (s userServiceStub) RegisterWithPassword(
	ctx context.Context,
	email string,
	password string,
) (models.User, error) {
	return s.registerWithPasswordFunc(ctx, email, password)
}

func (s userServiceStub) GetByID(ctx context.Context, userID uuid.UUID) (models.User, error) {
	return s.getByIDFunc(ctx, userID)
}

func TestUserHandlerCreateWithPassword(t *testing.T) {
	wantUser := models.User{
		ID:    uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"),
		Email: "user@example.com",
	}

	service := userServiceStub{
		registerWithPasswordFunc: func(
			ctx context.Context,
			email string,
			password string,
		) (models.User, error) {
			if ctx == nil {
				t.Fatal("RegisterWithPassword() received a nil context")
			}

			if email != "USER@example.com" {
				t.Errorf("RegisterWithPassword() email = %q, want %q", email, "USER@example.com")
			}

			if password != "senha-segura" {
				t.Errorf("RegisterWithPassword() password = %q, want %q", password, "senha-segura")
			}

			return wantUser, nil
		},
	}

	handler := handlers.NewUserHandler(service)
	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		strings.NewReader(`{"email":"USER@example.com","password":"senha-segura"}`),
	)
	response := httptest.NewRecorder()

	handler.RegisterWithPassword(response, request)

	result := response.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusCreated {
		t.Fatalf("status code = %d, want %d", result.StatusCode, http.StatusCreated)
	}

	if got := result.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	responseBody := response.Body.String()

	var gotResponse struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(strings.NewReader(responseBody)).Decode(&gotResponse); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if gotResponse.ID != wantUser.ID.String() {
		t.Errorf("response ID = %q, want %q", gotResponse.ID, wantUser.ID.String())
	}

	if gotResponse.Email != wantUser.Email {
		t.Errorf("response email = %q, want %q", gotResponse.Email, wantUser.Email)
	}

	if strings.Contains(responseBody, "password") || strings.Contains(responseBody, "hash") {
		t.Errorf("response body exposes credential data: %s", responseBody)
	}
}

func TestUserHandlerCreateWithPasswordRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty body",
			body: "",
		},
		{
			name: "malformed JSON",
			body: `{"email":`,
		},
		{
			name: "unknown field",
			body: `{"email":"user@example.com","password":"senha-segura","name":"Fred"}`,
		},
		{
			name: "multiple JSON objects",
			body: `{"email":"user@example.com","password":"senha-segura"} {"email":"other@example.com"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{
				registerWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
					t.Fatal("RegisterWithPassword() should not be called")
					return models.User{}, nil
				},
			}

			handler := handlers.NewUserHandler(service)
			request := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			handler.RegisterWithPassword(response, request)

			if response.Code != http.StatusBadRequest {
				t.Errorf("status code = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestUserHandlerRegisterWithPasswordMapsServiceErrors(t *testing.T) {
	unexpectedErr := errors.New("database unavailable")

	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "invalid email",
			serviceErr: services.ErrInvalidEmail,
			wantStatus: http.StatusBadRequest,
			wantBody:   "email inválido\n",
		},
		{
			name:       "password too short",
			serviceErr: services.ErrPasswordTooShort,
			wantStatus: http.StatusBadRequest,
			wantBody:   "senha muito curta\n",
		},
		{
			name:       "password too long",
			serviceErr: services.ErrPasswordTooLong,
			wantStatus: http.StatusBadRequest,
			wantBody:   "senha muito longa\n",
		},
		{
			name:       "email already exists",
			serviceErr: services.ErrEmailAlreadyExists,
			wantStatus: http.StatusConflict,
			wantBody:   "email já está em uso\n",
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
			service := userServiceStub{
				registerWithPasswordFunc: func(context.Context, string, string) (models.User, error) {
					return models.User{}, tt.serviceErr
				},
			}

			handler := handlers.NewUserHandler(service)
			request := httptest.NewRequest(
				http.MethodPost,
				"/users",
				strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`),
			)
			response := httptest.NewRecorder()

			handler.RegisterWithPassword(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", response.Code, tt.wantStatus)
			}

			if response.Body.String() != tt.wantBody {
				t.Errorf("response body = %q, want %q", response.Body.String(), tt.wantBody)
			}

			if errors.Is(tt.serviceErr, unexpectedErr) && strings.Contains(response.Body.String(), unexpectedErr.Error()) {
				t.Error("response body exposes an internal error")
			}
		})
	}
}

func TestUserHandlerGetUserInfo(t *testing.T) {
	wantUser := models.User{
		ID:    uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c317"),
		Email: "user@example.com",
	}

	service := userServiceStub{
		getByIDFunc: func(ctx context.Context, userID uuid.UUID) (models.User, error) {
			if ctx == nil {
				t.Fatal("GetByID() received a nil context")
			}
			if userID != wantUser.ID {
				t.Errorf("GetByID() user ID = %s, want %s", userID, wantUser.ID)
			}
			return wantUser, nil
		},
	}

	handler := handlers.NewUserHandler(service)
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
	service := userServiceStub{
		getByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
			serviceCalled = true
			return models.User{}, errors.New("unexpected GetByID call")
		},
	}

	handler := handlers.NewUserHandler(service)
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
	service := userServiceStub{
		getByIDFunc: func(context.Context, uuid.UUID) (models.User, error) {
			return models.User{}, wantErr
		},
	}

	handler := handlers.NewUserHandler(service)
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
