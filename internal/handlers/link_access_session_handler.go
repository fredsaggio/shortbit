package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fredsaggio/url-shortener/internal/services"
)

type LinkAccessSessionService interface {
	CreateSession(ctx context.Context, shortCode, password string) (services.LinkAccessSessionResult, error)
}

type LinkAccessSessionHandler struct {
	serv         LinkAccessSessionService
	cookieSecure bool
}

func NewLinkAccessSessionHandler(serv LinkAccessSessionService, cookieSecure bool) *LinkAccessSessionHandler {
	return &LinkAccessSessionHandler{
		serv:         serv,
		cookieSecure: cookieSecure,
	}
}

func (h *LinkAccessSessionHandler) RedirectPrivateLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shortCode := r.PathValue("code")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	password := r.PostForm.Get("password")
	if password == "" {
		http.Error(w, "senha do link obrigatória", http.StatusBadRequest)
		return
	}

	result, err := h.serv.CreateSession(ctx, shortCode, password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrURLNotFound):
			http.Error(w, "url não encontrada", http.StatusNotFound)
		case errors.Is(err, services.ErrIncorrectLinkPassword):
			http.Error(w, "senha incorreta", http.StatusUnauthorized)
		default:
			slog.ErrorContext(ctx, "create link access session failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		}
		return
	}

	setLinkAccessSessionCookie(w, shortCode, result.Token, result.ExpiresAt, h.cookieSecure)
	http.Redirect(w, r, "/"+shortCode, http.StatusSeeOther)
}
