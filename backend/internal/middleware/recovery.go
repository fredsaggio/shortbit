package middleware

import (
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

			http.Error(
				w,
				"erro interno do servidor",
				http.StatusInternalServerError,
			)
		}()
		next.ServeHTTP(w, r)
	})
}
