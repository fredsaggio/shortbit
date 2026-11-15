package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/models"
	"github.com/fredsaggio/url-shortener/internal/services"
)

var linkOwner = uuid.MustParse("01991f29-7c22-7ab3-a395-4d402f09c501")

func authedRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(ctxval.ContextWithUserID(t.Context(), linkOwner), method, path, strings.NewReader(body))
	return request
}

func TestURLHandlerUpdateSendsOnlyTheGivenFields(t *testing.T) {
	var got services.URLUpdate
	handler := newURLHandler(urlServiceStub{
		updateFunc: func(_ context.Context, userID uuid.UUID, shortCode string, update services.URLUpdate) (models.URL, error) {
			if userID != linkOwner || shortCode != "Ab3dX9" {
				t.Errorf("Update(%v, %q), want the owner and the path code", userID, shortCode)
			}
			got = update
			return models.URL{ShortCode: "Ab3dX9", OriginalURL: "https://example.com/new", Visibility: models.VisibilityPrivate, ClickCount: 3, CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(60, 0).UTC()}, nil
		},
	})

	request := authedRequest(t, http.MethodPatch, "/urls/Ab3dX9", `{"visibility":"private","password":"senha-do-link"}`)
	request.SetPathValue("code", "Ab3dX9")
	response := httptest.NewRecorder()
	handler.Update(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %q", response.Code, response.Body.String())
	}
	if got.OriginalURL != nil || got.Visibility == nil || *got.Visibility != models.VisibilityPrivate || got.Password == nil || *got.Password != "senha-do-link" {
		t.Errorf("update = %+v, want visibility and password only", got)
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["original_url"] != "https://example.com/new" || body["visibility"] != "private" {
		t.Errorf("body = %v, want the updated link", body)
	}
	if _, leaked := body["password"]; leaked {
		t.Error("response leaked the password")
	}
}

func TestURLHandlerUpdateMapsErrors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		err    error
		status int
		want   string
	}{
		{name: "unknown field", body: `{"destino":"x"}`, status: http.StatusBadRequest, want: "corpo da requisição inválido"},
		{name: "two objects", body: `{} {}`, status: http.StatusBadRequest, want: "exatamente um objeto JSON"},
		{name: "nothing to change", body: `{}`, err: services.ErrEmptyURLUpdate, status: http.StatusBadRequest, want: "informe o que alterar"},
		{name: "private without password", body: `{"visibility":"private"}`, err: services.ErrLinkPasswordRequired, status: http.StatusBadRequest, want: "informe a senha do link"},
		{name: "invalid destination", body: `{"url":"ftp://x"}`, err: services.ErrInvalidOriginalURL, status: http.StatusBadRequest, want: "URL original inválida"},
		{name: "someone else's link", body: `{"url":"https://example.com"}`, err: services.ErrURLNotFound, status: http.StatusNotFound, want: "url não encontrada"},
		{name: "infrastructure", body: `{"url":"https://example.com"}`, err: errors.New("database unavailable"), status: http.StatusInternalServerError, want: "erro interno do servidor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newURLHandler(urlServiceStub{
				updateFunc: func(context.Context, uuid.UUID, string, services.URLUpdate) (models.URL, error) {
					return models.URL{}, tt.err
				},
			})

			request := authedRequest(t, http.MethodPatch, "/urls/Ab3dX9", tt.body)
			request.SetPathValue("code", "Ab3dX9")
			response := httptest.NewRecorder()
			handler.Update(response, request)

			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.want) {
				t.Errorf("got %d %q, want %d containing %q", response.Code, response.Body.String(), tt.status, tt.want)
			}
		})
	}
}

func TestURLHandlerDelete(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "deleted", status: http.StatusNoContent},
		{name: "someone else's link", err: services.ErrURLNotFound, status: http.StatusNotFound},
		{name: "infrastructure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newURLHandler(urlServiceStub{
				deleteFunc: func(_ context.Context, userID uuid.UUID, shortCode string) error {
					if userID != linkOwner || shortCode != "Ab3dX9" {
						t.Errorf("Delete(%v, %q), want the owner and the path code", userID, shortCode)
					}
					return tt.err
				},
			})

			request := authedRequest(t, http.MethodDelete, "/urls/Ab3dX9", "")
			request.SetPathValue("code", "Ab3dX9")
			response := httptest.NewRecorder()
			handler.Delete(response, request)

			if response.Code != tt.status {
				t.Errorf("status = %d, want %d", response.Code, tt.status)
			}
		})
	}
}

func TestURLHandlerEditRequiresAuthentication(t *testing.T) {
	handler := newURLHandler(urlServiceStub{})

	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		request := httptest.NewRequestWithContext(t.Context(), method, "/urls/Ab3dX9", strings.NewReader(`{}`))
		response := httptest.NewRecorder()
		if method == http.MethodPatch {
			handler.Update(response, request)
		} else {
			handler.Delete(response, request)
		}
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", method, response.Code)
		}
	}
}
