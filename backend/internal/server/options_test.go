package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
)

func newOptionsTestHandlers() *Handlers {
	return &Handlers{
		UserHandler:           &handlers.UserHandler{},
		SessionHandler:        &handlers.SessionHandler{},
		PasswordResetHandler:  newPasswordResetTestHandler(),
		CreateURLHandler:      http.NotFoundHandler(),
		ListURLHandler:        http.NotFoundHandler(),
		GetURLHandler:         http.NotFoundHandler(),
		MeHandler:             http.NotFoundHandler(),
		UpdateMeHandler:       http.NotFoundHandler(),
		ChangePasswordHandler: http.NotFoundHandler(),
		UpdateURLHandler:      http.NotFoundHandler(),
		DeleteURLHandler:      http.NotFoundHandler(), DeleteMeHandler: http.NotFoundHandler(), ClickStatsHandler: http.NotFoundHandler(),
	}
}

type frontendStub struct{}

func (frontendStub) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /painel", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("app"))
	})
}

func TestWithFrontendRegistersAppRoutes(t *testing.T) {
	router := NewServer(newOptionsTestHandlers(), nil, WithFrontend(frontendStub{})).NewRouterHTTP()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/painel", http.NoBody)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "app" {
		t.Fatalf("GET /painel status = %d body = %q, want the app", response.Code, response.Body.String())
	}
}

func TestWithClientIPHeaderSeparatesClientsBehindAProxy(t *testing.T) {
	srv := NewServer(newOptionsTestHandlers(), nil, WithClientIPHeader("True-Client-Ip"))
	srv.rateLimiter = middleware.NewRateLimiter(0.001, 1, 10, time.Minute)
	router := srv.NewRouterHTTP()

	send := func(client string) int {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health/live", http.NoBody)
		request.RemoteAddr = "10.0.0.2:443"
		request.Header.Set("True-Client-IP", client)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Code
	}

	if code := send("198.51.100.1"); code != http.StatusOK {
		t.Fatalf("first client status = %d, want %d", code, http.StatusOK)
	}
	if code := send("198.51.100.2"); code != http.StatusOK {
		t.Fatalf("second client behind the same proxy status = %d, want %d", code, http.StatusOK)
	}
	if code := send("198.51.100.1"); code != http.StatusTooManyRequests {
		t.Fatalf("first client again status = %d, want %d", code, http.StatusTooManyRequests)
	}
}

func TestWithHSTSSendsTheHeaderOnEveryResponse(t *testing.T) {
	for _, tt := range []struct {
		name    string
		options []Option
		want    string
	}{
		{name: "behind HTTPS", options: []Option{WithHSTS()}, want: "max-age=31536000; includeSubDomains"},
		{name: "local HTTP", options: nil, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := NewServer(newOptionsTestHandlers(), nil, tt.options...).NewRouterHTTP()

			for _, path := range []string{"/health/live", "/nao-existe/de-jeito-nenhum"} {
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)

				if got := response.Header().Get("Strict-Transport-Security"); got != tt.want {
					t.Errorf("GET %s Strict-Transport-Security = %q, want %q", path, got, tt.want)
				}
			}
		})
	}
}

func TestRouterSendsNoSniffOnEveryResponse(t *testing.T) {
	router := NewServer(newOptionsTestHandlers(), nil).NewRouterHTTP()

	for _, path := range []string{"/health/live", "/nao-existe/de-jeito-nenhum"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET %s X-Content-Type-Options = %q, want nosniff", path, got)
		}
	}
}
