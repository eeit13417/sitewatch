# SiteWatch runbook

Quick-reference troubleshooting guide. Each entry: symptom → what to
check → likely fix → the full incident writeup if one exists
(`docs/rca/`). For live numbers, Grafana (`http://localhost:3000`,
dashboard `SiteWatch`) and Prometheus (`http://localhost:9090`) are the
first stop for anything below — this guide tells you what to look for
once you're there, not the whole story on its own.

## `GET /alerts` (or any list endpoint) is slow

**Check**: `EXPLAIN ANALYZE` the query directly against Postgres
(`docker exec -it sitewatch-postgres psql -U sitewatch -d sitewatch`).
Look for `Seq Scan` where you'd expect an `Index Scan` — that means a
column in the `WHERE`/`ORDER BY` has no supporting index.

**Fix**: add the missing index to `infra/postgres/init.sql` (permanent —
applies to every fresh environment from then on). Don't guess which
column; the query plan tells you exactly which one Postgres had to scan
for.

**See**: [`docs/rca/01-missing-index.md`](rca/01-missing-index.md) — a
real example (`alerts.triggered_at`), with before/after `EXPLAIN ANALYZE`
numbers. `scripts/demo-seed-bulk-alerts.sql` can reproduce volume locally
if the problem isn't visible yet at your current row count.

## `ingestion`'s memory/goroutine count keeps climbing

**Check**: `curl http://localhost:8081/debug/pprof/goroutine?debug=1` —
compare the goroutine count over time (a snapshot now, another in 10
minutes). If it's growing with no matching growth in message volume
(`sitewatch_ingestion_messages_received_total`), something's leaking.
The profile's stack traces point at the exact function — look for a
`go func(){ ... }()` that has no matching cancellation path, especially
anything spawned inside `OnConnect` (which can fire many times over a
process's life, not just once at startup).

**Fix**: whatever spawned the leaking goroutine needs an explicit stop
condition, usually tied to `OnConnectionLost` or a `context.Context`
that gets cancelled at the right time.

Separately — if it's actual per-connection memory rather than
goroutines, check whether a connection pool (`MONGO_MAX_POOL_SIZE`,
`POSTGRES_MAX_CONNS`) is unset and defaulting to something large.

**See**: [`docs/rca/02-mqtt-leak-and-pool.md`](rca/02-mqtt-leak-and-pool.md)

## `sitewatch_ingestion_messages_dropped_total` is climbing

**Check**: the Grafana "ingestion — message throughput" panel —
`received/s` flattening at a fixed ceiling while `dropped/s` climbs means
the worker pool has hit its processing capacity, not that something is
crashing. A queue that shows 0 drops isn't necessarily healthy either —
it just means the buffer (`INGESTION_QUEUE_SIZE`, default 256) hasn't
filled yet; sustained excess load will eventually drop regardless of
buffer size.

**Fix**: raise `INGESTION_WORKER_COUNT` (bounded by `POSTGRES_MAX_CONNS`
— going past the pool size just moves the bottleneck to `Acquire()`
instead of the job channel). If the real cause is a slow downstream
dependency rather than raw volume, fix that first — more workers just
means more of them stuck waiting on the same slow dependency.

**See**: [`docs/rca/03-mqtt-backlog.md`](rca/03-mqtt-backlog.md)

## Telemetry is landing (MongoDB has the reading) but no alert fired for an obvious breach

**Check**: `ingestion`'s logs around the time of that reading — look for
`"failed to process message"` or `"update last_seen_at failed"` with a
Postgres connection error. If Postgres was unreachable (or errored) at
exactly that moment, the alert evaluation for that specific reading was
skipped and nothing retries the read side automatically.

**Fix**: nothing to fix retroactively — this reading's evaluation is
gone. Confirm Postgres is healthy now; the *next* reading will evaluate
normally. If this happens often enough to matter, see the RCA's
"Prevention" section for the bigger fix (an outbox/reconciliation table)
that this project deliberately hasn't built yet.

**See**: [`docs/rca/04-mongo-postgres-inconsistency.md`](rca/04-mongo-postgres-inconsistency.md)

## An alert is flapping (create → resolve → create → resolve, rapidly)

**Check**: `sitewatch_ingestion_alert_decisions_total` — a `create` and
`auto_resolve` pair for the same rule within seconds of each other,
repeating. Almost always means a rule's `threshold` sits inside the
device's normal reading noise, not that the debounce logic is broken
(`ALERT_DEBOUNCE_BREACHES`, default 3, should already be absorbing this
for anything but a genuinely fast oscillation).

**Fix**: if it's still flapping despite debounce, either the threshold is
far enough inside the noise band to still cross it 3+ times in a row
periodically (raise the threshold, or raise `ALERT_DEBOUNCE_BREACHES`),
or check whether the *simulator's* configured noise/spike behavior for
that device changed.

**See**: [`docs/rca/05-alert-storm.md`](rca/05-alert-storm.md)

## Rate limited (`429 Too Many Requests`) unexpectedly

**Check**: `Retry-After` header on the response; `api`'s logs for `"rate
limit exceeded"` with the client IP. Defaults are `RATE_LIMIT_RPS=20`,
`RATE_LIMIT_BURST=40` per client IP — comfortably above normal frontend
polling (`frontend/src/hooks/`'s busiest poller refetches every 5s), so
this usually means either a genuine flood or a test/script hammering the
API without realizing it.

**Fix**: raise `RATE_LIMIT_RPS`/`RATE_LIMIT_BURST` if the traffic is
legitimate; otherwise find what's actually sending that much traffic.

**See**: [`docs/deployment-hardening-design.md`](deployment-hardening-design.md)
(Phase 5 — this predates Phase 6 but lives here since it's the same kind
of "here's the tunable knob" reference).

## WSL2 + Docker Desktop: Prometheus targets show `down`

Only relevant if you've somehow gone back to running `api`/`ingestion` as
bare host processes instead of the `docker-compose` services they are as
of Phase 5 — see the note in `infra/prometheus/prometheus.yml` and
[`docs/observability-design.md`](observability-design.md).
