package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		recorder := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		slog.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"response_bytes", recorder.bytes,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	bytes       int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.wroteHeader {
		return
	}
	rw.statusCode = statusCode
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(body []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	n, err := rw.ResponseWriter.Write(body)
	rw.bytes += n

	return n, err
}
