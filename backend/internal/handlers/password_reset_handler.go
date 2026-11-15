package handlers

import (
	"context"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"github.com/fredsaggio/url-shortener/internal/telemetry"
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
	if !decodeSingleJSON(w, r, &request) {
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(request.Email))
	allowed, retryAfter := h.emailRateLimiter.Allow(normalizedEmail)
	if !allowed {
		telemetry.SecurityEvent(r.Context(), telemetry.EventRateLimited, slog.String("limiter", "password_reset_email"))
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		problem.Write(w, r, http.StatusTooManyRequests, "rate_limited", "muitas solicitações para este email, tente novamente mais tarde")
		return
	}

	currentToken := ""
	if cookie, err := r.Cookie(passwordResetCookieName); err == nil {
		currentToken = cookie.Value
	} else if !errors.Is(err, http.ErrNoCookie) {
		problem.Write(w, r, http.StatusBadRequest, "invalid_cookie", "cookie de recuperação inválido")
		return
	}

	result, err := h.service.Start(r.Context(), normalizedEmail, currentToken)
	if err != nil {
		if errors.Is(err, services.ErrInvalidEmail) {
			problem.Write(w, r, http.StatusBadRequest, "invalid_email", "email inválido")
			return
		}
		slog.ErrorContext(r.Context(), "start password reset failed", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
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
	if !decodeSingleJSON(w, r, &request) {
		return
	}
	if request.Password != request.PasswordConfirmation {
		problem.Write(w, r, http.StatusBadRequest, "password_confirmation_mismatch", "as senhas não coincidem")
		return
	}

	token, err := requiredCookieValue(r, passwordResetCookieName)
	if err != nil {
		clearPasswordResetCookie(w, h.cookieSecure)
		problem.Write(w, r, http.StatusGone, "password_reset_unavailable", "tentativa de recuperação ausente ou expirada")
		return
	}

	if err := h.service.Confirm(r.Context(), token, request.Code, request.Password); err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordResetAttemptUnavailable):
			clearPasswordResetCookie(w, h.cookieSecure)
			problem.Write(w, r, http.StatusGone, "password_reset_unavailable", "tentativa de recuperação ausente ou expirada")
		case errors.Is(err, services.ErrPasswordResetAttemptLocked):
			telemetry.SecurityEvent(r.Context(), telemetry.EventVerificationFailed, slog.String("flow", "password_reset"), slog.String("reason", "locked"))
			problem.Write(w, r, http.StatusTooManyRequests, "attempt_locked", "aguarde antes de tentar novamente")
		case errors.Is(err, services.ErrPasswordResetCodeExpired):
			problem.Write(w, r, http.StatusBadRequest, "verification_code_expired", "código expirado; solicite um novo código")
		case errors.Is(err, services.ErrPasswordResetCodeInvalid):
			telemetry.SecurityEvent(r.Context(), telemetry.EventVerificationFailed, slog.String("flow", "password_reset"), slog.String("reason", "invalid_code"))
			problem.Write(w, r, http.StatusBadRequest, "invalid_verification_code", "código de recuperação inválido")
		case isPasswordPolicyError(err):
			writePasswordPolicyError(w, r, err)
		default:
			slog.ErrorContext(r.Context(), "confirm password reset failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	clearPasswordResetCookie(w, h.cookieSecure)
	telemetry.SecurityEvent(r.Context(), telemetry.EventPasswordResetComplete)
	w.WriteHeader(http.StatusNoContent)
}
