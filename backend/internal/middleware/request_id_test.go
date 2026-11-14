package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/middleware"
)

func TestRequestIDAddsSameIDToResponseAndContext(t *testing.T) {
	var contextRequestID string

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextRequestID = middleware.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequestID(nextHandler)
	request := httptest.NewRequest(http.MethodGet, "/urls", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	headerRequestID := response.Header().Get("X-Request-ID")

	if headerRequestID == "" {
		t.Fatal("expected X-Request-ID response header, got an empty value")
	}

	if contextRequestID == "" {
		t.Fatal("expected request ID in request context, got an empty value")
	}

	if contextRequestID != headerRequestID {
		t.Errorf(
			"expected context request ID %q to match header request ID %q",
			contextRequestID,
			headerRequestID,
		)
	}
}

func TestRequestIDGeneratesDifferentIDForEachRequest(t *testing.T) {
	var generatedIDs []string

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		generatedIDs = append(
			generatedIDs,
			middleware.RequestIDFromContext(r.Context()),
		)
		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.RequestID(nextHandler)

	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/urls", nil)
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)
	}

	if len(generatedIDs) != 2 {
		t.Fatalf("expected 2 generated request IDs, got %d", len(generatedIDs))
	}

	if generatedIDs[0] == generatedIDs[1] {
		t.Errorf("expected different request IDs, got %q twice", generatedIDs[0])
	}
}
