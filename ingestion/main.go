package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/sitewatch/shared"
)

// defaultWorkerCount bounds how many messages are processed concurrently —
// enough to not serialize on DB round-trips, small enough not to open
// unbounded connections under a burst. defaultQueueSize is the backpressure
// buffer: once full, new messages are dropped rather than blocking the MQTT
// client indefinitely (see the "drop, don't block" note in the process
// log). Both are overridable via INGESTION_WORKER_COUNT/INGESTION_QUEUE_SIZE
// — CLAUDE.md rule 2 flags worker counts as exactly the kind of value that
// needs tuning per environment rather than being buried as a constant; see
// docs/rca/03-mqtt-backlog.md for why this needed to actually be tunable
// (reproducing a backlog drill meant deliberately running with 1 worker).
// defaultDebounceBreaches is how many consecutive same-direction readings
// EvaluateRules requires before creating or auto-resolving an alert — see
// docs/alert-engine.md and docs/rca/05-alert-storm.md. Overridable via
// ALERT_DEBOUNCE_BREACHES.
const (
	defaultWorkerCount      = 4
	defaultQueueSize        = 256
	defaultDebounceBreaches = 3
)

func intFromEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

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

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		messagesReceived, messagesDropped, processingDuration, alertDecisionsTotal,
		shared.NewPostgresPoolCollector(pgPool, "ingestion"),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	metricsPort := os.Getenv("INGESTION_PORT")
	if metricsPort == "" {
		metricsPort = "8081"
	}
	metricsServer := newMetricsServer(registry, metricsPort)
	go func() {
		logger.Info("metrics server starting", "port", metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server stopped", "error", err)
		}
	}()

	workerCount := intFromEnv("INGESTION_WORKER_COUNT", defaultWorkerCount)
	queueSize := intFromEnv("INGESTION_QUEUE_SIZE", defaultQueueSize)
	debounceN := intFromEnv("ALERT_DEBOUNCE_BREACHES", defaultDebounceBreaches)
	tracker := NewBreachTracker()
	jobs := make(chan job, queueSize)
	for i := 0; i < workerCount; i++ {
		go worker(ctx, logger, store, tracker, debounceN, jobs)
	}
	logger.Info("worker pool started", "workers", workerCount, "queue_size", queueSize, "alert_debounce_breaches", debounceN)

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
				messagesDropped.Inc()
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("metrics server shutdown", "error", err)
	}
}

func worker(ctx context.Context, logger *slog.Logger, store *Store, tracker *BreachTracker, debounceN int, jobs <-chan job) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-jobs:
			messagesReceived.Inc()
			start := time.Now()
			err := process(ctx, logger, store, tracker, debounceN, j)
			processingDuration.Observe(time.Since(start).Seconds())
			if err != nil {
				logger.Warn("failed to process message", "topic", j.topic, "error", err)
			}
		}
	}
}

// process handles one message end to end. Every log line here goes
// through msgLogger (base logger + a fresh correlation_id) instead of the
// bare logger, so grepping one id shows this message's complete story —
// see docs/observability-design.md.
func process(ctx context.Context, logger *slog.Logger, store *Store, tracker *BreachTracker, debounceN int, j job) error {
	msgLogger := logger.With("correlation_id", shared.NewCorrelationID())

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
		msgLogger.Warn("update last_seen_at failed", "device_id", msg.DeviceID, "error", err)
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

	decisions := tracker.Evaluate(rules, msg.Readings, active, debounceN)
	if len(decisions) == 0 {
		return nil
	}

	if err := applyDecisionsWithRetry(ctx, store, msg.DeviceID, decisions, now); err != nil {
		return err
	}
	for _, d := range decisions {
		label := decisionLabel(d.Decision)
		alertDecisionsTotal.WithLabelValues(label).Inc()
		msgLogger.Info("alert decision", "device_id", msg.DeviceID, "rule_id", d.Rule.ID,
			"action", label, "value", d.Value)
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
