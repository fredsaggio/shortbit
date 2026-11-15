package handlers

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/fredsaggio/url-shortener/internal/googleoidc"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type GoogleAuthService interface {
	AuthorizationURL(state, nonce, codeVerifier string) string
	CompleteLogin(ctx context.Context, code, expectedNonce, codeVerifier string) (services.LoginResult, error)
}

type GoogleAuthorizationValuesGenerator func() (googleoidc.AuthorizationValues, error)

// The callback only redirects to these fixed frontend paths, never to a
// destination taken from the request, so it cannot become an open redirect.
const (
	googleLoginSucceededPath = "/painel"
	googleLoginFailedPath    = "/entrar?erro=google"
)

type GoogleAuthHandler struct {
	googleAuth                  GoogleAuthService
	generateAuthorizationValues GoogleAuthorizationValuesGenerator
	cookieSecure                bool
	frontendURL                 string
}

func NewGoogleAuthHandler(googleAuth GoogleAuthService, generateAuthorizationValues GoogleAuthorizationValuesGenerator, cookieSecure bool, frontendURL string) *GoogleAuthHandler {
	return &GoogleAuthHandler{
		googleAuth:                  googleAuth,
		generateAuthorizationValues: generateAuthorizationValues,
		cookieSecure:                cookieSecure,
		frontendURL:                 frontendURL,
	}
}

func (h *GoogleAuthHandler) redirectToFrontend(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, h.frontendURL+path, http.StatusSeeOther)
}

func (h *GoogleAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	values, err := h.generateAuthorizationValues()

	if err != nil {
		slog.ErrorContext(ctx, "generate Google authorization values failed", "error", err)
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno no servidor")
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
		// The value comes from the URL, so anyone can set it: keep it short.
		slog.WarnContext(ctx, "Google authorization rejected", "provider_error", truncate(providerError, maxLoggedProviderErrorLength))
		h.redirectToFrontend(w, r, googleLoginFailedPath)
		return
	}

	code := query.Get("code")
	receivedState := query.Get("state")

	if code == "" || receivedState == "" {
		slog.WarnContext(ctx, "Google callback without code or state")
		h.redirectToFrontend(w, r, googleLoginFailedPath)
		return
	}

	authorizationCookies, err := readGoogleAuthorizationCookies(r)
	if err != nil {
		slog.WarnContext(ctx, "Google callback without authorization cookies")
		h.redirectToFrontend(w, r, googleLoginFailedPath)
		return
	}

	if subtle.ConstantTimeCompare(
		[]byte(receivedState),
		[]byte(authorizationCookies.State),
	) != 1 {
		slog.WarnContext(ctx, "Google callback state mismatch")
		h.redirectToFrontend(w, r, googleLoginFailedPath)
		return
	}

	login, err := h.googleAuth.CompleteLogin(ctx, code, authorizationCookies.Nonce, authorizationCookies.CodeVerifier)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrGoogleAuthenticationFailed):
			slog.WarnContext(ctx, "Google authentication failed", "error", err)

		default:
			slog.ErrorContext(ctx, "Complete Google login failed", "error", err)
		}
		h.redirectToFrontend(w, r, googleLoginFailedPath)
		return
	}

	// RememberMe como true porque é login utilizando Google OIDC.
	setUserSessionCookie(w, login.Token, login.ExpiresAt, true, h.cookieSecure)
	h.redirectToFrontend(w, r, googleLoginSucceededPath)
}

// Google's error codes are short words such as access_denied.
const maxLoggedProviderErrorLength = 64

func truncate(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}
