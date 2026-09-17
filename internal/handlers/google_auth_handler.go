package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type GoogleAuthService interface {
	AuthorizationURL(state, nonce, codeVerifier string) string
	CompleteLogin(ctx context.Context, code, expectedNonce, codeVerifier string) (services.LoginResult, error)
}

type GoogleAuthorizationValuesGenerator func() (googleoidc.AuthorizationValues, error)

type GoogleAuthHandler struct {
	googleAuth                  GoogleAuthService
	generateAuthorizationValues GoogleAuthorizationValuesGenerator
	cookieSecure                bool
}

func NewGoogleAuthHandler(googleAuth GoogleAuthService, generateAuthorizationValues GoogleAuthorizationValuesGenerator, cookieSecure bool) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		googleAuth:                  googleAuth,
		generateAuthorizationValues: generateAuthorizationValues,
		cookieSecure:                cookieSecure,
	}
}

func (h *GoogleAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	values, err := h.generateAuthorizationValues()

	if err != nil {
		slog.ErrorContext(ctx, "generate Google authorization values failed", "error", err)
		http.Error(w, "erro interno no servidor", http.StatusInternalServerError)
		return
	}

	setGoogleAuthorizationCookies(w, values.State, values.Nonce, values.CodeVerifier, h.cookieSecure)

	authorizationURL := h.googleAuth.AuthorizationURL(values.State, values.Nonce, values.CodeVerifier)

	http.Redirect(w, r, authorizationURL, http.StatusFound)
}
