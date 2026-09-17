package handlers

import (
	"math"
	"net/http"
	"time"
)

const (
	userSessionCookieName            = "user_session"
	googleAuthStateCookieName        = "google_auth_state"
	googleAuthNonceCookieName        = "google_auth_nonce"
	googleAuthCodeVerifierCookieName = "google_auth_code_verifier"
	googleAuthCallbackPath           = "/auth/google/callback"
	googleAuthorizationCookieTTL     = 5 * time.Minute
)

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

func setGoogleAuthorizationCookies(w http.ResponseWriter, state, nonce, codeVerifier string, secure bool) {
	expiresAt := time.Now().Add(googleAuthorizationCookieTTL).UTC()

	cookies := []http.Cookie{
		{Name: googleAuthStateCookieName, Value: state},
		{Name: googleAuthNonceCookieName, Value: nonce},
		{Name: googleAuthCodeVerifierCookieName, Value: codeVerifier},
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
