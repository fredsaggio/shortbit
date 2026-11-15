package telemetry_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/logging"
	"github.com/fredsaggio/url-shortener/internal/telemetry"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestSetupIsOffWithoutAnEndpoint(t *testing.T) {
	tel, err := telemetry.Setup(t.Context(), func(string) string { return "" }, "shortbit-test")
	if err != nil || tel.Enabled() {
		t.Fatalf("Setup() = enabled %v, %v; want off without error", tel.Enabled(), err)
	}
	if err := tel.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

// A fake collector receives what the exporters send over OTLP/HTTP.
func TestSetupSendsMetricsAndLogsOverOTLP(t *testing.T) {
	var mu sync.Mutex
	received := map[string][]string{}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received[r.URL.Path] = append(received[r.URL.Path], r.Header.Get("Authorization")+"|"+string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "Authorization=Basic dGVzdDp0b2tlbg==")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")

	tel, err := telemetry.Setup(t.Context(), func(key string) string {
		if key == telemetry.EndpointVariable {
			return collector.URL
		}
		return ""
	}, "shortbit-test")
	if err != nil || !tel.Enabled() {
		t.Fatalf("Setup() = enabled %v, %v; want on", tel.Enabled(), err)
	}
	t.Cleanup(func() { telemetry.UseMeterProvider(noop.NewMeterProvider()) })

	telemetry.RecordRedirect(t.Context(), telemetry.RedirectFound, "qr")
	logger := logging.NewLogger(io.Discard, tel.LoggerProvider)
	logger.Info("http request", "route", "/{code}", "note", "owner ana@example.com")

	if err := tel.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	metrics, logs := received["/v1/metrics"], received["/v1/logs"]
	if len(metrics) == 0 || !strings.Contains(strings.Join(metrics, ""), "shortbit.redirects") {
		t.Errorf("collector got no redirect metric: %d metric requests", len(metrics))
	}
	if len(logs) == 0 || !strings.Contains(strings.Join(logs, ""), "http request") {
		t.Fatalf("collector got no log: %d log requests", len(logs))
	}
	all := strings.Join(append(metrics, logs...), "")
	if !strings.Contains(all, "Basic dGVzdDp0b2tlbg==") || !strings.Contains(all, "shortbit-test") {
		t.Error("exports lack the auth header or the service name")
	}
	if strings.Contains(all, "ana@example.com") {
		t.Error("an email reached the collector")
	}
}

func TestSecurityEventLogsTheEventName(t *testing.T) {
	var out strings.Builder
	previous := slog.Default()
	slog.SetDefault(logging.NewLogger(&out, nil))
	t.Cleanup(func() { slog.SetDefault(previous) })

	telemetry.SecurityEvent(t.Context(), telemetry.EventLoginFailed, slog.String("reason", "invalid_credentials"))
	telemetry.SecurityEvent(t.Context(), telemetry.EventPasswordChanged)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 ||
		!strings.Contains(lines[0], `"level":"WARN"`) || !strings.Contains(lines[0], `"event":"login_failed"`) || !strings.Contains(lines[0], `"reason":"invalid_credentials"`) ||
		!strings.Contains(lines[1], `"level":"INFO"`) || !strings.Contains(lines[1], `"event":"password_changed"`) {
		t.Errorf("log = %s", out.String())
	}
}
