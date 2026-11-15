package telemetry

import (
	"context"
	"log/slog"
)

// Security events, following the OWASP Logging Cheat Sheet: authentication
// failures, input that hit a rate limit, and changes to credentials or
// accounts. Each is a log line with a fixed "event" name and a counter of
// the same name, so a dashboard can count them and an alert can watch them.
const (
	EventLoginFailed           = "login_failed"
	EventGoogleLoginFailed     = "google_login_failed"
	EventRateLimited           = "rate_limited"
	EventLinkPasswordFailed    = "link_password_failed"
	EventVerificationFailed    = "verification_code_failed"
	EventPasswordChanged       = "password_changed"
	EventPasswordResetComplete = "password_reset_completed"
	EventAccountCreated        = "account_created"
	EventAccountDeleted        = "account_deleted"
)

// securityEventNames lists every event, so the counters start at 0.
var securityEventNames = []string{
	EventLoginFailed, EventGoogleLoginFailed, EventRateLimited, EventLinkPasswordFailed, EventVerificationFailed,
	EventPasswordChanged, EventPasswordResetComplete, EventAccountCreated, EventAccountDeleted,
}

// SecurityEvent logs the event and counts it. attrs must follow the log
// policy (backend/docs/development.md): internal IDs and fixed words only,
// never an email, IP, password, token or code.
func SecurityEvent(ctx context.Context, event string, attrs ...slog.Attr) {
	recordSecurityEvent(ctx, event)
	level := slog.LevelWarn
	switch event {
	case EventPasswordChanged, EventPasswordResetComplete, EventAccountCreated, EventAccountDeleted:
		level = slog.LevelInfo
	}
	slog.LogAttrs(ctx, level, "security event", append([]slog.Attr{slog.String("event", event)}, attrs...)...)
}
