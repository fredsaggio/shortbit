package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

const defaultURLListLimit = 20

type URLService interface {
	Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (services.CreateURLResult, error)
	List(ctx context.Context, userID uuid.UUID, limit int, cursor *repositories.URLCursor) (services.ListURLResult, error)
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

type listedURLResponse struct {
	ShortCode   string            `json:"short_code"`
	OriginalURL string            `json:"original_url"`
	Visibility  models.Visibility `json:"visibility"`
	ClickCount  int64             `json:"click_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type listURLResponse struct {
	URLs       []listedURLResponse `json:"urls"`
	NextCursor *string             `json:"next_cursor"`
}

type urlCursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int64     `json:"id"`
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

func (h *URLHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		http.Error(w, "autenticação necessária", http.StatusUnauthorized)
		return
	}

	limit := defaultURLListLimit
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "parâmetros inválidos", http.StatusBadRequest)
		return
	}
	if values, present := query["limit"]; present {
		if len(values) != 1 {
			http.Error(w, "limite inválido", http.StatusBadRequest)
			return
		}
		limit, err = strconv.Atoi(values[0])
		if err != nil {
			http.Error(w, "limite inválido", http.StatusBadRequest)
			return
		}
	}

	var cursor *repositories.URLCursor
	if values, present := query["cursor"]; present {
		// O parâmetro pode vir mais de uma vez, tipo: /urls?cursor=abc&cursor=def
		if len(values) != 1 {
			http.Error(w, "cursor inválido", http.StatusBadRequest)
			return
		}
		decoded, err := base64.RawURLEncoding.DecodeString(values[0])
		var payload urlCursorPayload
		if err != nil || json.Unmarshal(decoded, &payload) != nil || payload.ID <= 0 || payload.CreatedAt.IsZero() {
			http.Error(w, "cursor inválido", http.StatusBadRequest)
			return
		}
		cursor = &repositories.URLCursor{CreatedAt: payload.CreatedAt, ID: payload.ID}
	}

	result, err := h.serv.List(ctx, userID, limit, cursor)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			http.Error(w, "autenticação necessária", http.StatusUnauthorized)
		case errors.Is(err, services.ErrInvalidURLListLimit):
			http.Error(w, "limite inválido", http.StatusBadRequest)
		default:
			slog.ErrorContext(ctx, "list URLs failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
		}
		return
	}
	// O valor é 0 no make porque a lista tem que começar vazia. Se começasse preenchia, os appends não iriam para os índices certos (iriam para depois dos zeros).
	resp := listURLResponse{URLs: make([]listedURLResponse, 0, len(result.URLs))}
	for _, url := range result.URLs {
		// result.URLs devolve models.URL, ou seja, podemos ter acesso aos campos do model e jogá-los na response criada.
		resp.URLs = append(resp.URLs, listedURLResponse{
			ShortCode: url.ShortCode, OriginalURL: url.OriginalURL, Visibility: url.Visibility,
			ClickCount: url.ClickCount, CreatedAt: url.CreatedAt, UpdatedAt: url.UpdatedAt,
		})
	}
	if result.NextCursor != nil {
		encoded, err := json.Marshal(urlCursorPayload{CreatedAt: result.NextCursor.CreatedAt, ID: result.NextCursor.ID})
		if err != nil {
			slog.ErrorContext(ctx, "encode URL list cursor failed", "error", err)
			http.Error(w, "erro interno do servidor", http.StatusInternalServerError)
			return
		}

		// Estou transformando um payload json em uma string para comportar na resposta.
		value := base64.RawURLEncoding.EncodeToString(encoded)
		resp.NextCursor = &value
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "encode listed URLs response failed", "error", err)
	}
}
