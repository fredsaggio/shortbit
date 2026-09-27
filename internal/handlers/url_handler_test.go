package handlers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/repositories"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type urlServiceStub struct {
	createFunc func(context.Context, uuid.UUID, string, models.Visibility, string) (services.CreateURLResult, error)
	listFunc   func(context.Context, uuid.UUID, int, *repositories.URLCursor) (services.ListURLResult, error)
}

func (s urlServiceStub) Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (services.CreateURLResult, error) {
	return s.createFunc(ctx, userID, originalURL, visibility, password)
}

func (s urlServiceStub) List(ctx context.Context, userID uuid.UUID, limit int, cursor *repositories.URLCursor) (services.ListURLResult, error) {
	return s.listFunc(ctx, userID, limit, cursor)
}

func TestURLHandlerCreate(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c405")
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	ctx = ctxval.ContextWithUserID(ctx, userID)

	called := false
	handler := handlers.NewURLHandler(urlServiceStub{
		createFunc: func(gotCtx context.Context, gotUserID uuid.UUID, gotURL string, gotVisibility models.Visibility, gotPassword string) (services.CreateURLResult, error) {
			called = true
			if gotCtx.Value(contextKey{}) != "request-context" {
				t.Error("Create() did not receive request context")
			}
			if gotUserID != userID || gotURL != "https://example.com/article" ||
				gotVisibility != models.VisibilityPrivate || gotPassword != "senha-segura" {
				t.Errorf("Create() arguments = (%s, %q, %q, %q), want authenticated user and request values", gotUserID, gotURL, gotVisibility, gotPassword)
			}
			return services.CreateURLResult{ShortCode: "Ab3dX9", ShortURL: "https://sho.rt/Ab3dX9"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(
		`{"url":"https://example.com/article","visibility":"private","password":"senha-segura"}`,
	)).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.Create(response, req)

	if !called {
		t.Fatal("URLService.Create() was not called")
	}
	if response.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["short_code"] != "Ab3dX9" || body["short_url"] != "https://sho.rt/Ab3dX9" {
		t.Errorf("response = %v, want short code and short URL", body)
	}
	if len(body) != 2 {
		t.Errorf("response fields = %v, want only short_code and short_url", body)
	}
}

func TestURLHandlerCreateRejectsMissingAuthentication(t *testing.T) {
	handler := handlers.NewURLHandler(urlServiceStub{
		createFunc: func(context.Context, uuid.UUID, string, models.Visibility, string) (services.CreateURLResult, error) {
			t.Fatal("Create() called without an authenticated user")
			return services.CreateURLResult{}, nil
		},
	})

	response := httptest.NewRecorder()
	handler.Create(response, httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(
		`{"url":"https://example.com","visibility":"public"}`,
	)))
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestURLHandlerCreateRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body"},
		{name: "malformed JSON", body: `{"url":`},
		{name: "unknown field", body: `{"url":"https://example.com","visibility":"public","user_id":"attacker"}`},
		{name: "wrong field type", body: `{"url":123,"visibility":"public"}`},
		{name: "two objects", body: `{"url":"https://example.com","visibility":"public"} {"url":"https://other.example.com"}`},
		{name: "trailing content", body: `{"url":"https://example.com","visibility":"public"} garbage`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewURLHandler(urlServiceStub{
				createFunc: func(context.Context, uuid.UUID, string, models.Visibility, string) (services.CreateURLResult, error) {
					t.Fatal("Create() called for invalid JSON")
					return services.CreateURLResult{}, nil
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(tt.body))
			req = req.WithContext(ctxval.ContextWithUserID(req.Context(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c406")))
			response := httptest.NewRecorder()
			handler.Create(response, req)

			if response.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d; body = %q", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestURLHandlerCreateMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{name: "unauthenticated", serviceErr: services.ErrUnauthenticated, wantStatus: http.StatusUnauthorized, wantBody: "autenticação necessária\n"},
		{name: "invalid original URL", serviceErr: services.ErrInvalidOriginalURL, wantStatus: http.StatusBadRequest, wantBody: "URL original inválida\n"},
		{name: "URL too long", serviceErr: services.ErrURLTooLong, wantStatus: http.StatusBadRequest, wantBody: "URL original muito longa\n"},
		{name: "own domain", serviceErr: services.ErrURLPointsToShortDomain, wantStatus: http.StatusBadRequest, wantBody: "não é permitido encurtar o próprio domínio\n"},
		{name: "invalid visibility", serviceErr: services.ErrInvalidURLVisibility, wantStatus: http.StatusBadRequest, wantBody: "visibilidade inválida\n"},
		{name: "public URL with password", serviceErr: services.ErrUnexpectedLinkPassword, wantStatus: http.StatusBadRequest, wantBody: "link público não deve ter senha\n"},
		{name: "password too short", serviceErr: services.ErrLinkPasswordTooShort, wantStatus: http.StatusBadRequest, wantBody: "senha do link muito curta\n"},
		{name: "password too long", serviceErr: services.ErrLinkPasswordTooLong, wantStatus: http.StatusBadRequest, wantBody: "senha do link muito longa\n"},
		{name: "wrapped validation error", serviceErr: errors.Join(errors.New("context"), services.ErrInvalidOriginalURL), wantStatus: http.StatusBadRequest, wantBody: "URL original inválida\n"},
		{name: "internal error", serviceErr: errors.New("secret database details"), wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewURLHandler(urlServiceStub{
				createFunc: func(context.Context, uuid.UUID, string, models.Visibility, string) (services.CreateURLResult, error) {
					return services.CreateURLResult{}, tt.serviceErr
				},
			})
			req := httptest.NewRequest(http.MethodPost, "/urls", strings.NewReader(
				`{"url":"https://example.com","visibility":"public"}`,
			))
			req = req.WithContext(ctxval.ContextWithUserID(req.Context(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c407")))
			response := httptest.NewRecorder()
			handler.Create(response, req)

			if response.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", response.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestURLHandlerListPaginatesWithoutExposingPasswordHash(t *testing.T) {
	userID := uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c408")
	createdAt := time.Date(2026, 9, 27, 12, 30, 0, 123456000, time.UTC)
	passwordHash := "private-hash-must-not-appear"
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "request-context")
	ctx = ctxval.ContextWithUserID(ctx, userID)
	calls := 0
	handler := handlers.NewURLHandler(urlServiceStub{listFunc: func(gotCtx context.Context, gotUserID uuid.UUID, limit int, cursor *repositories.URLCursor) (services.ListURLResult, error) {
		calls++
		if gotCtx.Value(contextKey{}) != "request-context" || gotUserID != userID {
			t.Errorf("List() context/user = (%v, %v), want original context and owner", gotCtx, gotUserID)
		}
		if calls == 1 {
			if limit != 20 || cursor != nil {
				t.Errorf("first List() limit/cursor = (%d, %v), want (20, nil)", limit, cursor)
			}
			return services.ListURLResult{
				URLs: []models.URL{{ID: 42, ShortCode: "Ab3dX9", UserID: userID, OriginalURL: "https://example.com/private", Visibility: models.VisibilityPrivate,
					PasswordHash: &passwordHash, ClickCount: 7, CreatedAt: createdAt, UpdatedAt: createdAt}},
				NextCursor: &repositories.URLCursor{CreatedAt: createdAt, ID: 42},
			}, nil
		}
		if limit != 2 || cursor == nil || cursor.ID != 42 || !cursor.CreatedAt.Equal(createdAt) {
			t.Errorf("second List() limit/cursor = (%d, %v), want (2, cursor for ID 42)", limit, cursor)
		}
		return services.ListURLResult{URLs: []models.URL{}}, nil
	}})

	first := httptest.NewRecorder()
	handler.List(first, httptest.NewRequest(http.MethodGet, "/urls", nil).WithContext(ctx))
	if first.Code != http.StatusOK || first.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("first response status/content type = (%d, %q), want (200, application/json)", first.Code, first.Header().Get("Content-Type"))
	}
	if strings.Contains(first.Body.String(), passwordHash) || strings.Contains(first.Body.String(), "password_hash") {
		t.Fatalf("list response exposed password hash: %s", first.Body.String())
	}
	var firstBody struct {
		URLs       []map[string]any `json:"urls"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if len(firstBody.URLs) != 1 || firstBody.URLs[0]["short_code"] != "Ab3dX9" || firstBody.URLs[0]["original_url"] != "https://example.com/private" ||
		firstBody.URLs[0]["visibility"] != "private" || firstBody.URLs[0]["click_count"] != float64(7) {
		t.Errorf("first response URLs = %v, want private link metadata", firstBody.URLs)
	}
	if firstBody.NextCursor == nil || *firstBody.NextCursor == "" {
		t.Fatal("first response has no next cursor")
	}
	cursorJSON, err := base64.RawURLEncoding.DecodeString(*firstBody.NextCursor)
	if err != nil {
		t.Fatalf("next cursor is not Base64URL: %v", err)
	}
	var cursorFields map[string]any
	if err := json.Unmarshal(cursorJSON, &cursorFields); err != nil || cursorFields["id"] != float64(42) || cursorFields["created_at"] != createdAt.Format(time.RFC3339Nano) {
		t.Errorf("decoded cursor = %v, error = %v, want created_at and ID 42", cursorFields, err)
	}

	second := httptest.NewRecorder()
	handler.List(second, httptest.NewRequest(http.MethodGet, "/urls?limit=2&cursor="+*firstBody.NextCursor, nil).WithContext(ctx))
	if second.Code != http.StatusOK {
		t.Fatalf("second response status = %d, want 200; body = %s", second.Code, second.Body.String())
	}
	var secondBody struct {
		URLs       []map[string]any `json:"urls"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(secondBody.URLs) != 0 || secondBody.NextCursor != nil || calls != 2 {
		t.Errorf("second response = %+v, calls = %d, want empty URLs, nil cursor and two calls", secondBody, calls)
	}
}

func TestURLHandlerListRejectsMissingAuthentication(t *testing.T) {
	handler := handlers.NewURLHandler(urlServiceStub{listFunc: func(context.Context, uuid.UUID, int, *repositories.URLCursor) (services.ListURLResult, error) {
		t.Fatal("List() called without authentication")
		return services.ListURLResult{}, nil
	}})
	response := httptest.NewRecorder()
	handler.List(response, httptest.NewRequest(http.MethodGet, "/urls", nil))
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", response.Code)
	}
}

func TestURLHandlerListRejectsMalformedParameters(t *testing.T) {
	badCursor := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	tests := []struct {
		name string
		url  string
	}{
		{name: "non-numeric limit", url: "/urls?limit=abc"},
		{name: "empty limit", url: "/urls?limit="},
		{name: "duplicate limit", url: "/urls?limit=2&limit=3"},
		{name: "invalid Base64 cursor", url: "/urls?cursor=invalid!"},
		{name: "empty cursor", url: "/urls?cursor="},
		{name: "missing cursor fields", url: "/urls?cursor=" + badCursor},
		{name: "duplicate cursor", url: "/urls?cursor=x&cursor=y"},
		{name: "invalid query encoding", url: "/urls"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewURLHandler(urlServiceStub{listFunc: func(context.Context, uuid.UUID, int, *repositories.URLCursor) (services.ListURLResult, error) {
				t.Fatal("List() called with malformed parameters")
				return services.ListURLResult{}, nil
			}})
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			if tt.name == "invalid query encoding" {
				req.URL.RawQuery = "cursor=%"
			}
			req = req.WithContext(ctxval.ContextWithUserID(req.Context(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c409")))
			response := httptest.NewRecorder()
			handler.List(response, req)
			if response.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestURLHandlerListMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantBody   string
	}{
		{name: "invalid limit", serviceErr: services.ErrInvalidURLListLimit, wantStatus: http.StatusBadRequest, wantBody: "limite inválido\n"},
		{name: "unauthenticated", serviceErr: services.ErrUnauthenticated, wantStatus: http.StatusUnauthorized, wantBody: "autenticação necessária\n"},
		{name: "internal", serviceErr: errors.New("secret database details"), wantStatus: http.StatusInternalServerError, wantBody: "erro interno do servidor\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewURLHandler(urlServiceStub{listFunc: func(context.Context, uuid.UUID, int, *repositories.URLCursor) (services.ListURLResult, error) {
				return services.ListURLResult{}, tt.serviceErr
			}})
			req := httptest.NewRequest(http.MethodGet, "/urls?limit=21", nil)
			req = req.WithContext(ctxval.ContextWithUserID(req.Context(), uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c410")))
			response := httptest.NewRecorder()
			handler.List(response, req)
			if response.Code != tt.wantStatus || response.Body.String() != tt.wantBody {
				t.Errorf("response = (%d, %q), want (%d, %q)", response.Code, response.Body.String(), tt.wantStatus, tt.wantBody)
			}
		})
	}
}
