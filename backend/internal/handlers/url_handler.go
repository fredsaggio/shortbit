package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/fredsaggio/url-shortener/internal/problem"
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
	GetByShortCode(ctx context.Context, userID uuid.UUID, shortCode string) (models.URL, error)
	Update(ctx context.Context, userID uuid.UUID, shortCode string, update services.URLUpdate) (models.URL, error)
	Delete(ctx context.Context, userID uuid.UUID, shortCode string) error
}

type RedirectService interface {
	Resolve(ctx context.Context, shortCode, token string, source models.ClickSource) (services.RedirectResult, error)
}

type createURLRequest struct {
	URL        string            `json:"url"`
	Visibility models.Visibility `json:"visibility"`
	Password   string            `json:"password"`
}

// Absent fields stay as they are; on a private link an empty password keeps
// the current one.
type updateURLRequest struct {
	URL        *string            `json:"url"`
	Visibility *models.Visibility `json:"visibility"`
	Password   *string            `json:"password"`
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
	serv     URLService
	redirect RedirectService
}

func NewURLHandler(serv URLService, redirect RedirectService) *URLHandler {
	return &URLHandler{serv: serv, redirect: redirect}
}

func (h *URLHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctxval.UserIDFromContext(ctx)

	if !ok {
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	var req createURLRequest

	if !decodeSingleJSON(w, r, &req) {
		return
	}

	result, err := h.serv.Create(ctx, userID, req.URL, req.Visibility, req.Password)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")

		default:
			if code, message, ok := linkInputError(err); ok {
				problem.Write(w, r, http.StatusBadRequest, code, message)
				return
			}
			slog.ErrorContext(ctx, "create URL failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
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
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	limit := defaultURLListLimit
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		problem.Write(w, r, http.StatusBadRequest, "invalid_query", "parâmetros inválidos")
		return
	}
	if values, present := query["limit"]; present {
		if len(values) != 1 {
			problem.Write(w, r, http.StatusBadRequest, "invalid_limit", "limite inválido")
			return
		}
		limit, err = strconv.Atoi(values[0])
		if err != nil {
			problem.Write(w, r, http.StatusBadRequest, "invalid_limit", "limite inválido")
			return
		}
	}

	var cursor *repositories.URLCursor
	if values, present := query["cursor"]; present {
		// O parâmetro pode vir mais de uma vez, tipo: /urls?cursor=abc&cursor=def
		if len(values) != 1 {
			problem.Write(w, r, http.StatusBadRequest, "invalid_cursor", "cursor inválido")
			return
		}
		decoded, err := base64.RawURLEncoding.DecodeString(values[0])
		var payload urlCursorPayload
		if err != nil || json.Unmarshal(decoded, &payload) != nil || payload.ID <= 0 || payload.CreatedAt.IsZero() {
			problem.Write(w, r, http.StatusBadRequest, "invalid_cursor", "cursor inválido")
			return
		}
		cursor = &repositories.URLCursor{CreatedAt: payload.CreatedAt, ID: payload.ID}
	}

	result, err := h.serv.List(ctx, userID, limit, cursor)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		case errors.Is(err, services.ErrInvalidURLListLimit):
			problem.Write(w, r, http.StatusBadRequest, "invalid_limit", "limite inválido")
		default:
			slog.ErrorContext(ctx, "list URLs failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
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
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
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

func (h *URLHandler) GetByShortCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	shortCode := r.PathValue("code")
	userID, ok := ctxval.UserIDFromContext(ctx)

	if !ok {
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "usuário não autenticado")
		return
	}

	url, err := h.serv.GetByShortCode(ctx, userID, shortCode)

	if err != nil {
		if errors.Is(err, services.ErrURLNotFound) {
			problem.Write(w, r, http.StatusNotFound, "link_not_found", "url não encontrada")
			return
		}
		slog.ErrorContext(ctx, "get url by shortcode", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	resp := listedURLResponse{
		ShortCode:   url.ShortCode,
		OriginalURL: url.OriginalURL,
		Visibility:  url.Visibility,
		ClickCount:  url.ClickCount,
		CreatedAt:   url.CreatedAt,
		UpdatedAt:   url.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		slog.ErrorContext(ctx, "encode url failed", "error", err)
	}
}

func (h *URLHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	shortCode := r.PathValue("code")
	w.Header().Set("Cache-Control", "no-store")

	var token string
	cookie, err := r.Cookie(linkAccessSessionCookieName)
	if err == nil {
		token = cookie.Value
	} else if !errors.Is(err, http.ErrNoCookie) {
		slog.ErrorContext(ctx, "read link access cookie failed", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}

	result, err := h.redirect.Resolve(ctx, shortCode, token, clickSourceOf(r))

	if err != nil {
		if errors.Is(err, services.ErrURLNotFound) {
			problem.Write(w, r, http.StatusNotFound, "link_not_found", "url não encontrada")
			return
		}
		if errors.Is(err, services.ErrClickNotRecorded) {
			slog.ErrorContext(ctx, "record click failed", "error", err)
			problem.Write(w, r, http.StatusServiceUnavailable, "service_unavailable", "serviço indisponível")
			return
		}
		slog.ErrorContext(ctx, "resolve url", "error", err)
		problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		return
	}
	if result.PasswordRequired {
		renderPrivateLinkPasswordPage(w, r, shortCode, "", http.StatusOK)
		return
	}

	http.Redirect(w, r, result.OriginalURL, http.StatusFound)
}

// linkInputError maps the validation errors shared by creating and editing a
// link to a problem code and message.
func linkInputError(err error) (code, message string, ok bool) {
	switch {
	case errors.Is(err, services.ErrInvalidOriginalURL):
		return "invalid_url", "URL original inválida", true
	case errors.Is(err, services.ErrURLTooLong):
		return "url_too_long", "URL original muito longa", true
	case errors.Is(err, services.ErrURLPointsToShortDomain):
		return "url_points_to_short_domain", "não é permitido encurtar o próprio domínio", true
	case errors.Is(err, services.ErrInvalidURLVisibility):
		return "invalid_visibility", "visibilidade inválida", true
	case errors.Is(err, services.ErrUnexpectedLinkPassword):
		return "unexpected_link_password", "link público não deve ter senha", true
	case errors.Is(err, services.ErrLinkPasswordTooShort):
		return "link_password_too_short", "senha do link muito curta", true
	case errors.Is(err, services.ErrLinkPasswordTooLong):
		return "link_password_too_long", "senha do link muito longa", true
	case errors.Is(err, services.ErrLinkPasswordRequired):
		return "link_password_required", "informe a senha do link", true
	case errors.Is(err, services.ErrEmptyURLUpdate):
		return "empty_update", "informe o que alterar", true
	}
	return "", "", false
}

// Update handles PATCH /urls/{code}: the owner changes the destination, the
// visibility or the password, and gets the link back as GET returns it.
func (h *URLHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	var req updateURLRequest
	if !decodeSingleJSON(w, r, &req) {
		return
	}

	updated, err := h.serv.Update(ctx, userID, r.PathValue("code"), services.URLUpdate{
		OriginalURL: req.URL,
		Visibility:  req.Visibility,
		Password:    req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		case errors.Is(err, services.ErrURLNotFound):
			problem.Write(w, r, http.StatusNotFound, "link_not_found", "url não encontrada")
		default:
			if code, message, ok := linkInputError(err); ok {
				problem.Write(w, r, http.StatusBadRequest, code, message)
				return
			}
			slog.ErrorContext(ctx, "update URL failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	resp := listedURLResponse{
		ShortCode:   updated.ShortCode,
		OriginalURL: updated.OriginalURL,
		Visibility:  updated.Visibility,
		ClickCount:  updated.ClickCount,
		CreatedAt:   updated.CreatedAt,
		UpdatedAt:   updated.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		slog.ErrorContext(ctx, "encode updated URL failed", "error", err)
	}
}

// Delete handles DELETE /urls/{code}. Another user's link answers 404, as GET
// does, so the API never reveals whose a code is.
func (h *URLHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	if err := h.serv.Delete(ctx, userID, r.PathValue("code")); err != nil {
		switch {
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		case errors.Is(err, services.ErrURLNotFound):
			problem.Write(w, r, http.StatusNotFound, "link_not_found", "url não encontrada")
		default:
			slog.ErrorContext(ctx, "delete URL failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
