# MQTT contract

Defines the topic naming rule and payload shape that every publisher (the
device simulator now, real devices later) and every subscriber (the Phase 2
`ingestion` service) must agree on.

## Topic

```
sitewatch/<site_id>/<device_id>/telemetry
```

- `site_id`, `device_id` are the UUIDs from the `sites`/`devices` tables in
  PostgreSQL (see `infra/postgres/init.sql`).
- The topic is **derived**, never stored. Both the simulator and the
  ingestion service build it from the same two ids at runtime. This was a
  deliberate fix from the Phase 1 schema review — storing it as a `devices`
  column would let it drift out of sync with the ids it's built from.
- Ingestion subscribes with a wildcard: `sitewatch/+/+/telemetry`.

## Payload

One JSON object per MQTT message, one message per device per publish tick.

```json
{
  "device_id": "a1111111-0000-0000-0000-000000000002",
  "site_id": "11111111-1111-1111-1111-111111111111",
  "device_type": "temp_sensor",
  "readings": {
    "temperature_c": 24.8
  },
  "ts": "2026-08-08T12:00:00.000Z"
}
```

| Field | Type | Notes |
|---|---|---|
| `device_id` | string (uuid) | Matches `devices.id` |
| `site_id` | string (uuid) | Matches `devices.site_id`; duplicated here so ingestion doesn't have to look it up before writing to MongoDB |
| `device_type` | string | Matches `devices.type`; determines which keys appear in `readings` |
| `readings` | object | Metric name → numeric value. Metric names match `alert_rules.metric` so the Phase 2 alert engine can evaluate a rule by reading `readings[rule.metric]` directly, no translation table needed |
| `ts` | string (ISO 8601, UTC) | When the reading was taken, set by the publisher. Distinct from the `received_at` timestamp the ingestion service stamps when it writes the MongoDB document — the two can differ under network delay or backlog, which is itself useful signal |

`readings` keys are **not fixed globally** — they depend on `device_type`.
This is the concrete reason telemetry goes to MongoDB rather than a
PostgreSQL table: the shape genuinely varies per device type and doesn't
need a schema migration every time a new device type is added.

## `readings` shape per device type

| `device_type` | Keys | Has a seeded `alert_rule` on |
|---|---|---|
| `smart_meter` | `power_kw`, `voltage`, `current` | `power_kw` |
| `temp_sensor` | `temperature_c` | `temperature_c` |
| `humidity_sensor` | `humidity_pct` | — |
| `ups` | `battery_pct`, `load_pct` | — |
| `hvac` | `temperature_c`, `setpoint_c` | — |

## MongoDB write contract (Phase 2, documented here for continuity)

The ingestion service writes each received message into
`telemetry_raw` as close to verbatim as possible, adding one field:

```json
{
  "device_id": "...",
  "site_id": "...",
  "device_type": "...",
  "readings": { "...": 0 },
  "ts": "...",
  "received_at": "2026-08-08T12:00:00.412Z"
}
```

No other transformation happens at ingestion time — aggregation into
PostgreSQL (e.g. updating `devices.last_seen_at`) is a separate concern
handled at a debounced interval, not on every message (see the Phase 1
schema review notes on write amplification).
