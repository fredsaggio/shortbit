package email

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/resend/resend-go/v4"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestResendSenderSendPasswordResetCode(t *testing.T) {
	const code = "00123456"
	called := false
	sender := NewResendSender("test-api-key", "noreply@example.com")
	sender.client = resend.NewCustomClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Errorf("request = %s %s, want POST /emails", r.Method, r.URL.Path)
		}

		var request resend.SendEmailRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode email request: %v", err)
		}
		if request.From != "noreply@example.com" || len(request.To) != 1 || request.To[0] != "user@example.com" {
			t.Errorf("email addresses = (%q, %v), want sender and user", request.From, request.To)
		}
		if request.Subject != "Código para redefinir sua senha" {
			t.Errorf("subject = %q", request.Subject)
		}
		if !strings.Contains(request.Text, code) || !strings.Contains(request.Text, "ignore este email") {
			t.Errorf("email text does not contain code and unsolicited-request guidance: %q", request.Text)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"test-email-id"}`)),
			Request:    r,
		}, nil
	})}, "test-api-key")

	if err := sender.SendPasswordResetCode(t.Context(), "user@example.com", code); err != nil {
		t.Fatalf("SendPasswordResetCode() error = %v", err)
	}
	if !called {
		t.Fatal("Resend endpoint was not called")
	}
}

func TestResendSenderSendPasswordResetCodeReturnsProviderError(t *testing.T) {
	sender := NewResendSender("test-api-key", "noreply@example.com")
	sender.client = resend.NewCustomClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"message":"provider unavailable"}`)),
			Request:    r,
		}, nil
	})}, "test-api-key")
	if err := sender.SendPasswordResetCode(t.Context(), "user@example.com", "12345678"); err == nil {
		t.Fatal("SendPasswordResetCode() error = nil, want provider error")
	}
}
