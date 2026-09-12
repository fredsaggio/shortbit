package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/services"
)

type UserHandler struct {
	userServ *services.UserService
}

type createUserWithPasswordRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func NewUserHandler(userServ *services.UserService) *UserHandler {
	return &UserHandler{
		userServ: userServ,
	}
}

func (h *UserHandler) CreateWithPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req createUserWithPasswordRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request body must contain one JSON object", http.StatusBadRequest)
		return
	}

	user, err := h.userServ.RegisterWithPassword(ctx, req.Email, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidEmail):
			http.Error(
				w,
				"invalid email",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrPasswordTooShort):
			http.Error(
				w,
				"password is too short",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrPasswordTooLong):
			http.Error(
				w,
				"password is too long",
				http.StatusBadRequest,
			)

		case errors.Is(err, services.ErrEmailAlreadyExists):
			http.Error(
				w,
				"email already exists",
				http.StatusConflict,
			)

		default:
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(createUserResponse{
			ID:    user.ID.String(),
			Email: user.Email,
		}); err != nil {
			slog.Error("error to encode response")
			return
		}
}
