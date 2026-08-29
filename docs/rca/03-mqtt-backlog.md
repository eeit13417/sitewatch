# RCA 3: Consumer slower than publisher → MQTT backlog

## Symptom

`ingestion` already had backpressure handling in place: a bounded job
channel (`queueSize`, default 256) between the MQTT subscriber and the
worker pool, with a non-blocking send — full queue means drop, not block
(`sitewatch_ingestion_messages_dropped_total`, added in Phase 4). What
hadn't been demonstrated was what actually happens once a consumer falls
behind its publisher for a sustained period: does the drop mechanism
actually kick in, and what does it look like on the dashboards someone
would be watching?

Also surfaced along the way: `workerCount`/`queueSize` were hardcoded
constants (`workerCount = 4`, `queueSize = 256`) — not configurable per
CLAUDE.md rule 2, and specifically blocking this drill, since reproducing
"consumer slower than publisher" needed the ability to deliberately run
with fewer workers.

## Injection

Two temporary, clearly-marked changes, reverted immediately after
capturing evidence:

1. `ingestion/main.go` — `workerCount`/`queueSize` made configurable via
   `INGESTION_WORKER_COUNT`/`INGESTION_QUEUE_SIZE` (kept permanently —
   this is a real fix, not just a drill enabler).
2. A `DRILL_SLOW_PROCESSING_MS` env-gated `time.Sleep` at the top of
   `process()`, simulating a consumer that's become slower than its
   publisher (e.g. a downstream dependency degrading) — removed
   entirely once the drill was done.

Ran with `INGESTION_WORKER_COUNT=1` and `DRILL_SLOW_PROCESSING_MS=200`
(a single worker capped at 5 msg/s), against the simulator publishing 7
devices every 500ms (14 msg/s) — arrival at roughly 3x the consumer's
capacity.

## Investigation

`sitewatch_ingestion_messages_received_total` (rate) climbed and
flattened at exactly **5/s** — the single worker's hard ceiling, visible
directly on the existing Grafana "ingestion — message throughput" panel
(Phase 4). For the first ~75 seconds, `messages_dropped_total` stayed at
0 — the 256-slot queue was absorbing the gap between 14/s arriving and
5/s draining. Once that buffer filled, `dropped/s` started climbing too,
plateauing around **4.5/s** (final counters after a ~90s burst: 449
received, 130 dropped):

```
received/s: 0 → climbs → flatlines at 5/s (the worker's ceiling)
dropped/s:  0 → (queue draining the backlog) → climbs once the queue is full → ~4.5/s
```

The queue didn't fail instantly — it bought roughly a minute of grace
before the drop-rate line on the dashboard even appeared. That's the
useful, non-obvious finding: a healthy-looking "0 drops" reading doesn't
mean there's no problem building, it can mean the buffer just hasn't
filled yet.

## Root cause

Not a code defect — the drop-when-full behavior worked exactly as
designed (Phase 4). The actual gap was operational: no documented
guidance for choosing `workerCount`/`queueSize`, and until this drill,
the constants weren't even reachable without a code change to test
different values.

## Fix

- `INGESTION_WORKER_COUNT` / `INGESTION_QUEUE_SIZE` env vars (defaults
  unchanged: 4 workers, queue of 256) — a real capacity problem can now
  be responded to by raising `INGESTION_WORKER_COUNT` without a
  redeploy-from-source cycle.
- Reverted to defaults after the drill; confirmed a normal burst (default
  config) shows 0 drops.

## Prevention

- **Signal to watch**: `rate(sitewatch_ingestion_messages_dropped_total[5m]) > 0`
  sustained for more than a scrape interval or two is the point to scale
  `INGESTION_WORKER_COUNT` up, not wait for it to get worse — a
  reasonable candidate for a Phase-5-style alerting rule if/when
  Alertmanager is ever added (explicitly out of scope per
  `docs/observability-design.md`).
- **Choosing `workerCount`**: bounded by two things — Postgres pool size
  (`POSTGRES_MAX_CONNS`, default 10) since each worker can hold a
  connection mid-query, and realistic per-message latency. Going above
  the pool size just means workers start queuing on `Acquire()` instead
  of on the job channel — moving the backpressure point, not removing it.
- **Queue size** is grace period, not a fix: a bigger queue delays when
  drops start, it doesn't raise the sustained throughput ceiling (that's
  `workerCount` × messages/sec/worker). Sized too large, it also means a
  genuinely stuck consumer takes longer to *show* as unhealthy.
