# SiteWatch — Project Plan

Practice project for the Delta Electronics (Thailand) Software & Digital Enablement — Software Application Engineer role. Domain: smart energy / data center / building automation monitoring.

## JD requirement → project mapping

| JD requirement | How this project demonstrates it |
|---|---|
| JavaScript/TypeScript, React | Dashboard frontend (Phase 3) |
| Go | `api` and `ingestion` services |
| PostgreSQL | Sites, devices, alert rules, alerts, users |
| MongoDB | Raw telemetry, application logs |
| RESTful API | `api` service, documented with OpenAPI |
| Monitoring/logging/observability | Prometheus + Grafana, structured JSON logs with correlation IDs (Phase 4) |
| Troubleshooting / RCA | Deliberately injected production issues + written RCAs (Phase 6) |
| Git / Docker / CI-CD | Branch strategy, docker-compose, multi-stage Dockerfiles, GitHub Actions build+push to GHCR (Phase 5) |
| Event-driven / MQTT | Mosquitto broker, ingestion service subscribes and processes telemetry |
| Automated testing | Unit (Go testing/Jest), integration (testcontainers), E2E (Playwright) |

## Phases

**Phase 0 — Infrastructure skeleton** ✅
Git repo, branch/PR conventions, Docker Compose (Postgres, MongoDB, Mosquitto), placeholder Go services, basic CI (vet/fmt/build/test).

**Phase 1 — Data layer and device simulator** ✅
- PostgreSQL schema: `sites`, `devices`, `alert_rules`, `alerts`, `users` (`infra/postgres/init.sql`), reviewed and hardened (derived MQTT topic instead of stored, explicit `ON DELETE` policy, `resolved_by`, `alerts.severity` CHECK)
- MongoDB document contract for `telemetry_raw` / `application_logs` defined in [`docs/mqtt-contract.md`](mqtt-contract.md) (write path itself lands in Phase 2)
- MQTT topic + payload contract documented; device simulator (`simulator/`, Node + TS) publishes realistic drifting telemetry for all 7 seeded devices, with occasional threshold-testing spikes and simulated offline dropout
- Verified end-to-end against the real containers: schema, seed data, constraint enforcement, and live MQTT traffic all checked with `docker compose` + `mosquitto_sub`

**Phase 2 — Backend core services** ✅ verified end-to-end against real containers (testcontainers suite + a manual pass: fresh docker-compose stack, ingestion + api + the real simulator, full alert lifecycle exercised live)
- `ingestion`: bounded worker-pool MQTT subscriber (`sitewatch/+/+/telemetry`) → writes raw telemetry to MongoDB `telemetry_raw`, debounced `devices.last_seen_at` updates to PostgreSQL, then runs the alert engine
- Alert engine (`ingestion/alerts.go`, design in [`docs/alert-engine.md`](alert-engine.md)): pure rule-evaluation function + a dedup/auto-resolve state machine keyed on "at most one active alert per rule" — deliberately no debounce/hysteresis yet, that gap is a planned Phase 6 incident
- `api`: REST endpoints per [`docs/openapi.yaml`](openapi.yaml) — read-only `sites`/`devices`/`devices/{id}/telemetry` (the one endpoint reading MongoDB), full alert workflow (`list`, `acknowledge`, `resolve`), full `alert_rules` CRUD
- Unit tests (`ingestion/alerts_test.go`, pure logic, no DB) + integration tests (`*/integration_test.go`, `testcontainers-go`, real Postgres seeded from the actual `infra/postgres/init.sql` + real MongoDB) — both wired into CI (`integration-test` job)
- **Hardening pass** (see `CLAUDE.md` engineering standards): extracted `shared/` (env loading, Postgres/Mongo connection setup, tunable pool size) so `api` and `ingestion` stop duplicating it; added a MongoDB index on `telemetry_raw` (device_id, ts) — that query was an unindexed collection scan before; added `golangci-lint` + `gosec` to CI, which caught and fixed a real Slowloris DoS gap (no `ReadHeaderTimeout` on the API's `http.Server`) plus a few unchecked-error findings; added CORS for the Phase 3 frontend; fixed a bug where `deleteAlertRule` mislabeled any DB error as the FK-conflict case instead of checking the actual Postgres error code

**Known deferred security gaps (Phase 2)** — written down deliberately, not silently skipped:
- No authentication on the API — every endpoint is open. Out of scope because the JD doesn't call out auth as a target skill and this project's depth is meant to be elsewhere; would need to land before this was ever exposed beyond localhost.
- ~~No rate limiting~~ — closed in Phase 5, see below.
- Mosquitto broker allows anonymous connect/publish/subscribe (`infra/mosquitto/mosquitto.conf`) — fine for a broker only reachable on localhost/docker-network in this setup, called out there as dev-only.

**Phase 3 — Frontend dashboard** ✅ verified end-to-end against the real running stack (infra + ingestion + api + Vite dev server, Playwright driving a real browser)
- React + TS + Vite, design in [`docs/frontend-design.md`](frontend-design.md): site overview (`/`), per-site device list (`/sites/:id`), device detail with a polled telemetry chart + recent alert history (`/devices/:id`), and the full alert workflow — list/filter/acknowledge/resolve — on `/alerts`
- TanStack Query for all server state (polling via `refetchInterval`, not WebSocket — see the design doc for why), `react-router` for real routes, `recharts` for the telemetry chart
- No auth UI (nothing to authenticate against yet) and no WebSocket push (Phase 4) — both deliberate, matching the backend's own documented scope
- E2E tests (Playwright, `frontend/e2e/`) run against the real stack via real MQTT-triggered alerts, not a mocked API: alerts list rendering, acknowledge updating status, status-filter behavior, and site→device→detail navigation
- QA caught and fixed one real bug during this phase: the device-detail alert history had no `limit`, so a device with enough history rendered dozens of unpaginated rows — capped to the 10 most recent, full history stays on `/alerts` (see `CLAUDE.md` rule 5)
- Also fixed a Phase 1 seed-data bug this phase's design surfaced: the seeded user (needed as a fixed "acting user" id since there's no login) was inserted with `'u1111111-...'` — not a valid UUID (`u` isn't a hex digit), which silently rolled back the entire `init.sql` transaction, including all the table creation. Fixed to `'33333333-...'`, consistent with the sites/devices seed id pattern.

**Phase 4 — Observability** ✅ verified end-to-end against the real running stack (full docker-compose including Prometheus + Grafana, real `api`/`ingestion` binaries, live traffic driven through both HTTP and MQTT)
- Design in [`docs/observability-design.md`](observability-design.md). Prometheus metrics via `client_golang`: `api` request rate/duration by route (`sitewatch_api_http_requests_total`, `..._http_request_duration_seconds`), `ingestion` message throughput/drops/processing duration (`sitewatch_ingestion_messages_received_total`, `..._dropped_total`, `..._message_processing_duration_seconds`), alert-engine decisions by action (`sitewatch_ingestion_alert_decisions_total`), plus Go/process runtime metrics for both services
- `shared/metrics.go`'s `PostgresPoolCollector` implements `prometheus.Collector` directly against `pgxpool.Pool.Stat()` (acquired/idle/total/max conns, new conns, acquires) — one implementation used by both `api` and `ingestion` (`CLAUDE.md` rule 3), each registered with a `service` label
- Structured logs (`log/slog`) now carry a correlation ID: per-HTTP-request in `api` (generated or echoed from an incoming `X-Request-ID`, stored in request context, returned in the response header), per-MQTT-message in `ingestion` — every log line for that request/message shares the same id, making a single flow traceable across log lines
- Grafana provisioned entirely from files (`infra/grafana/provisioning/`), not clicked together in the UI: one datasource, one dashboard (`sitewatch-overview`) with 6 panels covering all of the above
- **Verification found and fixed a real WSL2 + Docker Desktop networking gap**: `host.docker.internal` resolves inside the Docker Desktop VM to its own internal gateway, which cannot reach a service bound inside the WSL2 distro — the two are separate network namespaces (`--network host` doesn't bridge this either, since Docker Desktop's containers never actually join the WSL2 distro's network namespace). Fixed for local dev by pointing Prometheus's `static_configs` at the WSL2 distro's own address instead; documented as WSL2-specific in `infra/prometheus/prometheus.yml` since it changes if the distro restarts, and goes away entirely once Phase 5 containerizes `api`/`ingestion` (scraped by compose service name at that point)
- Also fixed, opportunistically, a live CORS bug hit earlier in the session in the same file being touched for this phase's request-logging middleware: `api/main.go` reflected only `http://localhost:5173`, so a browser on `http://127.0.0.1:5173` was silently rejected. Now reads a comma-separated `CORS_ALLOWED_ORIGINS` allow-list (defaulting to both) and only ever reflects the actual matching origin, never a wildcard
- **Non-goals, deliberately deferred**: distributed tracing, containerizing `api`/`ingestion` (Phase 5), Alertmanager/paging, log aggregation — all noted in the design doc
- Testing note worth keeping: Prometheus `client_golang` metrics declared as package-level vars are process-global, so asserting an *absolute* counter value in a test is flaky once other tests in the same binary touch the same route — assert the *delta* around the action instead (`testutil.ToFloat64` before/after). Confirmed the fix holds under `-shuffle=on`.

**Phase 5 — Docker images & CI hardening** ✅ verified end-to-end (full `docker compose up --build`, live burst test against the running `api` container, image build confirmed locally with the same Dockerfiles CI uses)
- Design in [`docs/deployment-hardening-design.md`](deployment-hardening-design.md). `api`/`ingestion` get multi-stage Dockerfiles (`golang:1.25` builder → `gcr.io/distroless/static-debian12:nonroot` runtime — no shell, non-root by default) and become real `infra/docker-compose.yml` services, reachable by each other and by Prometheus via compose service name
- This retires the Phase 4 WSL2 workaround in `infra/prometheus/prometheus.yml` entirely — confirmed live: both scrape targets `UP` via `api:8080`/`ingestion:8081`, and the MQTT reconnect churn `ingestion` was logging as a bare host process (WSL2 networking quirk) is gone too now that it's on the same compose network as `mosquitto`
- CI gains a `docker-build-push` job: on push to `main` only, after `lint-and-build`/`integration-test` pass, builds and pushes both images to GitHub Container Registry, tagged with the commit SHA and `latest` — publishing an image, not deploying it anywhere (see below)
- `.github/PULL_REQUEST_TEMPLATE.md` added; branch protection on `main` (require CI to pass before merge)
- Closed the Phase 2 rate-limiting gap: per-IP token bucket (`golang.org/x/time/rate`), in-memory, `RATE_LIMIT_RPS`/`RATE_LIMIT_BURST` env-configurable, idle visitors swept by a background cleanup loop so the tracking map can't grow unbounded. Verified against the real running container with an actual burst of requests (`api/ratelimit.go`, `docs/deployment-hardening-design.md` for the per-IP/in-memory/`RemoteAddr` reasoning)
- **CD (automatic deployment to a live environment) — deliberately deferred, not abandoned.** Publishing an image to a registry is delivery, not deployment; an actual live target needs external hosting (no current free-tier PaaS runs this project's full multi-service stack — Postgres + MongoDB + an MQTT broker + two Go services — for free in one place), which is real scope of its own. Revisit once the rest of the phased plan is further along; `docker compose up` locally plus a Phase 6 screen recording is the plan for showing it working in the meantime.

**Phase 6 — Production-incident drills + documentation**
Deliberately inject and then investigate:
1. Missing index → slow query (fix with `EXPLAIN ANALYZE`, measure before/after)
2. Leaked MQTT subscription / oversized connection pool → memory growth (diagnose with `pprof`)
3. Consumer slower than publisher → MQTT backlog (show monitoring + backpressure/scaling fix)
4. Mongo write succeeds, Postgres aggregate fails → inconsistency (discuss retry/compensation)
5. Alert storm from a bad threshold → design debounce/dedup logic

Write a 1-page RCA per incident: symptom → investigation → root cause → fix → prevention.

Final deliverables: README, architecture doc, OpenAPI docs, troubleshooting guide/runbook, RCA write-ups, test coverage report, CI pipeline evidence, demo recording.

## Interview story mapping

| Likely question | Material to use |
|---|---|
| "Walk me through a hard bug you debugged" | One of the Phase 6 RCAs |
| "How do you ensure code quality?" | Test pyramid (unit/integration/E2E) + CI gates |
| "How do you prioritize with cross-functional teams?" | Backlog/issue prioritization approach used across phases |
| "What observability tools have you used?" | Prometheus/Grafana + structured logs + correlation IDs |
| "Explain event-driven architecture / message queues" | MQTT ingestion design, backpressure handling, delivery guarantees |
