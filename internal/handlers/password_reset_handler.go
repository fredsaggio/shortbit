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

type PasswordResetStarter interface {
	Start(ctx context.Context, email, currentToken string) (services.PasswordResetStartResult, error)
}

type PasswordResetHandler struct {
	service          PasswordResetStarter
	emailRateLimiter RegistrationRateLimiter
	cookieSecure     bool
}

func NewPasswordResetHandler(service PasswordResetStarter, emailRateLimiter RegistrationRateLimiter, cookieSecure bool) *PasswordResetHandler {
	return &PasswordResetHandler{service: service, emailRateLimiter: emailRateLimiter, cookieSecure: cookieSecure}
}

func (h *PasswordResetHandler) Start(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "corpo da requisição deve conter exatamente um objeto JSON", http.StatusBadRequest)
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(request.Email))
	allowed, retryAfter := h.emailRateLimiter.Allow(normalizedEmail)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		http.Error(w, "muitas solicitações para este email, tente novamente mais tarde", http.StatusTooManyRequests)
		return
	}

	currentToken := ""
	if cookie, err := r.Cookie(passwordResetCookieName); err == nil {
		currentToken = cookie.Value
	} else if !errors.Is(err, http.ErrNoCookie) {
		http.Error(w, "cookie de recuperação inválido", http.StatusBadRequest)
		return
	}

	result, err := h.service.Start(r.Context(), normalizedEmail, currentToken)
	if err != nil {
		if errors.Is(err, services.ErrInvalidEmail) {
			http.Error(w, "email inválido", http.StatusBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "start password reset failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	setPasswordResetCookie(w, result.Token, h.cookieSecure)
	w.WriteHeader(http.StatusAccepted)
}
