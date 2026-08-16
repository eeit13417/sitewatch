package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// workerCount bounds how many messages are processed concurrently — enough
// to not serialize on DB round-trips, small enough not to open unbounded
// connections under a burst. queueSize is the backpressure buffer: once
// full, new messages are dropped rather than blocking the MQTT client
// indefinitely (see the "drop, don't block" note in the process log).
const (
	workerCount = 4
	queueSize   = 256
)

type job struct {
	topic   string
	payload []byte
}

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

	store := NewStore(pgPool, mongoClient, os.Getenv("MONGO_DB"))

	jobs := make(chan job, queueSize)
	for i := 0; i < workerCount; i++ {
		go worker(ctx, logger, store, jobs)
	}

	brokerURL := os.Getenv("MQTT_BROKER_URL")
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("sitewatch-ingestion").
		SetAutoReconnect(true)
	opts.OnConnect = func(c mqtt.Client) {
		logger.Info("connected to broker, subscribing", "broker", brokerURL)
		token := c.Subscribe("sitewatch/+/+/telemetry", 1, func(_ mqtt.Client, m mqtt.Message) {
			payload := append([]byte(nil), m.Payload()...)
			select {
			case jobs <- job{topic: m.Topic(), payload: payload}:
			default:
				logger.Warn("job queue full, dropping message", "topic", m.Topic())
			}
		})
		token.Wait()
		if token.Error() != nil {
			logger.Error("subscribe failed", "error", token.Error())
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		logger.Warn("mqtt connection lost", "error", err)
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error("mqtt connect", "error", token.Error())
		os.Exit(1)
	}

	<-ctx.Done()
	logger.Info("shutting down")
	client.Disconnect(250)
}

func worker(ctx context.Context, logger *slog.Logger, store *Store, jobs <-chan job) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			if err := process(ctx, logger, store, j); err != nil {
				logger.Warn("failed to process message", "topic", j.topic, "error", err)
			}
		}
	}
}

func process(ctx context.Context, logger *slog.Logger, store *Store, j job) error {
	siteID, deviceID, err := parseTopic(j.topic)
	if err != nil {
		return err
	}

	msg, err := parseTelemetryMessage(j.payload)
	if err != nil {
		return err
	}
	if msg.SiteID != siteID || msg.DeviceID != deviceID {
		return fmt.Errorf("payload ids (%s/%s) don't match topic ids (%s/%s)",
			msg.SiteID, msg.DeviceID, siteID, deviceID)
	}

	if err := store.WriteTelemetry(ctx, msg); err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := store.MaybeUpdateLastSeen(ctx, msg.DeviceID, now); err != nil {
		// Non-fatal: telemetry already landed in Mongo; losing one
		// last_seen_at update just delays the next debounced write.
		logger.Warn("update last_seen_at failed", "device_id", msg.DeviceID, "error", err)
	}

	rules, err := store.LoadEnabledRules(ctx, msg.DeviceID)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	active, err := store.LoadActiveRuleIDs(ctx, msg.DeviceID)
	if err != nil {
		return err
	}

	decisions := EvaluateRules(rules, msg.Readings, active)
	if len(decisions) == 0 {
		return nil
	}

	if err := store.ApplyDecisions(ctx, msg.DeviceID, decisions, now); err != nil {
		return err
	}
	for _, d := range decisions {
		logger.Info("alert decision", "device_id", msg.DeviceID, "rule_id", d.Rule.ID,
			"action", decisionLabel(d.Decision), "value", d.Value)
	}
	return nil
}

func decisionLabel(d Decision) string {
	switch d {
	case CreateAlert:
		return "create"
	case AutoResolve:
		return "auto_resolve"
	default:
		return "none"
	}
}

// loadRootEnv loads the repo-root .env (two directories up from this file,
// whether running via `go run .` from source or a built binary in the same
// tree) so local dev doesn't need every var exported manually.
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
