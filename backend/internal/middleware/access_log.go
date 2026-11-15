package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/fredsaggio/url-shortener/internal/telemetry"
)

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		recorder := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		ctx, route := telemetry.WithRouteHolder(r.Context())
		next.ServeHTTP(recorder, r.WithContext(ctx))
		elapsed := time.Since(startedAt)

		telemetry.RecordHTTPRequest(ctx, r.Method, route(), recorder.statusCode, elapsed)
		slog.InfoContext(
			ctx,
			"http request",
			"request_id", RequestIDFromContext(ctx),
			"method", r.Method,
			"route", route(),
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", elapsed.Milliseconds(),
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
