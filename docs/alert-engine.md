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

| Condition met? | Active alert exists? | Action |
|---|---|---|
| yes | no | create a new alert (`status = 'open'`) |
| yes | yes | nothing — already tracked, don't duplicate |
| no | yes | auto-resolve the active alert (`status = 'resolved'`, `resolved_at = now()`, `resolved_by` stays `NULL` to distinguish "cleared itself" from "a person resolved it") |
| no | no | nothing |

This intentionally does **not** implement debounce/hysteresis (e.g. "only
trigger after 3 consecutive breaches") or alert-storm suppression — a
threshold that flaps around its boundary will flap the alert with it. That
gap is deliberate: Phase 6 uses exactly this as one of the drilled
incidents (an alert storm from a badly-tuned rule), so the naive version
needs to still be here to break.

## Why "active" includes `acknowledged`, not just `open`

If someone acknowledges an alert but the underlying condition is still
breaching, a second alert must not be created for the same rule — the
acknowledged one is still "the" alert for that condition. Only when the
reading genuinely returns to normal does the engine act again, this time
auto-resolving it regardless of whether it was sitting at `open` or
`acknowledged`.
