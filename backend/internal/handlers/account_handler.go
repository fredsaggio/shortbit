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
	DeleteAccount(ctx context.Context, userID uuid.UUID, password, email string) error
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

// The account with a password sends it; a Google-only account sends its email.
type deleteAccountRequest struct {
	Password string `json:"password"`
	Email    string `json:"email"`
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
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	account, err := h.accounts.GetAccount(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "get account failed", "error", err)
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	writeAccount(ctx, w, account)
}

func (h *AccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	var req updateAccountRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	account, err := h.accounts.UpdateFullName(ctx, userID, req.FullName)
	if err != nil {
		if errors.Is(err, services.ErrFullNameRequired) || errors.Is(err, services.ErrFullNameTooLong) || errors.Is(err, services.ErrInvalidFullName) {
			writeFullNameError(w, err)
			return
		}
		slog.ErrorContext(ctx, "update full name failed", "error", err)
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	writeAccount(ctx, w, account)
}

func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	// Counted per account, not per IP, and checked before the Argon2id
	// comparison of the current password.
	if allowed, retryAfter := h.rateLimiter.Allow(userID.String()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		problem.Write(w, http.StatusTooManyRequests, "rate_limited", "muitas tentativas de troca de senha, tente novamente mais tarde")
		return
	}

	var req changePasswordRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	cookie, err := r.Cookie(userSessionCookieName)
	if err != nil {
		problem.Write(w, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	result, err := h.accounts.ChangePassword(ctx, userID, cookie.Value, req.CurrentPassword, req.NewPassword, req.NewPasswordConfirmation)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPasswordConfirmationMismatch):
			problem.Write(w, http.StatusBadRequest, "password_confirmation_mismatch", "as senhas não coincidem")
		case isPasswordPolicyError(err):
			writePasswordPolicyError(w, err)
		case errors.Is(err, services.ErrIncorrectCurrentPassword):
			problem.Write(w, http.StatusBadRequest, "incorrect_current_password", "senha atual incorreta")
		case errors.Is(err, services.ErrSamePassword):
			problem.Write(w, http.StatusBadRequest, "same_password", "a nova senha deve ser diferente da atual")
		case errors.Is(err, services.ErrAccountHasNoPassword):
			problem.Write(w, http.StatusConflict, "account_has_no_password", "esta conta entra com Google e não tem senha")
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		default:
			slog.ErrorContext(ctx, "change password failed", "error", err)
			problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	setUserSessionCookie(w, result.Session.Token, result.Session.ExpiresAt, result.RememberMe, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// Delete handles DELETE /me. It shares the per-account limit of password
// changes, so guessing the password through either endpoint draws on the same
// budget, checked before any Argon2id comparison.
func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	if allowed, retryAfter := h.rateLimiter.Allow(userID.String()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		problem.Write(w, http.StatusTooManyRequests, "rate_limited", "muitas tentativas, tente novamente mais tarde")
		return
	}

	var req deleteAccountRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	if err := h.accounts.DeleteAccount(ctx, userID, req.Password, req.Email); err != nil {
		switch {
		case errors.Is(err, services.ErrIncorrectCurrentPassword):
			problem.Write(w, http.StatusBadRequest, "incorrect_password", "senha incorreta")
		case errors.Is(err, services.ErrAccountEmailMismatch):
			problem.Write(w, http.StatusBadRequest, "account_email_mismatch", "o email não confere com o da conta")
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		default:
			slog.ErrorContext(ctx, "delete account failed", "error", err)
			problem.Write(w, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	clearUserSessionCookie(w, h.cookieSecure)
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
		problem.Write(w, http.StatusBadRequest, "invalid_body", "corpo da requisição inválido")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		problem.Write(w, http.StatusBadRequest, "invalid_body", "corpo da requisição deve conter exatamente um objeto JSON")
		return false
	}
	return true
}
