# Alert engine design

Runs inside `ingestion`, called once per telemetry message after it's
written to MongoDB — not a separate service. At this data volume, splitting
it out would add a network hop and a deployment unit for no real benefit;
revisit only if evaluation load ever needs to scale independently of
ingestion.

## Inputs

For a telemetry message from `device_id` with `readings` (metric → value,
per `docs/mqtt-contract.md`):

1. Load the device's `enabled` `alert_rules`.
2. Load which of those rules currently have an **active** alert — an alert
   is active while `status IN ('open', 'acknowledged')`; `resolved` is
   terminal. An invariant the engine maintains: at most one active alert
   per `alert_rule_id` at any time.

## Decision (pure, no I/O — see `EvaluateRules` in `ingestion/alerts.go`)

For each rule whose `metric` appears in `readings`:

| Condition met? | Active alert exists? | Debounce streak reached `ALERT_DEBOUNCE_BREACHES`? | Action |
|---|---|---|---|
| yes | no | yes | create a new alert (`status = 'open'`) |
| yes | no | not yet | nothing — still accumulating consecutive breaches |
| yes | yes | n/a | nothing — already tracked, don't duplicate |
| no | yes | yes | auto-resolve the active alert (`status = 'resolved'`, `resolved_at = now()`, `resolved_by` stays `NULL` to distinguish "cleared itself" from "a person resolved it") |
| no | yes | not yet | nothing — still accumulating consecutive non-breaches |
| no | no | n/a | nothing |

## Debounce (added Phase 6 — see `docs/rca/05-alert-storm.md`)

The engine originally acted on every single reading: the moment a value
crossed a threshold it created an alert, and the moment it dipped back it
auto-resolved. A value oscillating right at a rule's boundary — which
happens routinely with real sensor noise, not just misconfiguration —
flapped the alert create/resolve/create/resolve on every message. That gap
was deliberate at the time: Phase 6 used exactly this as one of the
drilled incidents (an alert storm from a badly-tuned threshold), so the
naive version needed to still be here to break before it got fixed.

The fix: `EvaluateRules` now takes a `streaks map[string]BreachState`
(keyed by `alert_rule_id`) tracking how many consecutive readings in a row
have gone the same direction, and a `debounceN` — a rule only fires
`CreateAlert` after `debounceN` consecutive breaching readings, and only
fires `AutoResolve` after `debounceN` consecutive non-breaching ones. Any
reading that breaks the current streak's direction resets the counter to
1, so a value bouncing back and forth across the boundary never
accumulates enough consecutive readings in either direction to act at all
— which is exactly the flapping case this needed to stop.

`debounceN` is `ALERT_DEBOUNCE_BREACHES` (default 3, tunable per CLAUDE.md
rule 2). The streak state itself (`ingestion/alerts.go`'s
`BreachTracker`) lives in ingestion's process memory, not Postgres — it
resets on restart, which only ever delays a decision by up to
`debounceN - 1` readings (the safe direction: never causes a missed
necessary alert, never causes a spurious one either). Keyed purely by
`alert_rule_id` since a rule belongs to exactly one device, so no need to
also key by `device_id`.

## Why "active" includes `acknowledged`, not just `open`

If someone acknowledges an alert but the underlying condition is still
breaching, a second alert must not be created for the same rule — the
acknowledged one is still "the" alert for that condition. Only when the
reading genuinely returns to normal does the engine act again, this time
auto-resolving it regardless of whether it was sitting at `open` or
`acknowledged`.
