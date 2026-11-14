package middleware_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/middleware"
)

const requestBodyLimitBytes = 250 * 1024

func TestLimitRequestBodyAllowsBodyAtLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), requestBodyLimitBytes)
	nextHandlerCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true

		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}

		if !bytes.Equal(got, payload) {
			t.Errorf("request body length = %d, want %d", len(got), len(payload))
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := middleware.LimitRequestBody(nextHandler)
	request := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(payload))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if !nextHandlerCalled {
		t.Fatal("expected the middleware to call the next handler")
	}

	if response.Code != http.StatusNoContent {
		t.Errorf("status code = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestLimitRequestBodyRejectsBodyOverLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), requestBodyLimitBytes+1)
	nextHandlerCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true

		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("expected reading an oversized request body to fail")
		}

		var maxBytesError *http.MaxBytesError
		if !errors.As(err, &maxBytesError) {
			t.Fatalf("read request body error = %T %v, want *http.MaxBytesError", err, err)
		}

		if maxBytesError.Limit != requestBodyLimitBytes {
			t.Errorf(
				"MaxBytesError limit = %d, want %d",
				maxBytesError.Limit,
				requestBodyLimitBytes,
			)
		}

		http.Error(
			w,
			"request body is too large",
			http.StatusRequestEntityTooLarge,
		)
	})

	handler := middleware.LimitRequestBody(nextHandler)
	request := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(payload))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if !nextHandlerCalled {
		t.Fatal("expected the middleware to call the next handler")
	}

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"status code = %d, want %d",
			response.Code,
			http.StatusRequestEntityTooLarge,
		)
	}
}
