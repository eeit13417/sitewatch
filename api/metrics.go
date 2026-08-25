package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sitewatch/shared"
)

// See docs/observability-design.md.
var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "sitewatch_api_http_requests_total",
		Help: "HTTP requests handled, by method, route pattern, and status code.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "sitewatch_api_http_request_duration_seconds",
		Help:    "HTTP request latency, by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// instrument wraps a handler registered under pattern (e.g. "GET
// /devices/{id}") so the recorded "path" label is the route *pattern*, not
// the raw URL. Recording the raw URL would give /devices/<uuid-1> and
// /devices/<uuid-2> their own time series each — an unbounded-cardinality
// mistake Prometheus makes it easy to back into.
func instrument(pattern string, h http.HandlerFunc) http.HandlerFunc {
	method, path, _ := strings.Cut(pattern, " ")
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h(sw, r)
		httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(sw.status)).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	}
}

// statusWriter decorates http.ResponseWriter to capture the status code —
// the interface doesn't expose what was written after the fact otherwise.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

type ctxKey int

const loggerCtxKey ctxKey = iota

// withCorrelationID tags every request with an id (reusing the caller's
// X-Request-ID if it sent one), echoes it back in the response header, and
// makes a logger carrying it available via loggerFromContext — so every
// handler's log lines for one request share one greppable id, the same
// idea as ingestion's per-message correlation id.
func withCorrelationID(base *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = shared.NewCorrelationID()
		}
		w.Header().Set("X-Request-ID", id)

		ctx := context.WithValue(r.Context(), loggerCtxKey, base.With("correlation_id", id))
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loggerFromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey).(*slog.Logger); ok {
		return l
	}
	return fallback
}

// log is the per-request logger every handler should use instead of
// a.logger directly, so their log lines pick up the request's
// correlation id.
func (a *App) log(r *http.Request) *slog.Logger {
	return loggerFromContext(r.Context(), a.logger)
}
