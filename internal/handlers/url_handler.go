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

type URLService interface {
	Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (services.CreateURLResult, error)
}

type createURLRequest struct {
	URL        string            `json:"url"`
	Visibility models.Visibility `json:"visibility"`
	Password   string            `json:"password"`
}

type createURLResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

type URLHandler struct {
	serv URLService
}

func NewURLHandler(serv URLService) *URLHandler {
	return &URLHandler{serv: serv}
}

func (h *URLHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctxval.UserIDFromContext(ctx)

	if !ok {
		http.Error(w, "autenticação necessária", http.StatusUnauthorized)
		return
	}

	var req createURLRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "corpo da requisição inválido", http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "corpo da requisição deve conter apenas um objeto JSON", http.StatusBadRequest)
		return
	}

	result, err := h.serv.Create(ctx, userID, req.URL, req.Visibility, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			http.Error(w, "autenticação necessária", http.StatusUnauthorized)

		case errors.Is(err, services.ErrInvalidOriginalURL):
			http.Error(w, "URL original inválida", http.StatusBadRequest)

		case errors.Is(err, services.ErrURLTooLong):
			http.Error(w, "URL original muito longa", http.StatusBadRequest)

		case errors.Is(err, services.ErrURLPointsToShortDomain):
			http.Error(w, "não é permitido encurtar o próprio domínio", http.StatusBadRequest)

		case errors.Is(err, services.ErrInvalidURLVisibility):
			http.Error(w, "visibilidade inválida", http.StatusBadRequest)

		case errors.Is(err, services.ErrUnexpectedLinkPassword):
			http.Error(w, "link público não deve ter senha", http.StatusBadRequest)

		case errors.Is(err, services.ErrLinkPasswordTooShort):
			http.Error(w, "senha do link muito curta", http.StatusBadRequest)

		case errors.Is(err, services.ErrLinkPasswordTooLong):
			http.Error(w, "senha do link muito longa", http.StatusBadRequest)

		default:
			slog.ErrorContext(ctx, "create URL failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := createURLResponse{ShortCode: result.ShortCode, ShortURL: result.ShortURL}

	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		slog.ErrorContext(ctx, "encode created URL response failed", "error", err)
	}
}
