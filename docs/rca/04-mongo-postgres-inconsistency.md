# RCA 4: MongoDB write succeeds, PostgreSQL alert evaluation fails → silent inconsistency

## Symptom

`ingestion.process()` writes telemetry to MongoDB first, then reads/writes
Postgres to evaluate and record alerts. If MongoDB is up but Postgres is
unreachable (or errors) partway through, telemetry is already durably
committed — but the alert evaluation for that exact reading never
happens, with nothing tracking that it was skipped and nothing to
reconcile it once Postgres comes back.

## Investigation (live drill, temporary `ALERT_DEBOUNCE_BREACHES=1` override to isolate this from incident 5's debounce)

Baseline: `telemetry_raw` had 23,001 docs for `temp-sensor-01`
(`a1111111-...002`), 0 open alerts for it.

1. `docker compose stop postgres`.
2. Published one breaching reading directly (`mosquitto_pub`,
   `temperature_c: 35` — above both the warning (28) and critical (32)
   thresholds for this device):
   ```json
   {"device_id":"a1111111-0000-0000-0000-000000000002","site_id":"11111111-...","device_type":"temp_sensor","readings":{"temperature_c":35},"ts":"2026-08-29T13:18:30.000Z"}
   ```
3. `ingestion`'s own logs show exactly where it broke:
   ```
   WARN update last_seen_at failed ... lookup postgres ...: no such host
   WARN failed to process message ... query alert_rules: ... no such host
   ```
   (`MaybeUpdateLastSeen` failing is already-known-and-tolerated — see the
   comment at that call site. `LoadEnabledRules` failing is not: `process`
   returns the error immediately, `worker()` logs a bare `Warn` and moves
   on to the next message.)
4. Confirmed the telemetry landed anyway: `telemetry_raw` count for the
   device went from 23,001 → **23,002**, with the breaching `35°C`
   reading present in the document.
5. `docker compose start postgres`, waited for it to come back healthy.
6. Checked open alerts for the device: **still 0.** Nothing reconciled
   the missed evaluation — the record that a critical threshold was
   breached at `13:18:30` exists only as an unremarkable row in
   `telemetry_raw`, indistinguishable from any other reading unless
   someone manually re-scans history.
7. Published the *same* reading again now that Postgres was back:
   immediately created 2 open alerts (warning + critical), confirming
   normal operation resumed — this was specifically a transient-outage
   window that permanently dropped one evaluation, not a general
   breakage.

## Root cause

No compensating action exists for a Postgres failure that happens
*after* the Mongo write has already committed. The two writes aren't
transactional across databases (impossible here — they're different
storage systems) and nothing stands in for that: no retry on the read
path, no dead-letter/replay mechanism, no reconciliation job that could
notice "this telemetry doc implies a breach with no matching alert."

## Fix

Added `applyDecisionsWithRetry` (`ingestion/alerts.go`): bounded retry (3
attempts, 100ms/200ms backoff) specifically around the final Postgres
write (`ApplyDecisions`) — the step where, by definition, evaluation
already succeeded and there's a real decision that would otherwise be
lost outright rather than just delayed. Verified against a real
`pgxpool.Pool` with a pre-cancelled context (`TestApplyDecisionsWithRetry_GivesUpAfterMaxAttempts`,
`ingestion/integration_test.go`) — confirms the loop actually retries
(elapsed time includes both backoff sleeps) and eventually gives up with
a wrapped error rather than hanging or panicking.

**Deliberately not fixed further**: the retry only covers the write step.
A failure at `LoadEnabledRules`/`LoadActiveRuleIDs` (the read side, as in
this exact drill) or an outage longer than ~300ms still results in a
silently skipped evaluation, same as before. Extending retry to the read
path, or building a full outbox/reconciliation system that could replay
missed evaluations from `telemetry_raw` after the fact, would meaningfully
close the remaining gap — but is a bigger investment than this project's
actual reliability requirement justifies today (single ingestion
instance, no SLA). Written down here rather than silently left
undiscovered, matching CLAUDE.md rule 6's posture on deliberately
deferred gaps.

## Prevention

- Any future write that happens *after* an earlier write has already
  committed to a different store is exactly this class of risk — worth
  asking "what retries this, and what's the plan if it's still down after
  that" at design time, not after finding it in a drill.
- If this project's reliability bar ever goes up (multi-instance, an
  actual uptime target), the honest next step is a small `pending_alert_evaluations`
  outbox table: `MaybeUpdateLastSeen`/rule evaluation failures get a row
  instead of just a log line, and a periodic sweep retries them — turning
  "silently lost" into "delayed," the same trade this fix already made
  for the narrower write-step case.
