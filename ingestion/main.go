package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/sitewatch/shared"
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

	mongoDB := os.Getenv("MONGO_DB")
	if err := shared.EnsureTelemetryIndexes(ctx, mongoClient, mongoDB); err != nil {
		logger.Error("ensure mongo indexes", "error", err)
		os.Exit(1)
	}

	store := NewStore(pgPool, mongoClient, mongoDB)

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
