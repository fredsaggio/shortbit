package telemetry

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// Attribute values are always one of a few fixed words. A short code, an
// email, an IP or a user ID never becomes an attribute: each distinct value
// is a new series, which would break the plan limits and leak who did what.

const meterName = "github.com/fredsaggio/url-shortener"

type instruments struct {
	httpDuration         metric.Float64Histogram
	redirects            metric.Int64Counter
	cacheLookups         metric.Int64Counter
	clickEventsPublished metric.Int64Counter
	clickEventsProcessed metric.Int64Counter
	clickEventLag        metric.Float64Histogram
	securityEvents       metric.Int64Counter
	meter                metric.Meter
}

var current atomic.Pointer[instruments]

func init() {
	UseMeterProvider(noop.NewMeterProvider())
}

// UseMeterProvider points every instrument at provider. Setup calls it; tests
// call it with a provider backed by a manual reader.
func UseMeterProvider(provider metric.MeterProvider) {
	meter := provider.Meter(meterName)
	// Creating an instrument fails only for an invalid name or unit, fixed
	// here, so the errors are dropped.
	httpDuration, _ := meter.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP requests, by route pattern, method and status."),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10))
	redirects, _ := meter.Int64Counter("shortbit.redirects",
		metric.WithUnit("{redirect}"),
		metric.WithDescription("Visits to short links, by result and by source (link or QR code)."))
	cacheLookups, _ := meter.Int64Counter("shortbit.destination_cache.lookups",
		metric.WithUnit("{lookup}"),
		metric.WithDescription("Redis lookups of public link destinations, by result."))
	published, _ := meter.Int64Counter("shortbit.click_events.published",
		metric.WithUnit("{event}"),
		metric.WithDescription("Click events by how they were recorded: queued, counted synchronously, or lost."))
	processed, _ := meter.Int64Counter("shortbit.click_events.processed",
		metric.WithUnit("{event}"),
		metric.WithDescription("Click events handled by the worker, by outcome."))
	lag, _ := meter.Float64Histogram("shortbit.click_events.lag",
		metric.WithUnit("s"),
		metric.WithDescription("Time from a click to its count."),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60, 300, 900))
	security, _ := meter.Int64Counter("shortbit.security_events",
		metric.WithUnit("{event}"),
		metric.WithDescription("Security events, such as failed logins and rate limiting, by event name."))

	current.Store(&instruments{
		httpDuration:         httpDuration,
		redirects:            redirects,
		cacheLookups:         cacheLookups,
		clickEventsPublished: published,
		clickEventsProcessed: processed,
		clickEventLag:        lag,
		securityEvents:       security,
		meter:                meter,
	})
}

// RecordHTTPRequest records one answered request. route is the ServeMux
// pattern without the method ("/urls/{code}"), never the raw path.
func RecordHTTPRequest(ctx context.Context, method, route string, status int, elapsed time.Duration) {
	current.Load().httpDuration.Record(ctx, elapsed.Seconds(), metric.WithAttributes(
		attribute.String("http.request.method", knownMethod(method)),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
	))
}

// Redirect results.
const (
	RedirectFound            = "redirected"
	RedirectPasswordRequired = "password_required"
	RedirectNotFound         = "not_found"
	RedirectFailed           = "failed"
)

func RecordRedirect(ctx context.Context, result, source string) {
	current.Load().redirects.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
		attribute.String("source", source),
	))
}

// Cache lookup results.
const (
	CacheHit   = "hit"
	CacheMiss  = "miss"
	CacheError = "error"
)

func RecordCacheLookup(ctx context.Context, result string) {
	current.Load().cacheLookups.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

// How a click event was recorded at redirect time.
const (
	ClickQueued       = "queued"
	ClickSynchronous  = "synchronous"
	ClickNotRecorded  = "not_recorded"
	ClickCounted      = "counted"
	ClickDuplicate    = "duplicate"
	ClickDiscarded    = "discarded"
	ClickFailed       = "failed"
	ClickDeadLettered = "dead_lettered"
)

func RecordClickPublished(ctx context.Context, outcome string) {
	current.Load().clickEventsPublished.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// RecordClickProcessed records what the worker did with an event. A click
// counted now also records how long it waited since it happened.
func RecordClickProcessed(ctx context.Context, outcome string, occurredAt time.Time) {
	m := current.Load()
	m.clickEventsProcessed.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	if outcome == ClickCounted && !occurredAt.IsZero() {
		m.clickEventLag.Record(ctx, max(0, time.Since(occurredAt).Seconds()))
	}
}

func recordSecurityEvent(ctx context.Context, event string) {
	current.Load().securityEvents.Add(ctx, 1, metric.WithAttributes(attribute.String("event", event)))
}

// ObserveDBPool reports the connection pool on every export: connections in
// use, idle, open and the maximum. A pool always at its maximum means
// requests are waiting for a connection.
func ObserveDBPool(pool *pgxpool.Pool) error {
	meter := current.Load().meter
	inUse, err := meter.Int64ObservableGauge("db.client.connections.in_use", metric.WithUnit("{connection}"),
		metric.WithDescription("PostgreSQL connections in use."))
	if err != nil {
		return err
	}
	idle, err := meter.Int64ObservableGauge("db.client.connections.idle", metric.WithUnit("{connection}"),
		metric.WithDescription("PostgreSQL connections open and idle."))
	if err != nil {
		return err
	}
	limit, err := meter.Int64ObservableGauge("db.client.connections.max", metric.WithUnit("{connection}"),
		metric.WithDescription("Maximum PostgreSQL connections of the pool."))
	if err != nil {
		return err
	}
	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		stat := pool.Stat()
		observer.ObserveInt64(inUse, int64(stat.AcquiredConns()))
		observer.ObserveInt64(idle, int64(stat.IdleConns()))
		observer.ObserveInt64(limit, int64(stat.MaxConns()))
		return nil
	}, inUse, idle, limit)
	return err
}

// knownMethod keeps invented methods from becoming series of their own.
func knownMethod(method string) string {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return method
	default:
		return "_OTHER"
	}
}
