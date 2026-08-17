package shared

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// defaultMaxConns caps the Postgres pool per service instance. Explicit
// rather than left at the driver default so it's a documented, tunable
// knob for "many concurrent users" instead of an accident of whatever
// pgxpool ships with.
const defaultMaxConns = 10

// NewPostgresPool builds a pool from POSTGRES_URL, sized by
// POSTGRES_MAX_CONNS (falls back to defaultMaxConns).
func NewPostgresPool(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(os.Getenv("POSTGRES_URL"))
	if err != nil {
		return nil, fmt.Errorf("parse POSTGRES_URL: %w", err)
	}

	cfg.MaxConns = defaultMaxConns
	if raw := os.Getenv("POSTGRES_MAX_CONNS"); raw != "" {
		// ParseInt with an explicit bit size, not Atoi + a manual int32
		// cast, so an out-of-range value errors instead of silently
		// wrapping.
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
			cfg.MaxConns = int32(n)
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return pool, nil
}

// NewMongoClient connects using MONGO_URL.
func NewMongoClient(ctx context.Context) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URL")))
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	return client, nil
}

// EnsureTelemetryIndexes creates the indexes telemetry_raw's actual query
// patterns need — every read goes through GET /devices/{id}/telemetry,
// which filters by device_id and sorts by ts. Without this, that query is
// a full collection scan; MongoDB doesn't index anything for you the way
// Postgres indexes a primary key automatically. Creating an index that
// already exists (same name, same spec) is a no-op, so this is safe to
// call on every service startup rather than needing a separate migration
// step.
func EnsureTelemetryIndexes(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection("telemetry_raw")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "device_id", Value: 1},
			{Key: "ts", Value: -1},
		},
		Options: options.Index().SetName("device_id_ts"),
	})
	if err != nil {
		return fmt.Errorf("ensure telemetry_raw index: %w", err)
	}
	return nil
}
