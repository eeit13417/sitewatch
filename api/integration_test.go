//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sitewatch/shared"
	tcmongodb "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Run with: go test -tags=integration ./...
// Same approach as ingestion/integration_test.go: real Postgres seeded from
// the actual infra/postgres/init.sql, a real Mongo, and the exact route
// tree from routes() exercised over httptest — no mocks.

const (
	bangkokSiteID  = "11111111-1111-1111-1111-111111111111"
	tempSensor01ID = "a1111111-0000-0000-0000-000000000002"
	opsUserEmail   = "ops@sitewatch.local"
)

func setupTestApp(t *testing.T) (*App, http.Handler) {
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

	app := &App{
		pg:      pgPool,
		mongo:   mongoClient,
		mongoDB: "sitewatch",
		logger:  slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		httpRequestsTotal, httpRequestDuration,
		shared.NewPostgresPoolCollector(pgPool, "api-test"),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	// Mirrors main()'s handler chain exactly (withCORS(withCorrelationID(...)))
	// — a test that skipped these wrappers wouldn't actually be testing what
	// ships, which is how the two failures below were originally missed.
	handler := withCORS(withCorrelationID(app.logger, routes(app, registry)))
	return app, handler
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestListSites_ReturnsSeedData(t *testing.T) {
	_, handler := setupTestApp(t)

	rec := doJSON(t, handler, http.MethodGet, "/sites", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var sites []Site
	if err := json.Unmarshal(rec.Body.Bytes(), &sites); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 seeded sites, got %d", len(sites))
	}
}

func TestListDevices_FiltersBySite(t *testing.T) {
	_, handler := setupTestApp(t)

	rec := doJSON(t, handler, http.MethodGet, "/devices?site_id="+bangkokSiteID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var devices []Device
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(devices) != 4 {
		t.Fatalf("expected 4 seeded devices for Bangkok Data Center, got %d", len(devices))
	}
	for _, d := range devices {
		if d.SiteID != bangkokSiteID {
			t.Fatalf("device %s belongs to a different site than requested", d.ID)
		}
	}
}

func TestGetDeviceTelemetry_ReadsFromMongoChronologically(t *testing.T) {
	app, handler := setupTestApp(t)
	ctx := context.Background()

	coll := app.mongo.Database(app.mongoDB).Collection("telemetry_raw")
	base := time.Now().UTC().Add(-1 * time.Minute)
	for i := 0; i < 3; i++ {
		_, err := coll.InsertOne(ctx, bson.M{
			"device_id":   tempSensor01ID,
			"site_id":     bangkokSiteID,
			"device_type": "temp_sensor",
			"readings":    bson.M{"temperature_c": float64(20 + i)},
			"ts":          base.Add(time.Duration(i) * time.Second),
			"received_at": base.Add(time.Duration(i) * time.Second),
		})
		if err != nil {
			t.Fatalf("seed telemetry_raw: %v", err)
		}
	}

	rec := doJSON(t, handler, http.MethodGet, "/devices/"+tempSensor01ID+"/telemetry?limit=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var points []TelemetryPoint
	if err := json.Unmarshal(rec.Body.Bytes(), &points); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 telemetry points, got %d", len(points))
	}
	for i, p := range points {
		// Catches BSON field-mapping bugs (a Go field with no `bson` tag
		// silently decodes to its zero value instead of an error) that a
		// count-and-ordering check alone would miss.
		if p.DeviceID != tempSensor01ID {
			t.Fatalf("point %d: expected device_id %q, got %q", i, tempSensor01ID, p.DeviceID)
		}
		if p.ReceivedAt.IsZero() {
			t.Fatalf("point %d: received_at decoded as zero value", i)
		}
	}
	for i := 0; i < len(points)-1; i++ {
		if points[i].Ts.After(points[i+1].Ts) {
			t.Fatalf("expected chronological order, point %d (%v) is after point %d (%v)",
				i, points[i].Ts, i+1, points[i+1].Ts)
		}
	}
}

func TestAlertAcknowledgeThenResolve_FullWorkflow(t *testing.T) {
	app, handler := setupTestApp(t)
	ctx := context.Background()

	var ruleID string
	if err := app.pg.QueryRow(ctx,
		`SELECT id FROM alert_rules WHERE device_id = $1 AND metric = 'temperature_c' AND severity = 'warning'`,
		tempSensor01ID,
	).Scan(&ruleID); err != nil {
		t.Fatalf("find seeded alert_rule: %v", err)
	}

	var alertID string
	if err := app.pg.QueryRow(ctx, `
		INSERT INTO alerts (alert_rule_id, device_id, triggered_value, severity, status)
		VALUES ($1, $2, 29.5, 'warning', 'open') RETURNING id`,
		ruleID, tempSensor01ID,
	).Scan(&alertID); err != nil {
		t.Fatalf("seed alert: %v", err)
	}

	var userID string
	if err := app.pg.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, opsUserEmail).Scan(&userID); err != nil {
		t.Fatalf("find seeded user: %v", err)
	}

	// acknowledge
	rec := doJSON(t, handler, http.MethodPost, "/alerts/"+alertID+"/acknowledge", AckInput{UserID: userID})
	if rec.Code != http.StatusOK {
		t.Fatalf("acknowledge: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var acked Alert
	_ = json.Unmarshal(rec.Body.Bytes(), &acked)
	if acked.Status != "acknowledged" || acked.AcknowledgedBy == nil || *acked.AcknowledgedBy != userID {
		t.Fatalf("unexpected alert after acknowledge: %+v", acked)
	}

	// acknowledging again must conflict, not silently succeed
	rec = doJSON(t, handler, http.MethodPost, "/alerts/"+alertID+"/acknowledge", AckInput{UserID: userID})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 re-acknowledging, got %d: %s", rec.Code, rec.Body.String())
	}

	// resolve
	rec = doJSON(t, handler, http.MethodPost, "/alerts/"+alertID+"/resolve", AckInput{UserID: userID})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resolved Alert
	_ = json.Unmarshal(rec.Body.Bytes(), &resolved)
	if resolved.Status != "resolved" || resolved.ResolvedBy == nil || *resolved.ResolvedBy != userID {
		t.Fatalf("unexpected alert after resolve: %+v", resolved)
	}
}

func TestAlertRuleCRUD(t *testing.T) {
	_, handler := setupTestApp(t)

	create := AlertRuleInput{
		DeviceID:  tempSensor01ID,
		Metric:    "temperature_c",
		Operator:  "gt",
		Threshold: 40,
		Severity:  "critical",
	}
	rec := doJSON(t, handler, http.MethodPost, "/alert-rules", create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created AlertRule
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ID == "" || created.Threshold != 40 {
		t.Fatalf("unexpected created rule: %+v", created)
	}

	update := create
	update.Threshold = 45
	rec = doJSON(t, handler, http.MethodPatch, "/alert-rules/"+created.ID, update)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updated AlertRule
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Threshold != 45 {
		t.Fatalf("expected threshold 45 after update, got %v", updated.Threshold)
	}

	rec = doJSON(t, handler, http.MethodDelete, "/alert-rules/"+created.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteAlertRule_RestrictedWhenAlertsReferenceIt(t *testing.T) {
	app, handler := setupTestApp(t)
	ctx := context.Background()

	var ruleID string
	if err := app.pg.QueryRow(ctx,
		`SELECT id FROM alert_rules WHERE device_id = $1 LIMIT 1`, tempSensor01ID,
	).Scan(&ruleID); err != nil {
		t.Fatalf("find seeded alert_rule: %v", err)
	}
	if _, err := app.pg.Exec(ctx, `
		INSERT INTO alerts (alert_rule_id, device_id, triggered_value, severity, status)
		VALUES ($1, $2, 99, 'warning', 'open')`, ruleID, tempSensor01ID); err != nil {
		t.Fatalf("seed alert: %v", err)
	}

	rec := doJSON(t, handler, http.MethodDelete, "/alert-rules/"+ruleID, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 (ON DELETE RESTRICT), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMetricsEndpoint_ReflectsRealRequestActivity(t *testing.T) {
	_, handler := setupTestApp(t)

	// httpRequestsTotal is the same package-level var main() registers —
	// process-global, so other tests in this binary may have already
	// incremented it. Assert on the delta this test's own requests cause,
	// not an absolute value (that was the original version of this test,
	// and it was flaky exactly because of this).
	metric := httpRequestsTotal.WithLabelValues("GET", "/sites", "200")
	before := testutil.ToFloat64(metric)

	// Drive some real traffic through the real route tree first — this is
	// what actually proves the collector got registered and instrument()
	// is wired into routes(), not just that the exposition format parses.
	for i := 0; i < 3; i++ {
		rec := doJSON(t, handler, http.MethodGet, "/sites", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /sites: expected 200, got %d", rec.Code)
		}
	}

	if delta := testutil.ToFloat64(metric) - before; delta != 3 {
		t.Fatalf("expected sitewatch_api_http_requests_total{method=GET,path=/sites,status=200} to increase by 3, increased by %v", delta)
	}

	rec := doJSON(t, handler, http.MethodGet, "/metrics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	// Names only, not values — the values are equally process-global.
	for _, want := range []string{
		"sitewatch_api_http_requests_total",
		"sitewatch_api_http_request_duration_seconds",
		"sitewatch_postgres_pool_max_conns",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected /metrics output to contain %q, got:\n%s", want, body)
		}
	}
}

func TestCORS_ReflectsAllowedOriginsOnly(t *testing.T) {
	_, handler := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/sites", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected the allow-listed 127.0.0.1 origin to be reflected, got %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/sites", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin for a non-allow-listed origin, got %q", got)
	}
}
