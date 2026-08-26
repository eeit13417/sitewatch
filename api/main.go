package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sitewatch/shared"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	shared.LoadRootEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pgPool, err := shared.NewPostgresPool(ctx)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()

	mongoClient, err := shared.NewMongoClient(ctx)
	if err != nil {
		logger.Error("connect mongo", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Warn("mongo disconnect", "error", err)
		}
	}()

	app := &App{pg: pgPool, mongo: mongoClient, mongoDB: os.Getenv("MONGO_DB"), logger: logger}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		httpRequestsTotal, httpRequestDuration,
		shared.NewPostgresPoolCollector(pgPool, "api"),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	mux := routes(app, registry)
	limiter := rateLimitFromEnv()
	cleanupDone := make(chan struct{})
	go limiter.cleanupLoop(cleanupDone, time.Minute, 10*time.Minute)
	defer close(cleanupDone)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: withCORS(withCorrelationID(logger, withRateLimit(limiter, logger, mux))),
		// Unset, a slow/malicious client can hold a connection open
		// indefinitely while trickling in headers, tying up a goroutine
		// per connection (a Slowloris DoS) — flagged by gosec (G112).
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", "error", err)
		}
	}()

	logger.Info("api service starting", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// routes is factored out of main so integration tests can build the exact
// same handler tree against a test App without duplicating registrations.
// registry may be nil (tests that don't care about metrics) — /metrics is
// simply omitted in that case.
func routes(app *App, registry *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()

	// register wraps each handler with per-route request metrics
	// (docs/observability-design.md) so every route gets this without
	// remembering to call instrument() at every call site below.
	register := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, instrument(pattern, h))
	}

	register("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	if registry != nil {
		mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	}
	register("GET /sites", app.listSites)
	register("GET /devices", app.listDevices)
	register("GET /devices/{id}", app.getDevice)
	register("GET /devices/{id}/telemetry", app.getDeviceTelemetry)
	register("GET /alerts", app.listAlerts)
	register("POST /alerts/{id}/acknowledge", app.acknowledgeAlert)
	register("POST /alerts/{id}/resolve", app.resolveAlert)
	register("GET /alert-rules", app.listAlertRules)
	register("POST /alert-rules", app.createAlertRule)
	register("PATCH /alert-rules/{id}", app.updateAlertRule)
	register("DELETE /alert-rules/{id}", app.deleteAlertRule)
	return mux
}

// withCORS lets the Phase 3 browser frontend call this API cross-origin.
// CORS_ALLOWED_ORIGINS (comma-separated) defaults to localhost AND
// 127.0.0.1 on the Vite dev port — both are valid ways to reach the same
// local dev server, but the browser treats them as different origins, and
// a fixed single-origin default broke "Failed to fetch" the moment
// someone opened the app via the other one. Reflects the actual request
// Origin back only if it's in the allow-list — never a bare "*", which a
// security linter would flag for good reason and which this API has no
// case for anyway.
func withCORS(h http.Handler) http.Handler {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		raw = "http://localhost:5173,http://127.0.0.1:5173"
	}
	allowed := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		allowed[strings.TrimSpace(o)] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
