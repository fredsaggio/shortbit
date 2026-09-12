package server

import (
	"context"
	"net/http"
	"time"

	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/middleware"
)

const readinessTimeout = 2 * time.Second

type DatabasePinger interface {
	Ping(ctx context.Context) error
}

type Handlers struct {
	UserHandler *handlers.UserHandler
}

type Server struct {
	h        *Handlers
	database DatabasePinger
}

func NewServer(h *Handlers, database DatabasePinger) *Server {
	return &Server{
		h:        h,
		database: database,
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
				http.StatusText(http.StatusServiceUnavailable),
				http.StatusServiceUnavailable,
			)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /users", srv.h.UserHandler.RegisterWithPassword)

}

func (srv *Server) NewRouterHTTP() http.Handler {
	mux := http.NewServeMux()

	srv.registerRoutes(mux)

	handler := http.Handler(mux)

	handler = middleware.LimitRequestBody(handler)
	handler = middleware.Recovery(handler)
	handler = middleware.AccessLog(handler)
	handler = middleware.RequestID(handler)

	return handler
}
