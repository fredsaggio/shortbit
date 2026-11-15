package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/middleware"
	"github.com/fredsaggio/url-shortener/internal/problem/problemtest"
)

func TestDecodeSingleJSON(t *testing.T) {
	// The real global limit wraps the body, as in the router.
	handler := middleware.LimitRequestBody(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var target struct {
			Email string `json:"email"`
		}
		if decodeSingleJSON(w, r, &target) {
			w.WriteHeader(http.StatusNoContent)
		}
	}))

	huge := strings.Repeat("a", 300*1024)
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{name: "one object", body: `{"email":"ana@example.com"}`, wantStatus: http.StatusNoContent},
		{name: "malformed", body: `{"email":`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body", wantDetail: "corpo da requisição inválido"},
		{name: "unknown field", body: `{"name":"Ana"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body", wantDetail: "corpo da requisição inválido"},
		{name: "two objects", body: `{"email":"a@b.co"}{"email":"c@d.co"}`, wantStatus: http.StatusBadRequest, wantCode: "invalid_body", wantDetail: "corpo da requisição deve conter exatamente um objeto JSON"},
		{name: "too large inside the object", body: `{"email":"` + huge + `"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large", wantDetail: "corpo da requisição grande demais"},
		{name: "too large after the object", body: `{"email":"a@b.co"}` + strings.Repeat(" ", 300*1024) + `{}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: "body_too_large", wantDetail: "corpo da requisição grande demais"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(tt.body)))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if tt.wantCode == "" {
				return
			}
			details, ok := problemtest.Decode(recorder.Body.String())
			if !ok || details.Code != tt.wantCode || details.Detail != tt.wantDetail {
				t.Errorf("problem = %+v, want %s %q", details, tt.wantCode, tt.wantDetail)
			}
		})
	}
}
