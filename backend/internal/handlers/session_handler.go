package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "corpo da requisição deve conter exatamente um objeto JSON", http.StatusBadRequest)
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	allowed, retryAfter := h.emailRateLimiter.Allow(normalizedEmail)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		http.Error(w, "muitas tentativas de login, tente novamente mais tarde", http.StatusTooManyRequests)
		return
	}

	login, err := h.authServ.LoginWithPassword(ctx, normalizedEmail, req.Password, req.RememberMe)

	if err != nil {
		if errors.Is(err, services.ErrIncorrectEmailOrPassword) {
			http.Error(w, "email ou senha incorretos", http.StatusUnauthorized)
			return
		}

		slog.Error("login failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
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
		http.Error(w, "cookie de sessão inválido", http.StatusBadRequest)
		return
	}

	err = h.authServ.Logout(ctx, token)

	if err != nil {
		slog.Error("logout failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	clearUserSessionCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
