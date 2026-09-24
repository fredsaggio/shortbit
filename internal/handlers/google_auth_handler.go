package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
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

func (h *GoogleAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	clearGoogleAuthorizationCookies(w, h.cookieSecure)

	query := r.URL.Query()

	if providerError := query.Get("error"); providerError != "" {
		slog.WarnContext(ctx, "Google authorization rejected", "provider_error", providerError)
		http.Error(w, "autenticação com Google cancelada ou recusada", http.StatusBadRequest)
		return
	}

	code := query.Get("code")
	receivedState := query.Get("state")

	if code == "" || receivedState == "" {
		http.Error(w, "resposta de autenticação do Google inválida", http.StatusBadRequest)
		return
	}

	authorizationCookies, err := readGoogleAuthorizationCookies(r)
	if err != nil {
		http.Error(w, "tentativa de autenticação inválida ou expirada", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare(
		[]byte(receivedState),
		[]byte(authorizationCookies.State),
	) != 1 {
		http.Error(w, "tentativa de autenticação inválida ou expirada", http.StatusBadRequest)
		return
	}

	login, err := h.googleAuth.CompleteLogin(ctx, code, authorizationCookies.Nonce, authorizationCookies.CodeVerifier)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrGoogleAuthenticationFailed):
			slog.WarnContext(ctx, "Google authentication failed", "error", err)
			http.Error(w, "não foi possível autenticar com o Google", http.StatusUnauthorized)

		default:
			slog.ErrorContext(ctx, "Complete Google login failed", "error", err)
			http.Error(w, "erro interno no servidor", http.StatusInternalServerError)
		}
		return
	}

	setUserSessionCookie(w, login.Token, login.ExpiresAt, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
