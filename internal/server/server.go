package server

import (
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/middleware"
)

type Handlers struct {
}

type Server struct {
	h *Handlers
}

func NewServer(h *Handlers) *Server {
	return &Server{
		h: h,
	}
}

func (srv *Server) registerRoutes(mux *http.ServeMux) {

}

func (srv *Server) NewRouterHTTP() http.Handler {
	mux := http.NewServeMux()

	srv.registerRoutes(mux)

	handler := http.Handler(mux)

	handler = middleware.AccessLog(handler)
	handler = middleware.RequestID(handler)

	return handler
}
