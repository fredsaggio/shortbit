package handlers

import (
	"fmt"
	"math"
	"net/http"
	"time"
)

const (
	userSessionCookieName = "user_session"

	googleAuthStateCookieName        = "google_auth_state"
	googleAuthNonceCookieName        = "google_auth_nonce"
	googleAuthCodeVerifierCookieName = "google_auth_code_verifier"

	googleAuthCallbackPath       = "/auth/google/callback"
	googleAuthorizationCookieTTL = 5 * time.Minute

	passwordRegistrationCookieName = "password_registration"
	passwordRegistrationCookiePath = "/registrations/password"
	passwordResetCookieName        = "password_reset"
	passwordResetCookiePath        = "/password-resets"
)

func setPasswordResetCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     passwordResetCookieName,
		Value:    token,
		Path:     passwordResetCookiePath,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearPasswordResetCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     passwordResetCookieName,
		Value:    "",
		Path:     passwordResetCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func setPasswordRegistrationCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	maxAge := int(math.Ceil(time.Until(expiresAt).Seconds()))

	http.SetCookie(w, &http.Cookie{
		Name:     passwordRegistrationCookieName,
		Value:    token,
		Path:     passwordRegistrationCookiePath,
		MaxAge:   maxAge,
		Expires:  expiresAt.UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearPasswordRegistrationCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     passwordRegistrationCookieName,
		Value:    "",
		Path:     passwordRegistrationCookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func setUserSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time, secure bool) {
	maxAge := int(math.Ceil(time.Until(expiresAt).Seconds()))

	http.SetCookie(w, &http.Cookie{
		Name:     userSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt.UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearUserSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     userSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

type googleAuthorizationCookies struct {
	State        string
	Nonce        string
	CodeVerifier string
}

func setGoogleAuthorizationCookies(w http.ResponseWriter, state, nonce, codeVerifier string, secure bool) {
	expiresAt := time.Now().Add(googleAuthorizationCookieTTL).UTC()

	cookies := []http.Cookie{
		{
			Name:  googleAuthStateCookieName,
			Value: state,
		},
		{
			Name:  googleAuthNonceCookieName,
			Value: nonce,
		},
		{
			Name:  googleAuthCodeVerifierCookieName,
			Value: codeVerifier,
		},
	}

	for _, cookie := range cookies {
		cookie.Path = googleAuthCallbackPath
		cookie.MaxAge = int(googleAuthorizationCookieTTL.Seconds())
		cookie.Expires = expiresAt
		cookie.HttpOnly = true
		cookie.Secure = secure
		cookie.SameSite = http.SameSiteLaxMode

		http.SetCookie(w, &cookie)
	}
}

func readGoogleAuthorizationCookies(r *http.Request) (googleAuthorizationCookies, error) {
	state, err := requiredCookieValue(r, googleAuthStateCookieName)

	if err != nil {
		return googleAuthorizationCookies{}, fmt.Errorf("read Google OAuth state cookie: %w", err)
	}

	nonce, err := requiredCookieValue(r, googleAuthNonceCookieName)

	if err != nil {
		return googleAuthorizationCookies{}, fmt.Errorf("read Google OIDC nonce cookie: %w", err)
	}

	codeVerifier, err := requiredCookieValue(r, googleAuthCodeVerifierCookieName)

	if err != nil {
		return googleAuthorizationCookies{}, fmt.Errorf("read Google PKCE code verifier cookie: %w", err)
	}

	return googleAuthorizationCookies{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
	}, nil
}

func clearGoogleAuthorizationCookies(w http.ResponseWriter, secure bool) {
	cookieNames := []string{
		googleAuthStateCookieName,
		googleAuthNonceCookieName,
		googleAuthCodeVerifierCookieName,
	}

	for _, name := range cookieNames {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     googleAuthCallbackPath,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0).UTC(),
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func requiredCookieValue(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}

	if cookie.Value == "" {
		return "", fmt.Errorf("cookie %q is empty", name)
	}

	return cookie.Value, nil
}
