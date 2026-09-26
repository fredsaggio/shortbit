package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type loginAuthServiceStub struct {
	loginCalls int
}

func (s *loginAuthServiceStub) LoginWithPassword(context.Context, string, string, bool) (services.LoginResult, error) {
	s.loginCalls++
	return services.LoginResult{}, services.ErrIncorrectEmailOrPassword
}

func (s *loginAuthServiceStub) Logout(context.Context, string) error {
	return nil
}

type allowAllLoginRateLimiterStub struct{}

func (allowAllLoginRateLimiterStub) Allow(string) (bool, int) {
	return true, 0
}

type passwordResetServiceStub struct{}

func (passwordResetServiceStub) Start(context.Context, string, string) (services.PasswordResetStartResult, error) {
	return services.PasswordResetStartResult{Token: "test-token"}, nil
}

func (passwordResetServiceStub) Confirm(context.Context, string, string, string) error {
	return nil
}

func newPasswordResetTestHandler() *handlers.PasswordResetHandler {
	return handlers.NewPasswordResetHandler(passwordResetServiceStub{}, allowAllLoginRateLimiterStub{}, false)
}

func TestNewRouterHTTPRegistersPasswordResetRouteAndAppliesIPRateLimit(t *testing.T) {
	applicationHandlers := &Handlers{
		UserHandler:          &handlers.UserHandler{},
		SessionHandler:       &handlers.SessionHandler{},
		PasswordResetHandler: newPasswordResetTestHandler(),
		MeHandler:            http.NotFoundHandler(),
	}
	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewRateLimiter(1_000, 100, 10, time.Minute)
	srv.passwordResetStartLimiter = middleware.NewRateLimiter(0.001, 2, 10, time.Minute)
	router := srv.NewRouterHTTP()

	for requestNumber := 1; requestNumber <= 3; requestNumber++ {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/password-resets", strings.NewReader(`{"email":"user@example.com"}`))
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		wantStatus := http.StatusAccepted
		if requestNumber == 3 {
			wantStatus = http.StatusTooManyRequests
		}
		if response.Code != wantStatus {
			t.Errorf("request %d status = %d, want %d", requestNumber, response.Code, wantStatus)
		}
	}
}

func TestNewRouterHTTPRegistersPasswordResetConfirmRouteAndAppliesIPRateLimit(t *testing.T) {
	applicationHandlers := &Handlers{
		UserHandler: &handlers.UserHandler{}, SessionHandler: &handlers.SessionHandler{},
		PasswordResetHandler: newPasswordResetTestHandler(), MeHandler: http.NotFoundHandler(),
	}
	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewRateLimiter(1_000, 100, 10, time.Minute)
	srv.passwordResetConfirmLimiter = middleware.NewRateLimiter(0.001, 2, 10, time.Minute)
	router := srv.NewRouterHTTP()
	for requestNumber := 1; requestNumber <= 3; requestNumber++ {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/password-resets/confirm", strings.NewReader(`{"code":"12345678","password":"new-password","password_confirmation":"new-password"}`))
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		wantStatus := http.StatusGone // Missing attempt cookie reaches the confirm handler.
		if requestNumber == 3 {
			wantStatus = http.StatusTooManyRequests
		}
		if response.Code != wantStatus {
			t.Errorf("request %d status = %d, want %d", requestNumber, response.Code, wantStatus)
		}
	}
}

func TestNewRouterHTTPAppliesGlobalRateLimit(t *testing.T) {
	ctx := t.Context()
	applicationHandlers := &Handlers{
		UserHandler:          &handlers.UserHandler{},
		SessionHandler:       &handlers.SessionHandler{},
		PasswordResetHandler: newPasswordResetTestHandler(),
		MeHandler:            http.NotFoundHandler(),
	}

	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewRateLimiter(0.001, 2, 10, time.Minute)
	router := srv.NewRouterHTTP()

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/health/live", http.NoBody)
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("request %d status code = %d, want %d", requestNumber, response.Code, http.StatusOK)
		}
	}

	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/health/live", http.NoBody)
	request.RemoteAddr = "192.0.2.1:5678"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("request after burst status code = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}

func TestNewRouterHTTPAppliesLoginRateLimitOnlyToSessions(t *testing.T) {
	ctx := t.Context()
	authService := &loginAuthServiceStub{}
	applicationHandlers := &Handlers{
		UserHandler:          &handlers.UserHandler{},
		SessionHandler:       handlers.NewSessionHandler(authService, allowAllLoginRateLimiterStub{}, false),
		PasswordResetHandler: newPasswordResetTestHandler(),
		MeHandler:            http.NotFoundHandler(),
	}

	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewRateLimiter(1_000, 100, 10, time.Minute)
	srv.loginRateLimiter = middleware.NewRateLimiter(0.001, 5, 10, time.Minute)
	router := srv.NewRouterHTTP()

	for requestNumber := 1; requestNumber <= 5; requestNumber++ {
		request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("login request %d status code = %d, want %d", requestNumber, response.Code, http.StatusUnauthorized)
		}
	}

	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/sessions", strings.NewReader(`{"email":"user@example.com","password":"senha-segura"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.1:5678"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Errorf("login request after burst status code = %d, want %d", response.Code, http.StatusTooManyRequests)
	}

	if authService.loginCalls != 5 {
		t.Errorf("Login() calls = %d, want 5", authService.loginCalls)
	}

	healthRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/health/live", http.NoBody)
	healthRequest.RemoteAddr = "192.0.2.1:9999"
	healthResponse := httptest.NewRecorder()
	router.ServeHTTP(healthResponse, healthRequest)

	if healthResponse.Code != http.StatusOK {
		t.Errorf("health status code = %d, want %d", healthResponse.Code, http.StatusOK)
	}
}

func TestNewRouterHTTPAppliesPasswordRegistrationRateLimits(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		body          string
		allowedStatus int
		setLimiter    func(*Server, *middleware.RateLimiter)
	}{
		{
			name:          "start registration",
			path:          "/registrations/password",
			body:          `{`,
			allowedStatus: http.StatusBadRequest,
			setLimiter: func(srv *Server, limiter *middleware.RateLimiter) {
				srv.registrationStartLimiter = limiter
			},
		},
		{
			name:          "confirm registration",
			path:          "/registrations/password/confirm",
			body:          `{`,
			allowedStatus: http.StatusBadRequest,
			setLimiter: func(srv *Server, limiter *middleware.RateLimiter) {
				srv.registrationConfirmLimiter = limiter
			},
		},
		{
			name:          "resend registration code",
			path:          "/registrations/password/resend",
			allowedStatus: http.StatusGone,
			setLimiter: func(srv *Server, limiter *middleware.RateLimiter) {
				srv.registrationResendLimiter = limiter
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applicationHandlers := &Handlers{
				UserHandler:          &handlers.UserHandler{},
				SessionHandler:       &handlers.SessionHandler{},
				PasswordResetHandler: newPasswordResetTestHandler(),
				MeHandler:            http.NotFoundHandler(),
			}

			srv := NewServer(applicationHandlers, nil)
			srv.rateLimiter = middleware.NewRateLimiter(1_000, 100, 10, time.Minute)
			tt.setLimiter(srv, middleware.NewRateLimiter(0.001, 2, 10, time.Minute))
			router := srv.NewRouterHTTP()

			for requestNumber := 1; requestNumber <= 2; requestNumber++ {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tt.path, strings.NewReader(tt.body))
				request.RemoteAddr = "192.0.2.1:1234"
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)

				if response.Code != tt.allowedStatus {
					t.Fatalf("request %d status code = %d, want %d", requestNumber, response.Code, tt.allowedStatus)
				}
			}

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.RemoteAddr = "192.0.2.1:5678"
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusTooManyRequests {
				t.Errorf("request after burst status code = %d, want %d", response.Code, http.StatusTooManyRequests)
			}

			if response.Header().Get("Retry-After") == "" {
				t.Error("rate-limited response does not contain Retry-After")
			}
		})
	}
}

func TestPasswordRegistrationRateLimitersAreIndependent(t *testing.T) {
	applicationHandlers := &Handlers{
		UserHandler:          &handlers.UserHandler{},
		SessionHandler:       &handlers.SessionHandler{},
		PasswordResetHandler: newPasswordResetTestHandler(),
		MeHandler:            http.NotFoundHandler(),
	}

	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewRateLimiter(1_000, 100, 10, time.Minute)
	srv.registrationStartLimiter = middleware.NewRateLimiter(0.001, 1, 10, time.Minute)
	srv.registrationConfirmLimiter = middleware.NewRateLimiter(0.001, 1, 10, time.Minute)
	srv.registrationResendLimiter = middleware.NewRateLimiter(0.001, 1, 10, time.Minute)
	router := srv.NewRouterHTTP()

	requests := []struct {
		path          string
		body          string
		allowedStatus int
	}{
		{path: "/registrations/password", body: `{`, allowedStatus: http.StatusBadRequest},
		{path: "/registrations/password/confirm", body: `{`, allowedStatus: http.StatusBadRequest},
		{path: "/registrations/password/resend", allowedStatus: http.StatusGone},
	}

	for _, requestData := range requests {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, requestData.path, strings.NewReader(requestData.body))
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != requestData.allowedStatus {
			t.Fatalf("first request to %s status code = %d, want %d", requestData.path, response.Code, requestData.allowedStatus)
		}
	}

	for _, requestData := range requests {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, requestData.path, strings.NewReader(requestData.body))
		request.RemoteAddr = "192.0.2.1:5678"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusTooManyRequests {
			t.Errorf("second request to %s status code = %d, want %d", requestData.path, response.Code, http.StatusTooManyRequests)
		}
	}
}
