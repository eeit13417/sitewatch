package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// lastSeenDebounce caps how often devices.last_seen_at is written per
// device. Devices publish every few seconds; writing to Postgres on every
// message would be unnecessary write amplification (flagged in the Phase 1
// schema review) for a column only used to answer "is this online".
const lastSeenDebounce = 60 * time.Second

type Store struct {
	pg      *pgxpool.Pool
	mongo   *mongo.Client
	mongoDB string

	mu           sync.Mutex
	lastSeenSent map[string]time.Time
}

func NewStore(pg *pgxpool.Pool, mongoClient *mongo.Client, mongoDB string) *Store {
	return &Store{
		pg:           pg,
		mongo:        mongoClient,
		mongoDB:      mongoDB,
		lastSeenSent: make(map[string]time.Time),
	}
}

// WriteTelemetry appends the reading to MongoDB telemetry_raw, stamping
// received_at itself — the message's own "ts" stays the publisher's clock,
// per docs/mqtt-contract.md.
func (s *Store) WriteTelemetry(ctx context.Context, msg TelemetryMessage) error {
	doc := bson.M{
		"device_id":   msg.DeviceID,
		"site_id":     msg.SiteID,
		"device_type": msg.DeviceType,
		"readings":    msg.Readings,
		"ts":          msg.Ts,
		"received_at": time.Now().UTC(),
	}
	coll := s.mongo.Database(s.mongoDB).Collection("telemetry_raw")
	if _, err := coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("mongo insert telemetry_raw: %w", err)
	}
	return nil
}

// MaybeUpdateLastSeen writes devices.last_seen_at at most once per
// lastSeenDebounce window per device.
func (s *Store) MaybeUpdateLastSeen(ctx context.Context, deviceID string, at time.Time) error {
	s.mu.Lock()
	last, seen := s.lastSeenSent[deviceID]
	due := !seen || at.Sub(last) >= lastSeenDebounce
	if due {
		s.lastSeenSent[deviceID] = at
	}
	s.mu.Unlock()

	if !due {
		return nil
	}

	if _, err := s.pg.Exec(ctx, `UPDATE devices SET last_seen_at = $1 WHERE id = $2`, at, deviceID); err != nil {
		return fmt.Errorf("update last_seen_at: %w", err)
	}
	return nil
}
