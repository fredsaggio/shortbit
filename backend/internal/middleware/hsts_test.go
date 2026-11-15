package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStrictTransportSecurity(t *testing.T) {
	handler := StrictTransportSecurity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "muitas requisições", http.StatusTooManyRequests)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := response.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Errorf("Strict-Transport-Security = %q, want one year with subdomains", got)
	}
}
