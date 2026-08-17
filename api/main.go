package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	mux := routes(app)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: withCORS(mux),
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
func routes(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /sites", app.listSites)
	mux.HandleFunc("GET /devices", app.listDevices)
	mux.HandleFunc("GET /devices/{id}", app.getDevice)
	mux.HandleFunc("GET /devices/{id}/telemetry", app.getDeviceTelemetry)
	mux.HandleFunc("GET /alerts", app.listAlerts)
	mux.HandleFunc("POST /alerts/{id}/acknowledge", app.acknowledgeAlert)
	mux.HandleFunc("POST /alerts/{id}/resolve", app.resolveAlert)
	mux.HandleFunc("GET /alert-rules", app.listAlertRules)
	mux.HandleFunc("POST /alert-rules", app.createAlertRule)
	mux.HandleFunc("PATCH /alert-rules/{id}", app.updateAlertRule)
	mux.HandleFunc("DELETE /alert-rules/{id}", app.deleteAlertRule)
	return mux
}

// withCORS lets the Phase 3 browser frontend call this API cross-origin.
// CORS_ALLOWED_ORIGIN defaults to the local Vite dev server port rather
// than "*" — a wildcard origin is the kind of thing a security linter
// flags for good reason, and there's no case here where any origin should
// be allowed to call this API.
func withCORS(h http.Handler) http.Handler {
	origin := os.Getenv("CORS_ALLOWED_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
