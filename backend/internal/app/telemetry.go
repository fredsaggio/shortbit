package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/fredsaggio/url-shortener/internal/logging"
	"github.com/fredsaggio/url-shortener/internal/telemetry"
)

const telemetryFlushTimeout = 5 * time.Second

// StartTelemetry sends metrics and logs out when OTEL_EXPORTER_OTLP_ENDPOINT
// is set and makes the default logger write to both stdout and the
// collector. The returned function flushes what is buffered; call it on exit.
func StartTelemetry(ctx context.Context, getEnv func(string) string, service string) (func(), error) {
	t, err := telemetry.Setup(ctx, getEnv, service)
	if err != nil {
		return nil, err
	}
	if !t.Enabled() {
		slog.Info("telemetry off; set OTEL_EXPORTER_OTLP_ENDPOINT to send metrics and logs")
		return func() {}, nil
	}

	slog.SetDefault(logging.NewLogger(os.Stdout, t.LoggerProvider))
	slog.Info("telemetry on: sending metrics and logs over OTLP")

	return func() {
		// The run context is already canceled on shutdown.
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), telemetryFlushTimeout)
		defer cancel()
		if err := t.Shutdown(flushCtx); err != nil {
			slog.Warn("flush telemetry failed", "error", err)
		}
	}, nil
}
