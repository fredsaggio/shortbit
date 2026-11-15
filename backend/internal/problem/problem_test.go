package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/problem"
)

func TestWrite(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Length", "99")

	problem.Write(recorder, http.StatusBadRequest, "password_too_short", "senha muito curta")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q", got)
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
