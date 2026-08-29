# Architecture

## System overview

```
┌─────────────┐        MQTT         ┌──────────────┐
│  simulated   │ ───────────────────▶│  Mosquitto   │
│  devices     │  sitewatch/{site}/  │   broker     │
│ (simulator/) │  {device}/telemetry └──────┬───────┘
└─────────────┘                             │ subscribe
                                             ▼
                                    ┌──────────────────┐
                                    │    ingestion      │
                                    │  (Go, worker pool)│
                                    └───┬──────────┬────┘
                                        │          │
                          write raw    │          │  read rules,
                          telemetry    │          │  write alert
                                        ▼          ▼  decisions
                                 ┌──────────┐  ┌──────────┐
                                 │ MongoDB  │  │PostgreSQL│
                                 │telemetry_│  │sites,    │
                                 │   raw    │  │devices,  │
                                 └──────────┘  │alert_    │
                                        ▲       │rules,    │
                                        │       │alerts,   │
                                        │       │users     │
                                        │       └────┬─────┘
                                        │            │
                                        │   read     │ read/write
                                        │            ▼
                                 ┌──────┴───────────────┐
                                 │         api            │
                                 │  (Go, REST, stateless) │
                                 └──────────┬─────────────┘
                                            │ HTTP (JSON)
                                            ▼
                                 ┌───────────────────────┐
                                 │       frontend          │
                                 │ (React + TS, TanStack   │
                                 │  Query polling)         │
                                 └───────────────────────┘

Cross-cutting, both api and ingestion:
  Prometheus (metrics, scraped every 5s) ──▶ Grafana (dashboards)
  log/slog structured JSON logs, correlation-id per request/message
```

## Why two databases

- **PostgreSQL** — anything relational and low-volume: `sites`,
  `devices`, `alert_rules`, `alerts`, `users`. Foreign keys and CHECK
  constraints matter here (an alert must reference a real device, a
  severity must be one of a fixed set) — this is exactly what an RDBMS
  is for.
- **MongoDB** — `telemetry_raw`, one document per reading, high write
  volume, read pattern is always "this device, most recent N" (no joins,
  no cross-document transactions needed). A document store fits the
  access pattern better than forcing time-series readings into a
  relational schema.

Cross-referenced by `device_id` (a UUID minted in Postgres, carried into
every Mongo document and every MQTT payload) — not by a foreign key
Postgres could enforce, since Mongo can't participate in that constraint.
This is a deliberate, accepted trade documented in
`docs/mqtt-contract.md`.

## Why `ingestion` and `api` are separate services

`ingestion` is a long-running MQTT consumer with no HTTP surface beyond
`/healthz` and `/metrics`; `api` is a stateless HTTP server with no MQTT
involvement. They share a database layer (`shared/`, a Go module both
`go.mod`s pull in via a `replace` directive — CLAUDE.md rule 3) but have
nothing else in common architecturally, and scale along different axes
(`ingestion` by message volume, `api` by request volume). Splitting them
avoids one from blocking the other under load, and each has its own
resource bounds (worker pool, connection pool) sized independently.

## Request/message flow

**Telemetry → alert** (event-driven, `ingestion`):
1. Simulator publishes to `sitewatch/{site_id}/{device_id}/telemetry`.
2. `ingestion`'s MQTT client enqueues the message onto a bounded channel
   (`INGESTION_QUEUE_SIZE`); full queue drops rather than blocks
   (`docs/rca/03-mqtt-backlog.md`).
3. A worker pool (`INGESTION_WORKER_COUNT`) dequeues and processes each
   message: write to MongoDB, debounced `devices.last_seen_at` update,
   then the alert engine (`docs/alert-engine.md`) evaluates the device's
   enabled rules against the reading and writes any resulting
   create/auto-resolve decisions to Postgres.

**Dashboard → data** (request-driven, `api` + `frontend`):
1. `frontend` polls `api`'s REST endpoints (TanStack Query
   `refetchInterval`, not WebSocket — see `docs/frontend-design.md` for
   why) — sites/devices from Postgres, telemetry history from MongoDB,
   the full alert workflow (list/filter/acknowledge/resolve) against
   Postgres.
2. Every request is rate-limited per client IP
   (`docs/deployment-hardening-design.md`), CORS-checked against an
   allow-list, and tagged with a correlation id that threads through
   every log line for that request.

## Deployment shape

`api`, `ingestion`, `postgres`, `mongodb`, `mosquitto`, `prometheus`, and
`grafana` all run as `docker-compose` services on one shared network
(`infra/docker-compose.yml`) — `api`/`ingestion` reach the databases and
each other by compose service name, same as the databases reach each
other. `api`/`ingestion` ship as minimal multi-stage Docker images
(`golang:1.25` builder → `gcr.io/distroless/static-debian12:nonroot`
runtime — no shell, non-root by default,
`docs/deployment-hardening-design.md`), built and pushed to GitHub
Container Registry on every `main` commit by CI. `frontend` isn't
containerized yet — no live deploy target exists to run it on, so local
dev keeps using the Vite dev server (same doc, "deliberately not in this
phase").

## Non-goals, by design

- **No auth** — every `api` endpoint is open. Out of scope for this
  project's depth (`docs/PROJECT_PLAN.md`'s known-gaps list).
- **No distributed tracing** — `ingestion` reacts to MQTT messages,
  `api` reacts to HTTP requests; there's no single call chain connecting
  them the way there would be between two HTTP services
  (`docs/observability-design.md`).
- **No live public deployment (CD)** — publishing a Docker image (CI)
  isn't the same as automatically deploying it somewhere reachable;
  deliberately deferred, not abandoned
  (`docs/deployment-hardening-design.md`).
- **No outbox/reconciliation between MongoDB and PostgreSQL** — a
  transient Postgres failure after a MongoDB write has already committed
  can silently skip one alert evaluation; a bounded retry narrows this
  but doesn't close it entirely
  (`docs/rca/04-mongo-postgres-inconsistency.md`).
