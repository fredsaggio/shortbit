package middleware

import (
	"github.com/fredsaggio/url-shortener/internal/problem"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			slog.Error(
				"panic recovered",
				"request_id", RequestIDFromContext(r.Context()),
				"panic", recovered,
				"stack", string(debug.Stack()),
			)

			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}()
		next.ServeHTTP(w, r)
	})
}
