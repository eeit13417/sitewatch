package main

import (
	"log/slog"
	"net/http"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("api service starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
