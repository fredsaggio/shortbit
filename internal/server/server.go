package server

import (
	"context"
	"net/http"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
)

const (
	readinessTimeout                 = 2 * time.Second
	globalRateLimitRequestsPerSecond = 10
	globalRateLimitBurst             = 20
	globalRateLimitMaxClients        = 10_000
	globalRateLimitStaleAfter        = 10 * time.Minute

	loginRateLimitRequestsPerSecond = 20.0 / 60.0
	loginRateLimitBurst             = 5
	loginRateLimitMaxClients        = 10_000
	loginRateLimitStaleAfter        = 10 * time.Minute
)

type DatabasePinger interface {
	Ping(ctx context.Context) error
}

type Handlers struct {
	UserHandler    *handlers.UserHandler
	SessionHandler *handlers.SessionHandler
	MeHandler      http.Handler
}

type Server struct {
	h                *Handlers
	database         DatabasePinger
	rateLimiter      *middleware.RateLimiter
	loginRateLimiter *middleware.RateLimiter
}

func NewServer(h *Handlers, database DatabasePinger) *Server {
	return &Server{
		h:                h,
		database:         database,
		rateLimiter:      middleware.NewRateLimiter(globalRateLimitRequestsPerSecond, globalRateLimitBurst, globalRateLimitMaxClients, globalRateLimitStaleAfter),
		loginRateLimiter: middleware.NewRateLimiter(loginRateLimitRequestsPerSecond, loginRateLimitBurst, loginRateLimitMaxClients, loginRateLimitStaleAfter),
	}
}

func (srv *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := srv.database.Ping(ctx); err != nil {
			http.Error(
				w,
				"serviço indisponível",
				http.StatusServiceUnavailable,
			)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /registrations/password", srv.h.UserHandler.StartPasswordRegistration)
	mux.HandleFunc("POST /registrations/password/confirm", srv.h.UserHandler.ConfirmPasswordRegistration)
	mux.HandleFunc("POST /registrations/password/resend", srv.h.UserHandler.ResendPasswordRegistrationCode)

	mux.Handle("GET /me", srv.h.MeHandler)

	loginHandler := srv.loginRateLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.SessionHandler.LoginWithPassword))
	mux.Handle("POST /sessions", loginHandler)
	mux.HandleFunc("DELETE /sessions/current", srv.h.SessionHandler.Logout)

}

func (srv *Server) NewRouterHTTP() http.Handler {
	mux := http.NewServeMux()

	srv.registerRoutes(mux)

	handler := http.Handler(mux)

	handler = middleware.LimitRequestBody(handler)
	handler = srv.rateLimiter.MiddlewareByIP(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.AccessLog(handler)
	handler = middleware.RequestID(handler)

	return handler
}
