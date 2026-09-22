package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type UserService interface {
	StartPasswordRegistration(ctx context.Context, email, password string) (services.PasswordRegistrationResult, error)
	ConfirmPasswordRegistration(ctx context.Context, token, code string) (models.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

type startPasswordRegistrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type confirmPasswordRegistrationRequest struct {
	Code string `json:"code"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type UserHandler struct {
	userServ     UserService
	cookieSecure bool
}

func NewUserHandler(userServ UserService, cookieSecure bool) *UserHandler {
	return &UserHandler{
		userServ:     userServ,
		cookieSecure: cookieSecure,
	}
}

func (h *UserHandler) StartPasswordRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req startPasswordRegistrationRequest

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

	result, err := h.userServ.StartPasswordRegistration(ctx, req.Email, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidEmail):
			http.Error(w, "email inválido", http.StatusBadRequest)

		case errors.Is(err, services.ErrPasswordTooShort):
			http.Error(w, "senha muito curta", http.StatusBadRequest)

		case errors.Is(err, services.ErrPasswordTooLong):
			http.Error(w, "senha muito longa", http.StatusBadRequest)

		case errors.Is(err, services.ErrEmailAlreadyExists):
			http.Error(w, "email já está em uso", http.StatusConflict)

		default:
			slog.ErrorContext(ctx, "start password registration failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
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
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "corpo da requisição deve conter exatamente um objeto JSON", http.StatusBadRequest)
		return
	}

	token, err := requiredCookieValue(r, passwordRegistrationCookieName)

	if err != nil {
		clearPasswordRegistrationCookie(w, h.cookieSecure)
		http.Error(w, "tentativa de cadastro ausente ou expirada", http.StatusGone)
		return
	}

	user, err := h.userServ.ConfirmPasswordRegistration(ctx, token, req.Code)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrRegistrationAttemptUnavailable):
			clearPasswordRegistrationCookie(w, h.cookieSecure)
			http.Error(
				w,
				"tentativa de cadastro ausente ou expirada",
				http.StatusGone,
			)

		case errors.Is(err, services.ErrRegistrationAttemptLocked):
			http.Error(
				w,
				"aguarde antes de tentar novamente",
				http.StatusTooManyRequests,
			)

		case errors.Is(err, services.ErrVerificationCodeExpired):
			http.Error(
				w,
				"código expirado; solicite um novo código",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrVerificationCodeInvalid):
			http.Error(
				w,
				"código de confirmação inválido",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrEmailAlreadyExists):
			clearPasswordRegistrationCookie(w, h.cookieSecure)
			http.Error(
				w,
				"email já está em uso; faça login",
				http.StatusConflict,
			)

		default:
			slog.ErrorContext(
				ctx,
				"confirm password registration failed",
				"error",
				err,
			)
			http.Error(
				w,
				"erro interno do servidor",
				http.StatusInternalServerError,
			)
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

func (h *UserHandler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)

	if !ok {
		http.Error(w, "Erro interno no servidor", http.StatusInternalServerError)
		return
	}

	user, err := h.userServ.GetByID(ctx, userID)

	if err != nil {
		slog.ErrorContext(ctx, "get authenticated user failed", "error", err)
		http.Error(w, "erro interno no servidor", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := userResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	}

	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		slog.ErrorContext(ctx, "encode authenticated user response failed", "error", err)
	}
}
