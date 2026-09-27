package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type urlServiceStub struct {
	createFunc func(context.Context, uuid.UUID, string, models.Visibility, string) (services.CreateURLResult, error)
}

func (s urlServiceStub) Create(ctx context.Context, userID uuid.UUID, originalURL string, visibility models.Visibility, password string) (services.CreateURLResult, error) {
	return s.createFunc(ctx, userID, originalURL, visibility, password)
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
