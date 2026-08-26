# Observability design (Phase 4)

## Scope

Metrics (Prometheus + Grafana) and structured logs with correlation IDs,
per `docs/PROJECT_PLAN.md`. Deliberately **not** in this phase:

- **Distributed tracing** (OpenTelemetry spans). `ingestion` and `api` don't
  share a request/response relationship — `ingestion` reacts to MQTT
  messages, `api` reacts to HTTP requests — so there's no single call
  chain to trace across them the way there would be between two HTTP
  services. Linking the two properly is a bigger, separate investment
  than what "structured logs with a correlation id" calls for.
- **Containerizing `api`/`ingestion`**. They keep running as local `go run`
  processes, same as every phase so far. Prometheus (in Docker) reaches
  them via `host.docker.internal`. Turning them into images that
  docker-compose runs is explicitly Phase 5's job ("build image, push
  image") — pulling it into Phase 4 would be scope creep from one phase
  into the next.
- **Alertmanager / Prometheus alerting rules**. The application already has
  its own domain alert engine (`ingestion/alerts.go`). Infra-level
  alerting on top of these metrics is a legitimate future addition, not
  part of what was scoped here.
- **Centralized log aggregation** (Loki/ELK). Both services already emit
  structured JSON to stdout; shipping it somewhere centralized is a
  reasonable follow-up, not something the plan asked for.

## Metrics

Namespace `sitewatch`, subsystem per service, standard Prometheus naming
(`_total` counters, `_seconds` histograms).

### `api`

| Metric | Type | Labels | What it answers |
|---|---|---|---|
| `sitewatch_api_http_requests_total` | counter | `method`, `path`, `status` | request volume / error rate |
| `sitewatch_api_http_request_duration_seconds` | histogram | `method`, `path` | latency (matches the plan's "request latency/QPS") |

Populated by one middleware wrapping every route (`api/metrics.go`) — no
handler adds its own instrumentation. `path` uses the route *pattern*
(`/devices/{id}`), not the raw URL, so `/devices/<uuid-1>` and
`/devices/<uuid-2>` don't explode into separate label values (an
unbounded-cardinality mistake Prometheus is notoriously easy to make).

### `ingestion`

| Metric | Type | Labels | What it answers |
|---|---|---|---|
| `sitewatch_ingestion_messages_received_total` | counter | — | MQTT throughput |
| `sitewatch_ingestion_messages_dropped_total` | counter | — | backpressure — the existing "job queue full" log line, now quantified |
| `sitewatch_ingestion_message_processing_duration_seconds` | histogram | — | time from dequeue to fully processed (Mongo write + Postgres update + alert eval) |
| `sitewatch_ingestion_alert_decisions_total` | counter | `action` (`create`/`auto_resolve`) | alert engine activity |

`ingestion` has no HTTP surface today — it gets a minimal `net/http`
server exposing `/healthz` and `/metrics` only, mirroring `api`'s existing
pattern.

### Shared: Postgres pool stats

Both services pool Postgres connections via `pgxpool`, and `Pool.Stat()`
already exposes `AcquiredConns`/`IdleConns`/`MaxConns`/etc. Rather than a
periodically-updated gauge (which is stale between updates), this is a
`prometheus.Collector` implementation (`shared/metrics.go`) that reads
`pool.Stat()` live on every scrape:

```go
type PostgresPoolCollector struct {
    pool      *pgxpool.Pool
    subsystem string // "api" or "ingestion"
}

func (c *PostgresPoolCollector) Describe(ch chan<- *prometheus.Desc)
func (c *PostgresPoolCollector) Collect(ch chan<- prometheus.Metric)
```

A struct wrapping the pool reference and implementing an interface via
methods is the right shape here — genuinely stateful (holds the pool), and
Go's version of "the object is what satisfies the interface," not
inheritance. Both `api` and `ingestion` register one instance instead of
each hand-rolling the same gauge-update loop.

Also registered: Go's standard `collectors.NewGoCollector()` and
`NewProcessCollector()` (goroutine count, GC pauses, memory, open FDs) —
free, and genuinely useful given `ingestion`'s worker-pool concurrency
model.

## Correlation IDs

Not a cross-process trace — each service tags its own unit of work
consistently:

- **`ingestion`**: one correlation ID generated per MQTT message at the top
  of `process()`, attached to a child logger (`logger.With("correlation_id",
  id)`) that gets threaded through `WriteTelemetry` → `MaybeUpdateLastSeen`
  → rule evaluation → `ApplyDecisions`. Grepping one id shows the complete
  story of what happened to one message — directly useful for the Phase 6
  incident drills.
- **`api`**: standard `X-Request-ID` middleware — reuses the header if the
  caller sent one, otherwise generates one, puts it in `context.Context`,
  every log line for that request includes it, and it's echoed back in the
  response header so the frontend could display/report it.
- ID generation: `shared/correlation.go`, a short random hex string via
  `crypto/rand` — no new dependency for something this small (the project
  already avoids adding a UUID library where a lighter primitive does the
  job).

## Infra

`infra/docker-compose.yml` gains two services:

- **`prometheus`** — scrapes `api:8080/metrics` and `ingestion:8081/metrics`
  every 5s. Config lives at `infra/prometheus/prometheus.yml`, checked in
  — not clicked together.

  At the time this phase landed, `api`/`ingestion` weren't containerized
  yet (that was explicitly Phase 5's job — see below), so this had to
  reach them as bare host processes instead of compose services, which on
  Docker Desktop + WSL2 meant `host.docker.internal` didn't work
  (resolves to the Docker Desktop VM's own gateway, which can't reach into
  the WSL2 network namespace) and the config pointed at the WSL2 distro's
  own IP as a documented, temporary workaround. Phase 5
  (`docs/deployment-hardening-design.md`) containerized both services, so
  Prometheus now reaches them by service name like everything else and
  that workaround is gone.
- **`grafana`** — datasource and dashboard are **provisioned from files**
  (`infra/grafana/provisioning/`), not manually configured through the UI
  and lost on container recreation. One dashboard,
  `infra/grafana/provisioning/dashboards/sitewatch.json`, with panels for:
  request rate/latency (api), message throughput/drop rate (ingestion),
  alert decision rate, and Postgres pool utilization for both services.

`ingestion` needs a second port (`8081`, `INGESTION_PORT` env var) for its
new `/metrics` + `/healthz` server, alongside `api`'s existing `8080`.

## Testing

- Unit: correlation ID generator (format/uniqueness).
- Integration: after driving some activity through `api`/`ingestion`
  against real Postgres (testcontainers), `GET /metrics` contains the
  expected metric names — catches "the collector never actually got
  registered" class of bugs, which a purely visual Grafana check wouldn't.
- Manual end-to-end: full stack + Prometheus + Grafana up, simulator
  generating real traffic, confirm real numbers move in Prometheus's own
  query UI and the provisioned Grafana dashboard — same rigor as every
  prior phase.
