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

**Phase 2 — Backend core services**
- `ingestion`: MQTT subscriber → writes raw telemetry to MongoDB, aggregates into PostgreSQL
- `api`: REST endpoints for device CRUD, telemetry queries, alert queries; OpenAPI spec
- Alert engine: threshold rules (e.g. temp > X, abnormal consumption) → writes alerts
- Unit tests + integration tests (testcontainers against real Postgres/Mongo)

**Phase 3 — Frontend dashboard**
- React + TS: site overview, device list, live trend charts (WebSocket or polling), alert management
- E2E tests for the core flows (view alert, acknowledge alert) with Playwright

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
