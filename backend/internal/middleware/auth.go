package middleware

import (
	"context"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
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
				problem.Write(w, http.StatusUnauthorized, "authentication_required", "não autenticado")
				return
			}

			userID, err := auth.Authenticate(ctx, cookie.Value)
			if err != nil {
				if errors.Is(err, services.ErrUnauthenticated) {
					problem.Write(w, http.StatusUnauthorized, "authentication_required", "não autenticado")
					return
				}

				slog.ErrorContext(ctx, "authenticate session failed", "error", err)
				problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
				return
			}
			ctx = ctxval.ContextWithUserID(ctx, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
