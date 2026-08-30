# RCA 1: Missing index on `alerts.triggered_at`

## Symptom

`GET /alerts` with no filters — the default landing view of the `/alerts`
page — issues:

```sql
SELECT ... FROM alerts a ORDER BY a.triggered_at DESC LIMIT 50 OFFSET 0;
```

At the row counts this project has run with so far (a few thousand seeded
alerts), that's imperceptible. At real production volume, this is a full
table scan and sort on every single page load of the busiest page in the
dashboard.

## Investigation

`\d alerts` on the running database showed indexes on `id` (PK),
`device_id`, and `status` — nothing on `triggered_at`, and nothing
composite. Seeded 300,000 synthetic rows (`scripts/demo-seed-bulk-alerts.sql`,
via `alert_rules CROSS JOIN generate_series(1, 100000)`, marked with
`triggered_value = -1` so they're trivially distinguishable from real
alerts and safe to delete afterward) to make the cost visible, then ran
the exact query above through `EXPLAIN (ANALYZE, BUFFERS)`:

```
 Limit  (cost=10215.54..10221.37 rows=50 width=126) (actual time=14.015..16.643 rows=50 loops=1)
   Buffers: shared hit=4390
   ->  Gather Merge  (cost=10215.54..36668.98 rows=226728 width=126) (actual time=14.014..16.639 rows=50 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         ->  Sort  (cost=9215.51..9498.92 rows=113364 width=126) (actual time=11.660..11.661 rows=42 loops=3)
               Sort Key: triggered_at DESC
               Sort Method: top-N heapsort  Memory: 38kB
               ->  Parallel Seq Scan on alerts a  (cost=0.00..5449.64 rows=113364 width=126) (actual time=0.007..6.510 rows=100581 loops=3)
                     Buffers: shared hit=4316
 Execution Time: 16.738 ms
```

Postgres has to parallel-scan the entire table (all ~301k rows, 4390
buffer hits) and heapsort every row just to hand back the newest 50 —
there's no way to ask "give me the 50 newest" without either an index on
`triggered_at` or reading everything.

## Root cause

The `alerts` table review that added indexes on `device_id` and `status`
(Phase 1 schema hardening) covered the columns every *filtered* query
needs, but missed that the *unfiltered* default view's `ORDER BY` needs
its own index — sorting isn't free just because the table has indexes on
other columns.

## Fix

```sql
CREATE INDEX idx_alerts_triggered_at ON alerts (triggered_at DESC);
```

Added to `infra/postgres/init.sql` (permanent, applies to every fresh
environment from here on). Re-ran the identical query after creating the
index on the same 301k-row table:

```
 Limit  (cost=0.42..2.83 rows=50 width=126) (actual time=0.052..0.261 rows=50 loops=1)
   Buffers: shared hit=50 read=3
   ->  Index Scan using idx_alerts_triggered_at on alerts a  (cost=0.42..14524.41 rows=301742 width=126) (actual time=0.051..0.257 rows=50 loops=1)
         Buffers: shared hit=50 read=3
 Execution Time: 0.313 ms
```

**16.7ms → 0.31ms (≈53x), 4,390 buffer hits → 53 (≈83x).** Index Scan
instead of a Parallel Seq Scan + sort — it reads exactly the 50 rows it
needs, in order, off the index, and stops.

Descending order was chosen to match the query's actual `ORDER BY ...
DESC` — Postgres can walk a b-tree index in either direction from a plain
ascending index too, but declaring it `DESC` up front means the planner
never has to consider whether a backward scan is worth it.

## Prevention

- CLAUDE.md rule 5 already states this exactly ("every query path needs
  an index behind it before real data volume arrives") — this incident is
  a concrete case study for why that rule exists, not a new rule.
- Going forward: any new `ORDER BY` added to a query path — not just new
  `WHERE` filters — gets checked against existing indexes as part of the
  same review, not assumed to be free because the table already has
  *some* indexes.
- A composite `(status, triggered_at DESC)` would additionally speed up
  the very common "open alerts, newest first" filtered case beyond what
  the separate `idx_alerts_status` + `idx_alerts_triggered_at` combination
  gives Postgres today (it can bitmap-AND the two, but a single composite
  index avoids that step entirely). Not added now — no evidence yet that
  the filtered case is actually slow enough to need it — but worth
  profiling for if/when this ever runs at real volume.
