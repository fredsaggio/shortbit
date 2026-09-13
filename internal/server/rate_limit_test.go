package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
)

func TestNewRouterHTTPAppliesGlobalRateLimit(t *testing.T) {
	ctx := t.Context()
	applicationHandlers := &Handlers{
		UserHandler:    &handlers.UserHandler{},
		SessionHandler: &handlers.SessionHandler{},
		MeHandler:      http.NotFoundHandler(),
	}

	srv := NewServer(applicationHandlers, nil)
	srv.rateLimiter = middleware.NewIPRateLimiter(0.001, 2, 10, time.Minute)
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
