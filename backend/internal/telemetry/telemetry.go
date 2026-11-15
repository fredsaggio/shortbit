// Package telemetry sends metrics and logs to an OpenTelemetry collector over
// OTLP/HTTP, such as Grafana Cloud in production or the grafana/otel-lgtm
// container locally (ADR 0006).
//
// It is off unless OTEL_EXPORTER_OTLP_ENDPOINT is set: then every
// instrument records into a no-op provider and nothing leaves the process.
// The exporters read the standard OTEL_* variables themselves (endpoint,
// headers with the token, export interval), so the token never passes
// through this code.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const EndpointVariable = "OTEL_EXPORTER_OTLP_ENDPOINT"

// Telemetry holds what Setup started. LoggerProvider is nil when telemetry
// is off.
type Telemetry struct {
	LoggerProvider otellog.LoggerProvider
	shutdown       []func(context.Context) error
}

// Enabled reports whether metrics and logs leave the process.
func (t *Telemetry) Enabled() bool {
	return t.LoggerProvider != nil
}

// Shutdown flushes what is still buffered. Call it once, on exit.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, stop := range t.shutdown {
		errs = append(errs, stop(ctx))
	}
	return errors.Join(errs...)
}

// Setup starts the metric and log exporters when OTEL_EXPORTER_OTLP_ENDPOINT
// is set. service names the process (shortbit-api, shortbit-worker);
// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override it.
func Setup(ctx context.Context, getEnv func(string) string, service string) (*Telemetry, error) {
	if strings.TrimSpace(getEnv(EndpointVariable)) == "" {
		return &Telemetry{}, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(service)),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("build telemetry resource: %w", err)
	}

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	// One export a minute (OTEL_METRIC_EXPORT_INTERVAL changes it) keeps the
	// data points well inside free plan limits.
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)

	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		_ = meterProvider.Shutdown(ctx)
		return nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}
	loggerProvider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)

	otel.SetMeterProvider(meterProvider)
	UseMeterProvider(meterProvider)
	// An unreachable collector must not take the app down; it is reported
	// and the next export tries again.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("telemetry export failed", "error", err)
	}))
	if err := runtime.Start(runtime.WithMeterProvider(meterProvider), runtime.WithMinimumReadMemStatsInterval(15*time.Second)); err != nil {
		slog.Warn("Go runtime metrics unavailable", "error", err)
	}

	return &Telemetry{
		LoggerProvider: loggerProvider,
		shutdown:       []func(context.Context) error{meterProvider.Shutdown, loggerProvider.Shutdown},
	}, nil
}
