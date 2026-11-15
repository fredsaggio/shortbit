// Package logging builds the application logger. Code should never log
// personal data or secrets in the first place; this is the safety net for
// text it does not control, such as a third-party error message or a
// recovered panic value.
package logging

import (
	"context"
	"io"
	"log/slog"
	"regexp"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	otellog "go.opentelemetry.io/otel/log"
)

var (
	// Anything shaped like an email address.
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// Credentials inside a URL: scheme://user:password@host.
	urlCredentialsPattern = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)[^/\s:@]+:[^/\s@]+@`)
)

const (
	redactedEmail       = "[email]"
	redactedCredentials = "[credentials]@"
)

// Redact removes email addresses and URL credentials from text. Credentials
// go first: user:password@host would otherwise read as an email.
func Redact(text string) string {
	text = urlCredentialsPattern.ReplaceAllString(text, "${1}"+redactedCredentials)
	return emailPattern.ReplaceAllString(text, redactedEmail)
}

// NewJSONLogger writes JSON lines to w with every string and error value
// passed through Redact.
func NewJSONLogger(w io.Writer) *slog.Logger {
	return NewLogger(w, nil)
}

// NewLogger writes JSON lines to w and, when provider is not nil, also sends
// every record to it (OpenTelemetry logs, see the telemetry package). Both
// outputs go through Redact.
func NewLogger(w io.Writer, provider otellog.LoggerProvider) *slog.Logger {
	stdout := slog.NewJSONHandler(w, &slog.HandlerOptions{ReplaceAttr: redactAttr})
	if provider == nil {
		return slog.New(stdout)
	}
	exported := &redactingHandler{next: otelslog.NewHandler(loggerName, otelslog.WithLoggerProvider(provider))}
	return slog.New(slog.NewMultiHandler(stdout, exported))
}

const loggerName = "github.com/fredsaggio/url-shortener"

// redactingHandler applies Redact to a handler that has no ReplaceAttr hook.
type redactingHandler struct {
	next slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, Redact(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactNested(attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		redacted[i] = redactNested(attr)
	}
	return &redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

// redactNested redacts an attribute and, for a group, every attribute in it.
func redactNested(attr slog.Attr) slog.Attr {
	value := attr.Value.Resolve()
	if value.Kind() != slog.KindGroup {
		return redactAttr(nil, slog.Attr{Key: attr.Key, Value: value})
	}
	group := value.Group()
	redacted := make([]any, len(group))
	for i, member := range group {
		redacted[i] = redactNested(member)
	}
	return slog.Group(attr.Key, redacted...)
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	value := attr.Value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return slog.String(attr.Key, Redact(value.String()))
	case slog.KindAny:
		switch v := value.Any().(type) {
		case error:
			return slog.String(attr.Key, Redact(v.Error()))
		case string:
			return slog.String(attr.Key, Redact(v))
		case interface{ String() string }:
			return slog.String(attr.Key, Redact(v.String()))
		}
	}
	return attr
}
