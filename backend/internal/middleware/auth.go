package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type AuthService interface {
	Authenticate(ctx context.Context, token string) (uuid.UUID, error)
}

func Authenticator(auth AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("user_session")
			ctx := r.Context()
			if err != nil {
				http.Error(w, "não autenticado", http.StatusUnauthorized)
				return
			}

			userID, err := auth.Authenticate(ctx, cookie.Value)
			if err != nil {
				if errors.Is(err, services.ErrUnauthenticated) {
					http.Error(w, "não autenticado", http.StatusUnauthorized)
					return
				}

				slog.ErrorContext(ctx, "authenticate session failed", "error", err)
				http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
				return
			}
			ctx = ctxval.ContextWithUserID(ctx, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
