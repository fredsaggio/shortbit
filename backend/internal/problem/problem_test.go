package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/problem"
)

const browserAccept = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

func request(accept string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/Ab3dX9", nil)
	if accept != "" {
		r.Header.Set("Accept", accept)
	}
	return r
}

func TestWriteSendsAProblemDocumentToPrograms(t *testing.T) {
	for _, accept := range []string{"application/json", "*/*", ""} {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Content-Length", "99")

		problem.Write(recorder, request(accept), http.StatusBadRequest, "password_too_short", "senha muito curta")

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("Accept %q: status = %d, want 400", accept, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
			t.Errorf("Accept %q: Content-Type = %q", accept, got)
		}
		if recorder.Header().Get("Content-Length") != "" {
			t.Error("stale Content-Length kept")
		}

		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		want := map[string]any{"type": "about:blank", "title": "Bad Request", "status": float64(400), "detail": "senha muito curta", "code": "password_too_short"}
		for key, value := range want {
			if body[key] != value {
				t.Errorf("%s = %v, want %v", key, body[key], value)
			}
		}
		if len(body) != len(want) {
			t.Errorf("body has %d fields, want %d: %v", len(body), len(want), body)
		}
	}
}

func TestWriteShowsPeopleAPageInsteadOfJSON(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		code     string
		detail   string
		want     []string
		mustSkip []string
	}{
		{
			name: "deleted link", status: http.StatusNotFound, code: "link_not_found", detail: "url não encontrada",
			want:     []string{"Link não encontrado", "Este link não existe ou foi apagado", `href="/"`},
			mustSkip: []string{"url não encontrada"},
		},
		{
			name: "server error hides the detail", status: http.StatusInternalServerError, code: "something_new", detail: "pq: connection refused",
			want:     []string{"Algo deu errado", "Tente de novo em instantes"},
			mustSkip: []string{"pq:", "connection refused"},
		},
		{
			name: "sign in", status: http.StatusUnauthorized, code: "authentication_required", detail: "autenticação necessária",
			want: []string{"Entre na sua conta", `href="/entrar"`},
		},
		{
			name: "other client error uses its message as a sentence", status: http.StatusBadRequest, code: "invalid_url", detail: "URL original inválida",
			want: []string{"Não foi possível continuar", "URL original inválida."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			problem.Write(recorder, request(browserAccept), tt.status, tt.code, tt.detail)

			if recorder.Code != tt.status {
				t.Errorf("status = %d, want %d", recorder.Code, tt.status)
			}
			if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want HTML", got)
			}
			if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Error("page without a restrictive Content-Security-Policy")
			}
			if recorder.Header().Get("Vary") != "Accept" {
				t.Errorf("Vary = %q, want Accept", recorder.Header().Get("Vary"))
			}
			body := recorder.Body.String()
			if strings.Contains(body, `"code"`) || strings.Contains(body, tt.code) {
				t.Errorf("page leaked the problem document: %s", body)
			}
			for _, text := range tt.want {
				if !strings.Contains(body, text) {
					t.Errorf("page missing %q", text)
				}
			}
			for _, text := range tt.mustSkip {
				if strings.Contains(body, text) {
					t.Errorf("page shows %q", text)
				}
			}
		})
	}
}
