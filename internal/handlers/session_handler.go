package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/services"
)

type AuthService interface {
	Login(ctx context.Context, email, password string) (services.LoginResult, error)
}

type SessionHandler struct {
	authServ     AuthService
	cookieSecure bool
}

type createSessionRequest struct {
  	Email    string `json:"email"`
  	Password string `json:"password"`
  }

func NewSessionHandler(authServ AuthService, cookieSecure bool) *SessionHandler {
	return &SessionHandler{
		authServ: authServ,
		cookieSecure: cookieSecure,
	}
}

func (h *SessionHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createSessionRequest

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

	login, err := h.authServ.Login(ctx, req.Email, req.Password)

	if err != nil {
		if errors.Is(err, services.ErrIncorrectEmailOrPassword) {
			http.Error(w, "email or password is incorrect", http.StatusUnauthorized)
			return
		}

		slog.Error("login failed", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	setUserSessionCookie(w, login.Token, login.ExpiresAt, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
