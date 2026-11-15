package handlers

import (
	"context"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/fredsaggio/url-shortener/internal/services"
)

type AuthService interface {
	LoginWithPassword(ctx context.Context, email, password string, rememberMe bool) (services.LoginResult, error)
	Logout(ctx context.Context, token string) error
}

type LoginRateLimiter interface {
	Allow(key string) (allowed bool, retryAfter int)
}

type SessionHandler struct {
	authServ         AuthService
	emailRateLimiter LoginRateLimiter
	cookieSecure     bool
}

type createSessionRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

func NewSessionHandler(authServ AuthService, emailRateLimiter LoginRateLimiter, cookieSecure bool) *SessionHandler {
	return &SessionHandler{
		authServ:         authServ,
		emailRateLimiter: emailRateLimiter,
		cookieSecure:     cookieSecure,
	}
}

func (h *SessionHandler) LoginWithPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createSessionRequest

	if !decodeSingleJSON(w, r, &req) {
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	allowed, retryAfter := h.emailRateLimiter.Allow(normalizedEmail)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		problem.Write(w, r, http.StatusTooManyRequests, "rate_limited", "muitas tentativas de login, tente novamente mais tarde")
		return
	}

	login, err := h.authServ.LoginWithPassword(ctx, normalizedEmail, req.Password, req.RememberMe)

	if err != nil {
		if errors.Is(err, services.ErrIncorrectEmailOrPassword) {
			problem.Write(w, r, http.StatusUnauthorized, "invalid_credentials", "email ou senha incorretos")
			return
		}

		slog.Error("login failed", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	setUserSessionCookie(w, login.Token, login.ExpiresAt, req.RememberMe, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := ""

	cookie, err := r.Cookie(userSessionCookieName)

	if err == nil {
		token = cookie.Value

	} else if !errors.Is(err, http.ErrNoCookie) {
		problem.Write(w, r, http.StatusBadRequest, "invalid_cookie", "cookie de sessão inválido")
		return
	}

	err = h.authServ.Logout(ctx, token)

	if err != nil {
		slog.Error("logout failed", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	clearUserSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
