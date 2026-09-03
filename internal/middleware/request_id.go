package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

const requestIDHeader = "X-Request-ID"

type requestIDContextKey struct{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := rand.Text()

		w.Header().Set(requestIDHeader, requestID)

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	// Without _, the lack of a value here would cause a panic
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)

	return requestID
}
