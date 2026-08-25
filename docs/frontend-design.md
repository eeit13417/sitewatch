# Frontend design (Phase 3)

## Stack

- Vite + React + TypeScript
- [TanStack Query](https://tanstack.com/query) for all server data — caching, refetch, loading/error state, instead of hand-rolled `useEffect` + `fetch`
- `react-router` for real, bookmarkable routes (site overview / site detail / device detail / alerts are distinct pages, not tab state)
- `recharts` for the telemetry trend chart — charting isn't the skill this project demonstrates, so a proven library over hand-rolled SVG
- **Tailwind CSS v4** + `lucide-react` for icons — adopted when the UI was redesigned from an AI-generated (v0.dev) mockup; see "Visual redesign" below
- Function components + hooks throughout, no class components — the idiomatic React style, not a compromise (see the OOP discussion in project history: use a class/object only where there's real state to encapsulate, and React's own convention for that is hooks, not `this`)

## Visual redesign

The original hand-written-CSS version was replaced with a design produced
in v0.dev and ported in by hand — not pasted in wholesale. v0's export was
already Vite + TypeScript + react-router, which made the port mostly a
1:1 file mapping, but two things always get replaced when adopting a
generated UI, and did here:

- **Mock data and mock state are never kept.** v0's export shipped a
  `lib/data.ts` (hand-written fake sites/devices/telemetry) and a
  `lib/alerts-store.tsx` (a `useState`-backed context standing in for
  acknowledge/resolve). Both were deleted outright — every page was
  rewired to the real hooks (`useSites`, `useDevices`, `useAlerts`,
  `useAcknowledgeAlert`/`useResolveAlert`) that already talked to `api`.
  Field names also differ (the mock used camelCase like `deviceId`; the
  real API is snake_case, `device_id`) — components consume the real
  `api/types.ts` shapes directly rather than adding a translation layer
  that would just be one more thing to keep in sync.
- **Test selectors don't couple to visual structure.** The pre-redesign
  E2E tests selected on CSS classes (`.alert-table`, `.badge--warning`) —
  which the redesign promptly broke, since a new visual system has no
  reason to keep old class names. Fixed by adding `data-testid` /
  `data-*` attributes to the components tests actually care about
  (`indicators.tsx`, `AlertTable.tsx`) and switching selectors to those —
  a future visual change can happen without touching the test suite
  again.

One addition beyond a straight port: `lib/utils.ts`'s `metricLabels`
gives telemetry chart lines human labels and units (e.g. "Temperature
(°C)" instead of the raw key `temperature_c`), keyed by metric name
rather than per-device-type — one metric name means the same thing
regardless of which device type reports it, so it stays a flat lookup.

## Routes

| Route | Shows |
|---|---|
| `/` | Site cards: device count, open-alert count by severity |
| `/sites/:id` | That site's devices: type, online/offline (derived from `last_seen_at` freshness) |
| `/devices/:id` | Telemetry trend chart (`GET /devices/{id}/telemetry`) + that device's alert history |
| `/alerts` | All alerts, filterable by status/site/device, acknowledge/resolve actions |

## Live updates: polling, not WebSocket

`api` is pure REST today — there's no WebSocket server to connect to. Building
one just for this phase would be backend scope creep disguised as a
frontend task. TanStack Query's `refetchInterval` covers "near-live" well
enough (alerts list every 5s, telemetry chart every 10s). A real WebSocket
push channel is a natural Phase 4 addition once observability work is
already touching connection/streaming infrastructure — not before.

## Shared code (not duplicated per component)

- `src/api/client.ts` — one fetch wrapper (base URL from `VITE_API_URL`,
  JSON parsing, error handling), every hook goes through it
- `src/api/types.ts` — TypeScript types mirroring `docs/openapi.yaml`
  (`Site`, `Device`, `Alert`, `AlertRule`, `TelemetryPoint`). Go and
  TypeScript can't literally share these definitions across languages —
  same situation as the MQTT topic convention — so `openapi.yaml` is the
  documented source of truth both sides are kept aligned to by hand.
- `src/hooks/` — one hook per resource (`useSites`, `useDevices`,
  `useAlerts`, `useDeviceTelemetry`, `useAlertMutations`); components call
  hooks, never `fetch`/`client` directly.

## Efficiency

Alerts list uses the API's existing `limit`/`offset` — never fetches
"everything." The telemetry chart always passes an explicit `limit`,
never an unbounded query.

## Security

- No login screen: `api` has no auth to back one (documented gap in
  `docs/PROJECT_PLAN.md`) — a UI that pretends to authenticate against a
  backend that doesn't check anything would be worse than no UI at all.
- No `dangerouslySetInnerHTML` anywhere — React escapes all rendered text
  by default, which is the entire XSS mitigation here, and it only holds
  if that escape hatch stays unused.
- API base URL comes from `VITE_API_URL` (falls back to
  `http://localhost:8080` for local dev), never hardcoded into request
  code.

## Testing

Unit/integration tests already cover the backend (Phase 1/2). Phase 3
closes the pyramid with **Playwright E2E**, run against the real stack —
Vite dev server + real `api` + real Postgres/Mongo from
`infra/docker-compose.yml` — not a mocked API. Core flows: view the
alerts list, acknowledge an alert and see its state update, filter by
status.
