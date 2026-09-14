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
	Login(ctx context.Context, email, password string) (services.LoginResult, error)
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewSessionHandler(authServ AuthService, emailRateLimiter LoginRateLimiter, cookieSecure bool) *SessionHandler {
	return &SessionHandler{
		authServ:         authServ,
		emailRateLimiter: emailRateLimiter,
		cookieSecure:     cookieSecure,
	}
}

func (h *SessionHandler) Login(w http.ResponseWriter, r *http.Request) {
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

	login, err := h.authServ.Login(ctx, normalizedEmail, req.Password)

	if err != nil {
		if errors.Is(err, services.ErrIncorrectEmailOrPassword) {
			http.Error(w, "email ou senha incorretos", http.StatusUnauthorized)
			return
		}

		slog.Error("login failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	setUserSessionCookie(w, login.Token, login.ExpiresAt, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
