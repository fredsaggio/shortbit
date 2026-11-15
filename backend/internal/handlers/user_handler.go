package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type UserService interface {
	StartPasswordRegistration(ctx context.Context, email, password, fullName string) (services.PasswordRegistrationResult, error)
	ConfirmPasswordRegistration(ctx context.Context, token, code string) (models.User, error)
	ResendPasswordRegistrationCode(ctx context.Context, token string) error
}

type RegistrationRateLimiter interface {
	Allow(key string) (allowed bool, retryAfter int)
}

type startPasswordRegistrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type confirmPasswordRegistrationRequest struct {
	Code string `json:"code"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UserHandler struct {
	userServ         UserService
	emailRateLimiter RegistrationRateLimiter
	cookieSecure     bool
}

func NewUserHandler(userServ UserService, emailRateLimiter RegistrationRateLimiter, cookieSecure bool) *UserHandler {
	return &UserHandler{
		userServ:         userServ,
		emailRateLimiter: emailRateLimiter,
		cookieSecure:     cookieSecure,
	}
}

func (h *UserHandler) StartPasswordRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req startPasswordRegistrationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		problem.Write(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição inválido")
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		problem.Write(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição deve conter exatamente um objeto JSON")
		return
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(req.Email))
	allowed, retryAfter := h.emailRateLimiter.Allow(normalizedEmail)
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		problem.Write(w, r, http.StatusTooManyRequests, "rate_limited", "muitas solicitações para este email, tente novamente mais tarde")
		return
	}

	result, err := h.userServ.StartPasswordRegistration(ctx, req.Email, req.Password, req.FullName)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidEmail):
			problem.Write(w, r, http.StatusBadRequest, "invalid_email", "email inválido")

		case isPasswordPolicyError(err):
			writePasswordPolicyError(w, r, err)

		case errors.Is(err, services.ErrFullNameRequired), errors.Is(err, services.ErrFullNameTooLong), errors.Is(err, services.ErrInvalidFullName):
			writeFullNameError(w, r, err)

		case errors.Is(err, services.ErrEmailAlreadyExists):
			problem.Write(w, r, http.StatusConflict, "email_already_registered", "email já está em uso")

		default:
			slog.ErrorContext(ctx, "start password registration failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	setPasswordRegistrationCookie(w, result.Token, result.ExpiresAt, h.cookieSecure)
	w.WriteHeader(http.StatusAccepted)
}

func (h *UserHandler) ConfirmPasswordRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req confirmPasswordRegistrationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		problem.Write(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição inválido")
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		problem.Write(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição deve conter exatamente um objeto JSON")
		return
	}

	token, err := requiredCookieValue(r, passwordRegistrationCookieName)

	if err != nil {
		clearPasswordRegistrationCookie(w, h.cookieSecure)
		problem.Write(w, r, http.StatusGone, "registration_unavailable", "tentativa de cadastro ausente ou expirada")
		return
	}

	user, err := h.userServ.ConfirmPasswordRegistration(ctx, token, req.Code)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrRegistrationAttemptUnavailable):
			clearPasswordRegistrationCookie(w, h.cookieSecure)
			problem.Write(w, r, http.StatusGone, "registration_unavailable", "tentativa de cadastro ausente ou expirada")

		case errors.Is(err, services.ErrRegistrationAttemptLocked):
			problem.Write(w, r, http.StatusTooManyRequests, "attempt_locked", "aguarde antes de tentar novamente")

		case errors.Is(err, services.ErrVerificationCodeExpired):
			problem.Write(w, r, http.StatusBadRequest, "verification_code_expired", "código expirado; solicite um novo código")

		case errors.Is(err, services.ErrVerificationCodeInvalid):
			problem.Write(w, r, http.StatusBadRequest, "invalid_verification_code", "código de confirmação inválido")

		case errors.Is(err, services.ErrEmailAlreadyExists):
			clearPasswordRegistrationCookie(w, h.cookieSecure)
			problem.Write(w, r, http.StatusConflict, "email_already_registered", "email já está em uso; faça login")

		default:
			slog.ErrorContext(
				ctx,
				"confirm password registration failed",
				"error",
				err,
			)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}

		return
	}

	clearPasswordRegistrationCookie(w, h.cookieSecure)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := userResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "encode confirmed user response failed", "error", err)
	}
}

func (h *UserHandler) ResendPasswordRegistrationCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, err := requiredCookieValue(r, passwordRegistrationCookieName)

	if err != nil {
		clearPasswordRegistrationCookie(w, h.cookieSecure)
		problem.Write(w, r, http.StatusGone, "registration_unavailable", "tentativa de cadastro ausente ou expirada")
		return
	}

	err = h.userServ.ResendPasswordRegistrationCode(ctx, token)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrRegistrationAttemptUnavailable):
			clearPasswordRegistrationCookie(w, h.cookieSecure)
			problem.Write(w, r, http.StatusGone, "registration_unavailable", "tentativa de cadastro ausente ou expirada")

		case errors.Is(err, services.ErrRegistrationAttemptLocked):
			problem.Write(w, r, http.StatusTooManyRequests, "resend_too_soon", "aguarde antes de solicitar outro código")

		case errors.Is(err, services.ErrRegistrationCodeResendTooSoon):
			problem.Write(w, r, http.StatusTooManyRequests, "resend_too_soon", "aguarde 30 segundos antes de solicitar outro código")

		default:
			slog.ErrorContext(
				ctx,
				"resend password registration code failed",
				"error",
				err,
			)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)

}
