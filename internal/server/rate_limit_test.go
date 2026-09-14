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

func (s *loginAuthServiceStub) LoginWithPassword(context.Context, string, string) (services.LoginResult, error) {
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

func TestNewRouterHTTPAppliesGlobalRateLimit(t *testing.T) {
	ctx := t.Context()
	applicationHandlers := &Handlers{
		UserHandler:    &handlers.UserHandler{},
		SessionHandler: &handlers.SessionHandler{},
		MeHandler:      http.NotFoundHandler(),
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
		UserHandler:    &handlers.UserHandler{},
		SessionHandler: handlers.NewSessionHandler(authService, allowAllLoginRateLimiterStub{}, false),
		MeHandler:      http.NotFoundHandler(),
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
