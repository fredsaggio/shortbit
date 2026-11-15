package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// collect points the instruments at a manual reader for one test.
func collect(t *testing.T) func() metricdata.ResourceMetrics {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	UseMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	t.Cleanup(func() { UseMeterProvider(noop.NewMeterProvider()) })
	return func() metricdata.ResourceMetrics {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			t.Fatalf("collect: %v", err)
		}
		return rm
	}
}

func find(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

// counts maps the value of one attribute to the counter total.
func counts(t *testing.T, rm metricdata.ResourceMetrics, name, key string) map[string]int64 {
	t.Helper()
	m, ok := find(rm, name)
	if !ok {
		t.Fatalf("metric %s not recorded", name)
	}
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("metric %s is %T, want a counter", name, m.Data)
	}
	got := map[string]int64{}
	for _, point := range sum.DataPoints {
		value, _ := point.Attributes.Value(attribute.Key(key))
		got[value.AsString()] += point.Value
	}
	return got
}

func TestCountersUseFixedWordsOnly(t *testing.T) {
	read := collect(t)
	ctx := t.Context()

	RecordRedirect(ctx, RedirectFound, "link")
	RecordRedirect(ctx, RedirectFound, "qr")
	RecordRedirect(ctx, RedirectNotFound, "link")
	RecordCacheLookup(ctx, CacheHit)
	RecordCacheLookup(ctx, CacheMiss)
	RecordCacheLookup(ctx, CacheMiss)
	RecordClickPublished(ctx, ClickQueued)
	RecordClickPublished(ctx, ClickSynchronous)
	SecurityEvent(ctx, EventLoginFailed)
	SecurityEvent(ctx, EventLoginFailed)
	SecurityEvent(ctx, EventRateLimited)

	rm := read()
	if got := counts(t, rm, "shortbit.redirects", "result"); got[RedirectFound] != 2 || got[RedirectNotFound] != 1 {
		t.Errorf("redirects by result = %v", got)
	}
	if got := counts(t, rm, "shortbit.redirects", "source"); got["link"] != 2 || got["qr"] != 1 {
		t.Errorf("redirects by source = %v", got)
	}
	if got := counts(t, rm, "shortbit.destination_cache.lookups", "result"); got[CacheHit] != 1 || got[CacheMiss] != 2 {
		t.Errorf("cache lookups = %v", got)
	}
	if got := counts(t, rm, "shortbit.click_events.published", "outcome"); got[ClickQueued] != 1 || got[ClickSynchronous] != 1 {
		t.Errorf("published clicks = %v", got)
	}
	if got := counts(t, rm, "shortbit.security_events", "event"); got[EventLoginFailed] != 2 || got[EventRateLimited] != 1 {
		t.Errorf("security events = %v", got)
	}
}

func TestClickLagIsRecordedOnlyForCountedClicks(t *testing.T) {
	read := collect(t)
	ctx := t.Context()

	RecordClickProcessed(ctx, ClickCounted, time.Now().Add(-3*time.Second))
	RecordClickProcessed(ctx, ClickDuplicate, time.Now().Add(-time.Hour))
	RecordClickProcessed(ctx, ClickDeadLettered, time.Time{})

	rm := read()
	if got := counts(t, rm, "shortbit.click_events.processed", "outcome"); got[ClickCounted] != 1 || got[ClickDuplicate] != 1 || got[ClickDeadLettered] != 1 {
		t.Errorf("processed clicks = %v", got)
	}
	m, _ := find(rm, "shortbit.click_events.lag")
	lag := m.Data.(metricdata.Histogram[float64])
	if len(lag.DataPoints) != 1 || lag.DataPoints[0].Count != 1 || lag.DataPoints[0].Sum < 3 || lag.DataPoints[0].Sum > 10 {
		t.Errorf("lag = %+v, want one sample of about 3 seconds", lag.DataPoints)
	}
}

func TestHTTPDurationUsesTheRoutePatternAndKnownMethods(t *testing.T) {
	read := collect(t)
	ctx := t.Context()

	RecordHTTPRequest(ctx, http.MethodGet, "/{code}", http.StatusFound, 20*time.Millisecond)
	RecordHTTPRequest(ctx, "BREW", NoRoute, http.StatusNotFound, time.Millisecond)

	m, ok := find(read(), "http.server.request.duration")
	if !ok || m.Unit != "s" {
		t.Fatalf("duration metric = %+v", m)
	}
	routes := map[string]bool{}
	methods := map[string]bool{}
	for _, point := range m.Data.(metricdata.Histogram[float64]).DataPoints {
		route, _ := point.Attributes.Value("http.route")
		method, _ := point.Attributes.Value("http.request.method")
		routes[route.AsString()] = true
		methods[method.AsString()] = true
	}
	if !routes["/{code}"] || !routes[NoRoute] || !methods["GET"] || !methods["_OTHER"] || methods["BREW"] {
		t.Errorf("routes = %v, methods = %v", routes, methods)
	}
}

func TestRecordRouteSurvivesRequestCopies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /urls/{code}", func(http.ResponseWriter, *http.Request) {})
	// A middleware between the holder and the mux that copies the request,
	// as ClientIP and RequestID do.
	type key struct{}
	copying := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, "copy")))
		})
	}
	handler := copying(RecordRoute(mux))

	for path, want := range map[string]string{"/urls/Ab3dX9": "/urls/{code}", "/nada/aqui": NoRoute} {
		ctx, route := WithRouteHolder(t.Context())
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil))
		if got := route(); got != want {
			t.Errorf("route of %s = %q, want %q", path, got, want)
		}
	}
}

func TestRoutePath(t *testing.T) {
	for pattern, want := range map[string]string{
		"GET /urls/{code}":           "/urls/{code}",
		"/assets/":                   "/assets/",
		"GET shortbit.com.br/{code}": "/{code}",
		"":                           "",
	} {
		if got := routePath(pattern); got != want {
			t.Errorf("routePath(%q) = %q, want %q", pattern, got, want)
		}
	}
}
