// Package logging builds the application logger. Code should never log
// personal data or secrets in the first place; this is the safety net for
// text it does not control, such as a third-party error message or a
// recovered panic value.
package logging

import (
	"io"
	"log/slog"
	"regexp"
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
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{ReplaceAttr: redactAttr}))
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
