# RCA 5: Alert storm from a value oscillating at the threshold

## Symptom

The alert engine's dedup only ever guaranteed "at most one active alert
per rule" — it said nothing about how *fast* an alert could be created
and resolved. A reading that oscillates right at a rule's threshold
(routine sensor noise, not just a badly-chosen threshold) flaps the alert
create/resolve/create/resolve on every single message.

## Investigation (live drill, `ALERT_DEBOUNCE_BREACHES=1` temporarily to reproduce the pre-fix "naive" behavior)

`temp-sensor-01` has a warning rule at `temperature_c > 28`. Published 8
readings alternating `29, 27, 29, 27, ...` (each briefly above, then
below, the threshold) via `mosquitto_pub`, 0.5s apart. Ingestion's logs:

```
alert decision ... action=auto_resolve value=29
alert decision ... action=auto_resolve value=27
alert decision ... action=create       value=29
alert decision ... action=auto_resolve value=27
alert decision ... action=create       value=29
alert decision ... action=auto_resolve value=27
alert decision ... action=create       value=29
alert decision ... action=auto_resolve value=27
```

`sitewatch_ingestion_alert_decisions_total`: 3 `create` + 5 `auto_resolve`
— essentially one decision per message, the alert flapping in lockstep
with the noise. At real sensor-noise rates (readings every few seconds,
not 0.5s), this is still one open→resolved cycle per reading — a
dashboard and (if this project ever added paging) an on-call phone
lighting up for a condition that was never actually a sustained problem.

## Root cause

`EvaluateRules`'s decision table only looked at the *current* reading
against the *current* active-alert state — there was no memory of recent
readings, so there was no way to distinguish "genuinely just crossed the
line" from "bouncing across the line."

## Fix

`ingestion/alerts.go` — `EvaluateRules` now takes a `streaks
map[string]BreachState` (per-`alert_rule_id`, tracking consecutive
same-direction readings) and a `debounceN`: `CreateAlert` only fires
after `debounceN` consecutive breaching readings, `AutoResolve` only
after `debounceN` consecutive non-breaching ones. Any reading that breaks
the current direction resets the streak to 1. State lives in
`BreachTracker`, a small mutex-protected holder (ingestion's worker pool
processes multiple devices concurrently) around the otherwise-pure
`EvaluateRules`. `ALERT_DEBOUNCE_BREACHES` (default 3) is env-tunable.

**Re-ran the identical 8-reading oscillating pattern with the default
`ALERT_DEBOUNCE_BREACHES=3`: zero decisions, zero new alert rows.** The
streak never reaches 3 in either direction because the value flips back
every single reading — exactly the case this was built to stop.

Confirmed the fix doesn't just suppress everything: publishing 3
*consecutive* genuine breaches (`33°C`, three readings in a row, no
flip-flopping) correctly created both the warning and critical alerts on
the 3rd reading, and 3 consecutive normal readings correctly auto-resolved
them.

Also covered by unit tests (`ingestion/alerts_test.go`,
`TestEvaluateRules_Debounce_*`) — including
`TestEvaluateRules_Debounce_StopsFlappingAtTheThreshold`, the same
oscillating scenario as the live drill, exercised without needing a
database.

## Prevention

- `docs/alert-engine.md` documents the debounce design and the reasoning
  (consecutive-count over a fixed time window: simpler to reason about,
  no wall-clock dependency, and it was the design already sketched there
  before this incident, from Phase 2).
- Debounce state is intentionally in-memory, not persisted to Postgres —
  it resets on an `ingestion` restart, which only ever *delays* a
  decision by up to `debounceN - 1` readings (the safe direction: never
  misses a necessary alert, never spuriously creates one either).
- Anyone tuning a rule's `threshold` going forward should expect it to
  interact with `ALERT_DEBOUNCE_BREACHES` — a threshold set to sit
  exactly inside a device's normal noise band will still generate
  *eventual* alerts once real drift pushes it past the debounce window,
  just not the immediate per-reading flap this incident was about.
