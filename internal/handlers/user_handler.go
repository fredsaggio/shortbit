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
	RegisterWithPassword(ctx context.Context, email string, password string) (models.User, error)
	GetByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

type UserHandler struct {
	userServ UserService
}

type createUserWithPasswordRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func NewUserHandler(userServ UserService) *UserHandler {
	return &UserHandler{
		userServ: userServ,
	}
}

func (h *UserHandler) RegisterWithPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req createUserWithPasswordRequest

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

	user, err := h.userServ.RegisterWithPassword(ctx, req.Email, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidEmail):
			http.Error(
				w,
				"email inválido",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrPasswordTooShort):
			http.Error(
				w,
				"senha muito curta",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrPasswordTooLong):
			http.Error(
				w,
				"senha muito longa",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrEmailAlreadyExists):
			http.Error(
				w,
				"email já está em uso",
				http.StatusConflict,
			)

		default:
			http.Error(
				w,
				"erro interno do servidor",
				http.StatusInternalServerError,
			)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(userResponse{
		ID:    user.ID.String(),
		Email: user.Email,
	}); err != nil {
		slog.Error("error to encode response")
		return
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
