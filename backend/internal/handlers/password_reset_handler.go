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
	Confirm(ctx context.Context, token, code, password string) error
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

func (h *PasswordResetHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code                 string `json:"code"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
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
	if request.Password != request.PasswordConfirmation {
		http.Error(w, "as senhas não coincidem", http.StatusBadRequest)
		return
	}

	token, err := requiredCookieValue(r, passwordResetCookieName)
	if err != nil {
		clearPasswordResetCookie(w, h.cookieSecure)
		http.Error(w, "tentativa de recuperação ausente ou expirada", http.StatusGone)
		return
	}

	if err := h.service.Confirm(r.Context(), token, request.Code, request.Password); err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordResetAttemptUnavailable):
			clearPasswordResetCookie(w, h.cookieSecure)
			http.Error(w, "tentativa de recuperação ausente ou expirada", http.StatusGone)
		case errors.Is(err, services.ErrPasswordResetAttemptLocked):
			http.Error(w, "aguarde antes de tentar novamente", http.StatusTooManyRequests)
		case errors.Is(err, services.ErrPasswordResetCodeExpired):
			http.Error(w, "código expirado; solicite um novo código", http.StatusBadRequest)
		case errors.Is(err, services.ErrPasswordResetCodeInvalid):
			http.Error(w, "código de recuperação inválido", http.StatusBadRequest)
		case errors.Is(err, services.ErrPasswordTooShort):
			http.Error(w, "senha muito curta", http.StatusBadRequest)
		case errors.Is(err, services.ErrPasswordTooLong):
			http.Error(w, "senha muito longa", http.StatusBadRequest)
		default:
			slog.ErrorContext(r.Context(), "confirm password reset failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		}
		return
	}

	clearPasswordResetCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
