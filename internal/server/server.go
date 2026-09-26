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

	registrationStartRateLimitRequestsPerSecond = 5.0 / 60.0
	registrationStartRateLimitBurst             = 3

	registrationConfirmRateLimitRequestsPerSecond = 10.0 / 60.0
	registrationConfirmRateLimitBurst             = 5

	registrationResendRateLimitRequestsPerSecond = 5.0 / 60.0
	registrationResendRateLimitBurst             = 3

	registrationRateLimitMaxClients = 10_000
	registrationRateLimitStaleAfter = 10 * time.Minute

	passwordResetStartRateLimitRequestsPerSecond   = 5.0 / 60.0
	passwordResetStartRateLimitBurst               = 3
	passwordResetConfirmRateLimitRequestsPerSecond = 10.0 / 60.0
	passwordResetConfirmRateLimitBurst             = 5
)

type DatabasePinger interface {
	Ping(ctx context.Context) error
}

type Handlers struct {
	UserHandler          *handlers.UserHandler
	SessionHandler       *handlers.SessionHandler
	GoogleAuthHandler    *handlers.GoogleAuthHandler
	PasswordResetHandler *handlers.PasswordResetHandler
	MeHandler            http.Handler
}

type Server struct {
	h                           *Handlers
	database                    DatabasePinger
	rateLimiter                 *middleware.RateLimiter
	loginRateLimiter            *middleware.RateLimiter
	registrationStartLimiter    *middleware.RateLimiter
	registrationConfirmLimiter  *middleware.RateLimiter
	registrationResendLimiter   *middleware.RateLimiter
	passwordResetStartLimiter   *middleware.RateLimiter
	passwordResetConfirmLimiter *middleware.RateLimiter
}

func NewServer(h *Handlers, database DatabasePinger) *Server {
	return &Server{
		h:                           h,
		database:                    database,
		rateLimiter:                 middleware.NewRateLimiter(globalRateLimitRequestsPerSecond, globalRateLimitBurst, globalRateLimitMaxClients, globalRateLimitStaleAfter),
		loginRateLimiter:            middleware.NewRateLimiter(loginRateLimitRequestsPerSecond, loginRateLimitBurst, loginRateLimitMaxClients, loginRateLimitStaleAfter),
		registrationStartLimiter:    middleware.NewRateLimiter(registrationStartRateLimitRequestsPerSecond, registrationStartRateLimitBurst, registrationRateLimitMaxClients, registrationRateLimitStaleAfter),
		registrationConfirmLimiter:  middleware.NewRateLimiter(registrationConfirmRateLimitRequestsPerSecond, registrationConfirmRateLimitBurst, registrationRateLimitMaxClients, registrationRateLimitStaleAfter),
		registrationResendLimiter:   middleware.NewRateLimiter(registrationResendRateLimitRequestsPerSecond, registrationResendRateLimitBurst, registrationRateLimitMaxClients, registrationRateLimitStaleAfter),
		passwordResetStartLimiter:   middleware.NewRateLimiter(passwordResetStartRateLimitRequestsPerSecond, passwordResetStartRateLimitBurst, registrationRateLimitMaxClients, registrationRateLimitStaleAfter),
		passwordResetConfirmLimiter: middleware.NewRateLimiter(passwordResetConfirmRateLimitRequestsPerSecond, passwordResetConfirmRateLimitBurst, registrationRateLimitMaxClients, registrationRateLimitStaleAfter),
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

	startRegistrationHandler := srv.registrationStartLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.UserHandler.StartPasswordRegistration))
	confirmRegistrationHandler := srv.registrationConfirmLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.UserHandler.ConfirmPasswordRegistration))
	resendRegistrationHandler := srv.registrationResendLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.UserHandler.ResendPasswordRegistrationCode))

	mux.Handle("POST /registrations/password", startRegistrationHandler)
	mux.Handle("POST /registrations/password/confirm", confirmRegistrationHandler)
	mux.Handle("POST /registrations/password/resend", resendRegistrationHandler)
	resetHandler := srv.passwordResetStartLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.PasswordResetHandler.Start))
	mux.Handle("POST /password-resets", resetHandler)
	confirmResetHandler := srv.passwordResetConfirmLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.PasswordResetHandler.Confirm))
	mux.Handle("POST /password-resets/confirm", confirmResetHandler)

	mux.Handle("GET /me", srv.h.MeHandler)

	loginHandler := srv.loginRateLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.SessionHandler.LoginWithPassword))
	mux.Handle("POST /sessions", loginHandler)
	mux.HandleFunc("DELETE /sessions/current", srv.h.SessionHandler.Logout)

	googleStartHandler := srv.loginRateLimiter.MiddlewareByIP(http.HandlerFunc(srv.h.GoogleAuthHandler.Start))
	mux.Handle("GET /auth/google", googleStartHandler)
	mux.HandleFunc("GET /auth/google/callback", srv.h.GoogleAuthHandler.Callback)

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
