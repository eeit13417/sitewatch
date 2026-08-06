package main

import (
	"log/slog"
	"os"
)

// Phase 2 will replace this with an MQTT subscriber that writes
// telemetry into MongoDB and aggregates into PostgreSQL.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("ingestion service placeholder — MQTT wiring lands in Phase 2")
}
