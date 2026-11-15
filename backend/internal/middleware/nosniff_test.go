package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoSniff(t *testing.T) {
	handler := NoSniff(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "url não encontrada", http.StatusNotFound)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}
