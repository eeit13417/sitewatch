package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	loadRootEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pgPool, err := pgxpool.New(ctx, os.Getenv("POSTGRES_URL"))
	if err != nil {
		logger.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pgPool.Close()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		logger.Error("connect mongo", "error", err)
		os.Exit(1)
	}
	defer mongoClient.Disconnect(context.Background())

	app := &App{pg: pgPool, mongo: mongoClient, mongoDB: os.Getenv("MONGO_DB"), logger: logger}
	mux := routes(app)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
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

func loadRootEnv() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", ".env")
	if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Warn("could not load .env", "path", path, "error", err)
	}
}
