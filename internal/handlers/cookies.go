package handlers

import (
	"math"
	"net/http"
	"time"
)

const userSessionCookieName = "user_session"

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
