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

type linkAccessRateLimitServiceStub struct {
	createFunc func(context.Context, string, string) (services.LinkAccessSessionResult, error)
}

func (s linkAccessRateLimitServiceStub) CreateSession(ctx context.Context, code, password string) (services.LinkAccessSessionResult, error) {
	return s.createFunc(ctx, code, password)
}

func TestLinkAccessRouteHasRateLimitPerIPAndCode(t *testing.T) {
	serviceCalls := 0
	service := linkAccessRateLimitServiceStub{createFunc: func(context.Context, string, string) (services.LinkAccessSessionResult, error) {
		serviceCalls++
		return services.LinkAccessSessionResult{}, services.ErrIncorrectLinkPassword
	}}
	h := &Handlers{
		LinkAccessSessionHandler: handlers.NewLinkAccessSessionHandler(service, false),
		MeHandler:                http.NotFoundHandler(),
		CreateURLHandler:         http.NotFoundHandler(),
		ListURLHandler:           http.NotFoundHandler(),
		GetURLHandler:            http.NotFoundHandler(),
	}
	srv := NewServer(h, nil)
	srv.linkAccessLimiter = middleware.NewRateLimiter(0.001, 2, 10, time.Minute)
	router := srv.NewRouterHTTP()

	request := func(ip, code string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/"+code+"/access", strings.NewReader("password=wrong-password"))
		req.RemoteAddr = ip + ":1234"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if got := request("192.0.2.1", "Ab3dX9").Code; got != http.StatusUnauthorized {
			t.Errorf("attempt %d status = %d, want 401", attempt, got)
		}
	}
	blocked := request("192.0.2.1", "Ab3dX9")
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") == "" {
		t.Errorf("blocked request = status %d, Retry-After %q; want 429 with retry time", blocked.Code, blocked.Header().Get("Retry-After"))
	}
	if got := request("192.0.2.1", "Qz7Rt2").Code; got != http.StatusUnauthorized {
		t.Errorf("other link status = %d, want 401", got)
	}
	if got := request("192.0.2.2", "Ab3dX9").Code; got != http.StatusUnauthorized {
		t.Errorf("other IP status = %d, want 401", got)
	}
	if serviceCalls != 4 {
		t.Errorf("service calls = %d, want 4; blocked request must not reach password verification", serviceCalls)
	}
}
