//go:build integration

package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Run with: go test -tags=integration ./...
// Spins up real Postgres and MongoDB containers and applies the actual
// infra/postgres/init.sql — the same file docker-compose uses — so these
// tests exercise the real schema, not a hand-rolled test fixture.

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("sitewatch"),
		tcpostgres.WithUsername("sitewatch"),
		tcpostgres.WithPassword("sitewatch"),
		tcpostgres.WithInitScripts(filepath.Join("..", "infra", "postgres", "init.sql")),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	pgConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	pgPool, err := pgxpool.New(ctx, pgConnStr)
	if err != nil {
		t.Fatalf("connect postgres pool: %v", err)
	}
	t.Cleanup(pgPool.Close)

	mongoContainer, err := tcmongodb.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}
	t.Cleanup(func() { _ = mongoContainer.Terminate(ctx) })

	mongoConnStr, err := mongoContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("mongo connection string: %v", err)
	}
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoConnStr))
	if err != nil {
		t.Fatalf("connect mongo client: %v", err)
	}
	t.Cleanup(func() { _ = mongoClient.Disconnect(ctx) })

	return NewStore(pgPool, mongoClient, "sitewatch")
}

func TestWriteTelemetry_LandsInMongo(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	msg := TelemetryMessage{
		DeviceID:   "a1111111-0000-0000-0000-000000000002", // temp-sensor-01, seeded in init.sql
		SiteID:     "11111111-1111-1111-1111-111111111111",
		DeviceType: "temp_sensor",
		Readings:   map[string]float64{"temperature_c": 24.5},
		Ts:         time.Now().UTC(),
	}
	if err := store.WriteTelemetry(ctx, msg); err != nil {
		t.Fatalf("WriteTelemetry: %v", err)
	}

	coll := store.mongo.Database(store.mongoDB).Collection("telemetry_raw")
	count, err := coll.CountDocuments(ctx, bson.M{"device_id": msg.DeviceID})
	if err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 telemetry_raw document, found %d", count)
	}
}

func TestMaybeUpdateLastSeen_DebouncesWithinWindow(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	deviceID := "a1111111-0000-0000-0000-000000000001" // smart-meter-01

	t1 := time.Now().UTC()
	if err := store.MaybeUpdateLastSeen(ctx, deviceID, t1); err != nil {
		t.Fatalf("first update: %v", err)
	}

	var lastSeen1 time.Time
	if err := store.pg.QueryRow(ctx, `SELECT last_seen_at FROM devices WHERE id = $1`, deviceID).Scan(&lastSeen1); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	if !lastSeen1.Equal(t1) {
		t.Fatalf("expected last_seen_at %v, got %v", t1, lastSeen1)
	}

	// Within the debounce window: should NOT write again.
	t2 := t1.Add(5 * time.Second)
	if err := store.MaybeUpdateLastSeen(ctx, deviceID, t2); err != nil {
		t.Fatalf("second update: %v", err)
	}
	var lastSeen2 time.Time
	if err := store.pg.QueryRow(ctx, `SELECT last_seen_at FROM devices WHERE id = $1`, deviceID).Scan(&lastSeen2); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	if !lastSeen2.Equal(t1) {
		t.Fatalf("expected last_seen_at to stay at %v (debounced), got %v", t1, lastSeen2)
	}

	// Past the debounce window: should write.
	t3 := t1.Add(lastSeenDebounce + time.Second)
	if err := store.MaybeUpdateLastSeen(ctx, deviceID, t3); err != nil {
		t.Fatalf("third update: %v", err)
	}
	var lastSeen3 time.Time
	if err := store.pg.QueryRow(ctx, `SELECT last_seen_at FROM devices WHERE id = $1`, deviceID).Scan(&lastSeen3); err != nil {
		t.Fatalf("read last_seen_at: %v", err)
	}
	if !lastSeen3.Equal(t3) {
		t.Fatalf("expected last_seen_at %v after debounce window passed, got %v", t3, lastSeen3)
	}
}

func TestAlertLifecycle_CreateThenAutoResolve(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	deviceID := "a1111111-0000-0000-0000-000000000002" // temp-sensor-01, has seeded warning/critical rules

	rules, err := store.LoadEnabledRules(ctx, deviceID)
	if err != nil {
		t.Fatalf("LoadEnabledRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 seeded rules for temp-sensor-01, got %d", len(rules))
	}

	// A spike above both thresholds: both should create alerts.
	active, err := store.LoadActiveRuleIDs(ctx, deviceID)
	if err != nil {
		t.Fatalf("LoadActiveRuleIDs: %v", err)
	}
	decisions := EvaluateRules(rules, map[string]float64{"temperature_c": 33}, active)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 CreateAlert decisions, got %d", len(decisions))
	}
	now := time.Now().UTC()
	if err := store.ApplyDecisions(ctx, deviceID, decisions, now); err != nil {
		t.Fatalf("ApplyDecisions (create): %v", err)
	}

	var openCount int
	if err := store.pg.QueryRow(ctx,
		`SELECT count(*) FROM alerts WHERE device_id = $1 AND status = 'open'`, deviceID,
	).Scan(&openCount); err != nil {
		t.Fatalf("count open alerts: %v", err)
	}
	if openCount != 2 {
		t.Fatalf("expected 2 open alerts, found %d", openCount)
	}

	// Re-evaluating the same spike must not create duplicates (dedup).
	active, _ = store.LoadActiveRuleIDs(ctx, deviceID)
	decisions = EvaluateRules(rules, map[string]float64{"temperature_c": 33}, active)
	if len(decisions) != 0 {
		t.Fatalf("expected no new decisions while already active, got %d", len(decisions))
	}

	// Reading back to normal: both should auto-resolve.
	active, _ = store.LoadActiveRuleIDs(ctx, deviceID)
	decisions = EvaluateRules(rules, map[string]float64{"temperature_c": 24}, active)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 AutoResolve decisions, got %d", len(decisions))
	}
	if err := store.ApplyDecisions(ctx, deviceID, decisions, time.Now().UTC()); err != nil {
		t.Fatalf("ApplyDecisions (resolve): %v", err)
	}

	var resolvedCount int
	if err := store.pg.QueryRow(ctx,
		`SELECT count(*) FROM alerts WHERE device_id = $1 AND status = 'resolved'`, deviceID,
	).Scan(&resolvedCount); err != nil {
		t.Fatalf("count resolved alerts: %v", err)
	}
	if resolvedCount != 2 {
		t.Fatalf("expected 2 resolved alerts, found %d", resolvedCount)
	}
}
