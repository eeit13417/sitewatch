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
| Git / Docker / CI-CD | Branch strategy, docker-compose, GitHub Actions |
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
- No rate limiting — a single client can call any endpoint as fast as it wants. Reasonable follow-up for Phase 5 (CI/CD & production hardening) alongside the free-tier deploy step.
- Mosquitto broker allows anonymous connect/publish/subscribe (`infra/mosquitto/mosquitto.conf`) — fine for a broker only reachable on localhost/docker-network in this setup, called out there as dev-only.

**Phase 3 — Frontend dashboard** ✅ verified end-to-end against the real running stack (infra + ingestion + api + Vite dev server, Playwright driving a real browser)
- React + TS + Vite, design in [`docs/frontend-design.md`](frontend-design.md): site overview (`/`), per-site device list (`/sites/:id`), device detail with a polled telemetry chart + recent alert history (`/devices/:id`), and the full alert workflow — list/filter/acknowledge/resolve — on `/alerts`
- TanStack Query for all server state (polling via `refetchInterval`, not WebSocket — see the design doc for why), `react-router` for real routes, `recharts` for the telemetry chart
- No auth UI (nothing to authenticate against yet) and no WebSocket push (Phase 4) — both deliberate, matching the backend's own documented scope
- E2E tests (Playwright, `frontend/e2e/`) run against the real stack via real MQTT-triggered alerts, not a mocked API: alerts list rendering, acknowledge updating status, status-filter behavior, and site→device→detail navigation
- QA caught and fixed one real bug during this phase: the device-detail alert history had no `limit`, so a device with enough history rendered dozens of unpaginated rows — capped to the 10 most recent, full history stays on `/alerts` (see `CLAUDE.md` rule 5)
- Also fixed a Phase 1 seed-data bug this phase's design surfaced: the seeded user (needed as a fixed "acting user" id since there's no login) was inserted with `'u1111111-...'` — not a valid UUID (`u` isn't a hex digit), which silently rolled back the entire `init.sql` transaction, including all the table creation. Fixed to `'33333333-...'`, consistent with the sites/devices seed id pattern.

**Phase 4 — Observability**
- Prometheus metrics: request latency/QPS, MQTT message throughput, DB connection pool stats
- Grafana dashboards
- Structured logs with a correlation/request ID threaded through ingestion → API → alert engine

**Phase 5 — CI/CD hardening**
- GitHub Actions: lint → test → build image → push image → (optional) deploy to a free-tier host
- Branch protection, PR template

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
