package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/middleware"
)

func TestAccessLog(t *testing.T) {
	originalLogger := slog.Default()

	var logOutput bytes.Buffer

	testLogger := slog.New(slog.NewJSONHandler(&logOutput, nil))

	slog.SetDefault(testLogger)

	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	const responseBody = "created"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	})

	handler := middleware.AccessLog(nextHandler)

	request := httptest.NewRequest(http.MethodPost, "/urls", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	if response.Body.String() != responseBody {
		t.Errorf("expected body %q, got %q", responseBody, response.Body.String())
	}

	var logEntry struct {
		Message       string `json:"msg"`
		Method        string `json:"method"`
		Path          string `json:"path"`
		Status        int    `json:"status"`
		ResponseBytes int    `json:"response_bytes"`
	}

	if err := json.Unmarshal(logOutput.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to decode log: %v; output: %q", err, logOutput.String())
	}

	if logEntry.Message != "http request" {
		t.Errorf(
			"expected message %q, got %q",
			"http request",
			logEntry.Message,
		)
	}

	if logEntry.Method != http.MethodPost {
		t.Errorf(
			"expected method %q, got %q",
			http.MethodPost,
			logEntry.Method,
		)
	}

	if logEntry.Path != "/urls" {
		t.Errorf(
			"expected path %q, got %q",
			"/urls",
			logEntry.Path,
		)
	}

	if logEntry.Status != http.StatusCreated {
		t.Errorf(
			"expected logged status %d, got %d",
			http.StatusCreated,
			logEntry.Status,
		)
	}

	if logEntry.ResponseBytes != len(responseBody) {
		t.Errorf(
			"expected %d response bytes, got %d",
			len(responseBody),
			logEntry.ResponseBytes,
		)
	}

}
