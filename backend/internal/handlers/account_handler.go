package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type AccountService interface {
	GetAccount(ctx context.Context, userID uuid.UUID) (models.Account, error)
	UpdateFullName(ctx context.Context, userID uuid.UUID, fullName string) (models.Account, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, sessionToken, currentPassword, newPassword, confirmation string) (services.PasswordChangeResult, error)
}

type PasswordChangeRateLimiter interface {
	Allow(key string) (allowed bool, retryAfter int)
}

type accountResponse struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	FullName    *string   `json:"full_name"`
	HasPassword bool      `json:"has_password"`
	HasGoogle   bool      `json:"has_google"`
	CreatedAt   time.Time `json:"created_at"`
}

type updateAccountRequest struct {
	FullName string `json:"full_name"`
}

type changePasswordRequest struct {
	CurrentPassword         string `json:"current_password"`
	NewPassword             string `json:"new_password"`
	NewPasswordConfirmation string `json:"new_password_confirmation"`
}

type AccountHandler struct {
	accounts     AccountService
	rateLimiter  PasswordChangeRateLimiter
	cookieSecure bool
}

func NewAccountHandler(accounts AccountService, rateLimiter PasswordChangeRateLimiter, cookieSecure bool) *AccountHandler {
	return &AccountHandler{accounts: accounts, rateLimiter: rateLimiter, cookieSecure: cookieSecure}
}

func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	account, err := h.accounts.GetAccount(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "get account failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	writeAccount(ctx, w, account)
}

func (h *AccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	var req updateAccountRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	account, err := h.accounts.UpdateFullName(ctx, userID, req.FullName)
	if err != nil {
		if errors.Is(err, services.ErrFullNameRequired) || errors.Is(err, services.ErrFullNameTooLong) || errors.Is(err, services.ErrInvalidFullName) {
			http.Error(w, fullNameErrorMessage(err), http.StatusBadRequest)
			return
		}
		slog.ErrorContext(ctx, "update full name failed", "error", err)
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	writeAccount(ctx, w, account)
}

func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		return
	}

	// Counted per account, not per IP, and checked before the Argon2id
	// comparison of the current password.
	if allowed, retryAfter := h.rateLimiter.Allow(userID.String()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		http.Error(w, "muitas tentativas de troca de senha, tente novamente mais tarde", http.StatusTooManyRequests)
		return
	}

	var req changePasswordRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	cookie, err := r.Cookie(userSessionCookieName)
	if err != nil {
		http.Error(w, "autenticação necessária", http.StatusUnauthorized)
		return
	}

	result, err := h.accounts.ChangePassword(ctx, userID, cookie.Value, req.CurrentPassword, req.NewPassword, req.NewPasswordConfirmation)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordConfirmationMismatch):
			http.Error(w, "as senhas não coincidem", http.StatusBadRequest)
		case errors.Is(err, services.ErrPasswordTooShort):
			http.Error(w, "senha muito curta", http.StatusBadRequest)
		case errors.Is(err, services.ErrPasswordTooLong):
			http.Error(w, "senha muito longa", http.StatusBadRequest)
		case errors.Is(err, services.ErrIncorrectCurrentPassword):
			http.Error(w, "senha atual incorreta", http.StatusBadRequest)
		case errors.Is(err, services.ErrSamePassword):
			http.Error(w, "a nova senha deve ser diferente da atual", http.StatusBadRequest)
		case errors.Is(err, services.ErrAccountHasNoPassword):
			http.Error(w, "esta conta entra com Google e não tem senha", http.StatusConflict)
		case errors.Is(err, services.ErrUnauthenticated):
			http.Error(w, "autenticação necessária", http.StatusUnauthorized)
		default:
			slog.ErrorContext(ctx, "change password failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		}
		return
	}

	setUserSessionCookie(w, result.Session.Token, result.Session.ExpiresAt, result.RememberMe, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func writeAccount(ctx context.Context, w http.ResponseWriter, account models.Account) {
	w.Header().Set("Content-Type", "application/json")
	resp := accountResponse{
		ID:          account.ID.String(),
		Email:       account.Email,
		FullName:    account.FullName,
		HasPassword: account.HasPassword,
		HasGoogle:   account.HasGoogle,
		CreatedAt:   account.CreatedAt.UTC(),
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		slog.ErrorContext(ctx, "encode account response failed", "error", err)
	}
}

// decodeSingleJSON writes the 400 itself and reports whether decoding worked.
func decodeSingleJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "corpo da requisição deve conter exatamente um objeto JSON", http.StatusBadRequest)
		return false
	}
	return true
}
