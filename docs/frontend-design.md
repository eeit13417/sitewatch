# Frontend design (Phase 3)

## Stack

- Vite + React + TypeScript
- [TanStack Query](https://tanstack.com/query) for all server data — caching, refetch, loading/error state, instead of hand-rolled `useEffect` + `fetch`
- `react-router` for real, bookmarkable routes (site overview / site detail / device detail / alerts are distinct pages, not tab state)
- `recharts` for the telemetry trend chart — charting isn't the skill this project demonstrates, so a proven library over hand-rolled SVG
- Function components + hooks throughout, no class components — the idiomatic React style, not a compromise (see the OOP discussion in project history: use a class/object only where there's real state to encapsulate, and React's own convention for that is hooks, not `this`)

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
