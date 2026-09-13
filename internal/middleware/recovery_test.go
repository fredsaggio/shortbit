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

func TestRecoveryHandlesPanic(t *testing.T) {
	originalLogger := slog.Default()

	var logOutput bytes.Buffer

	testLogger := slog.New(
		slog.NewJSONHandler(&logOutput, nil),
	)

	slog.SetDefault(testLogger)

	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected failure")
	})

	handler := middleware.Recovery(nextHandler)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	expectedBody := "erro interno do servidor\n"

	if response.Body.String() != expectedBody {
		t.Errorf(
			"expected body %q, got %q",
			expectedBody,
			response.Body.String(),
		)
	}

	var logEntry struct {
		Message string `json:"msg"`
		Panic   string `json:"panic"`
		Stack   string `json:"stack"`
	}

	if err := json.Unmarshal(logOutput.Bytes(), &logEntry); err != nil {
		t.Fatalf(
			"failed to decode log: %v; output: %q",
			err,
			logOutput.String(),
		)
	}

	if logEntry.Message != "panic recovered" {
		t.Errorf(
			"expected message %q, got %q",
			"panic recovered",
			logEntry.Message,
		)
	}

	if logEntry.Panic != "unexpected failure" {
		t.Errorf(
			"expected panic %q, got %q",
			"unexpected failure",
			logEntry.Panic,
		)
	}

	if logEntry.Stack == "" {
		t.Error("expected stack trace, got empty string")
	}
}
