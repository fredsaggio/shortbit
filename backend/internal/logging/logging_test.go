package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := map[string]string{
		"send to ana.souza+news@example.com.br failed":                "send to [email] failed",
		"dial amqps://shortbit:s3cr3t@host.cloudamqp.com/vh":          "dial amqps://[credentials]@host.cloudamqp.com/vh",
		"rediss://default:Abc123@gorgeous-reptile.upstash.io:6379":    "rediss://[credentials]@gorgeous-reptile.upstash.io:6379",
		"postgresql://owner:pw@ep-x.neon.tech/neondb?sslmode=require": "postgresql://[credentials]@ep-x.neon.tech/neondb?sslmode=require",
		"click event for missing URL discarded":                       "click event for missing URL discarded",
		"https://example.com/path?x=1":                                "https://example.com/path?x=1",
	}
	for input, want := range tests {
		if got := Redact(input); got != want {
			t.Errorf("Redact(%q) = %q, want %q", input, got, want)
		}
	}
}

type stringer struct{}

func (stringer) String() string { return "user bia@example.com" }

func TestLoggerRedactsStringsErrorsAndPanicValues(t *testing.T) {
	var out bytes.Buffer
	logger := NewJSONLogger(&out)

	cause := errors.New(`resend: 422 invalid "to": ana@example.com`)
	logger.Error("send failed",
		"error", fmt.Errorf("send password reset code: %w", cause),
		"note", "owner ana@example.com",
		"panic", stringer{},
		"dsn", "amqps://user:pass@host/vh",
		"count", 3,
	)

	line := out.String()
	for _, leaked := range []string{"ana@example.com", "bia@example.com", "user:pass"} {
		if strings.Contains(line, leaked) {
			t.Errorf("log line leaked %q: %s", leaked, line)
		}
	}
	for _, kept := range []string{`"count":3`, "send password reset code", `[email]`, `[credentials]@host`} {
		if !strings.Contains(line, kept) {
			t.Errorf("log line lost %q: %s", kept, line)
		}
	}
}

func TestRedactingHandlerCoversAttrsGroupsAndMessages(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(&redactingHandler{next: slog.NewJSONHandler(&out, nil)}).
		With("owner", "ana@example.com")

	logger.Info("sent to bia@example.com",
		slog.Group("request", slog.String("dsn", "amqps://user:pass@host/vh")),
		"error", errors.New("bounce for caio@example.com"),
		"count", 2,
	)

	line := out.String()
	for _, leaked := range []string{"ana@example.com", "bia@example.com", "caio@example.com", "user:pass"} {
		if strings.Contains(line, leaked) {
			t.Errorf("exported log leaked %q: %s", leaked, line)
		}
	}
	for _, kept := range []string{`"msg":"sent to [email]"`, `"owner":"[email]"`, `"request":{"dsn":"amqps://[credentials]@host/vh"}`, `"count":2`} {
		if !strings.Contains(line, kept) {
			t.Errorf("exported log lost %s: %s", kept, line)
		}
	}
}
