package telemetry

import (
	"context"
	"net/http"
	"strings"
)

// The ServeMux writes the matched pattern into the request it receives,
// but middlewares in between pass copies made with WithContext, so the
// pattern never reaches the access log. The access log puts a holder in the
// context and RecordRoute, right around the mux, fills it.

type routeKey struct{}

type routeHolder struct {
	route string
}

// NoRoute labels requests answered before routing (rate limited) or
// matching no pattern.
const NoRoute = "none"

// WithRouteHolder returns a context RecordRoute can write into, and a
// function that reads what it wrote.
func WithRouteHolder(ctx context.Context) (context.Context, func() string) {
	holder := &routeHolder{}
	return context.WithValue(ctx, routeKey{}, holder), func() string {
		if holder.route == "" {
			return NoRoute
		}
		return holder.route
	}
}

// RecordRoute wraps the mux and saves the pattern it matched.
func RecordRoute(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if holder, ok := r.Context().Value(routeKey{}).(*routeHolder); ok {
			holder.route = routePath(r.Pattern)
		}
	})
}

// routePath drops the method and host from a pattern: "GET /urls/{code}"
// becomes "/urls/{code}".
func routePath(pattern string) string {
	if _, path, ok := strings.Cut(pattern, " "); ok {
		pattern = path
	}
	if i := strings.Index(pattern, "/"); i > 0 {
		pattern = pattern[i:]
	}
	return pattern
}
