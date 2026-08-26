# Deployment & hardening design (Phase 5)

## Scope

Three independent pieces, tied together only by "things that make this
project safer/more real to run," per `docs/PROJECT_PLAN.md`:

1. Containerize `api`/`ingestion` and wire them into `infra/docker-compose.yml`
2. CI publishes those images to a registry on every `main` commit
3. Close the rate-limiting gap flagged as deferred back in Phase 2

Deliberately **not** in this phase:

- **Actual deployment (CD) to a live environment.** Publishing an image is
  "continuous delivery" — the image exists and is pullable. Continuous
  *deployment* would mean something automatically runs it somewhere
  reachable by other people, which needs an external hosting target.
  Assembling that (a PaaS for `api`/`ingestion`, managed Postgres, managed
  MongoDB, and a managed MQTT broker, since no current free-tier PaaS runs
  a full multi-service stack for free) is real scope, not a footnote —
  explicitly deferred to revisit once the rest of the phased plan is
  further along, not abandoned. Local `docker compose up` remains the way
  to run the full stack; screen recording (Phase 6) is the plan for
  showing it working without needing a public URL.
- **Containerizing `frontend`.** No live deploy target yet means no
  reverse proxy/CDN in front of it either — a Dockerfile for a static
  Vite build only earns its keep once there's somewhere to actually run
  it. Local dev keeps using the Vite dev server.
- **Auth.** Still out of scope for the same reason it was in Phase 2 — see
  `docs/PROJECT_PLAN.md`'s known-gaps list.

## 1. Containerizing `api`/`ingestion`

Multi-stage build: a `golang:1.25` builder stage compiles a static binary
(`CGO_ENABLED=0` — nothing here needs cgo, and a static binary is what
lets the runtime stage have no libc at all), then a
`gcr.io/distroless/static-debian12:nonroot` runtime stage copies just that
binary in. No shell, no package manager, runs as a non-root UID by
default — smallest attack surface available for a Go binary, and CLAUDE.md
rule 6 already treats attack surface as something to actively minimize,
not just not actively violate. Trade-off, written down rather than
discovered later: there's no shell to `docker exec` into for on-the-spot
debugging — `docker logs` and the app's own `/healthz` and `/metrics`
endpoints (Phase 4) are the tools available against a running container.
That's an acceptable trade for a stateless HTTP/MQTT service whose entire
job is already observable through those two things.

Build context is the repo root (not `api/` or `ingestion/` individually):
both modules pull in `shared/` via a `replace` directive in `go.mod`
(CLAUDE.md rule 3), so the Dockerfile needs to see both directories. A
root `.dockerignore` keeps `frontend/`, `simulator/`, `.git/`, and other
irrelevant trees out of the build context.

`infra/docker-compose.yml` gains `api` and `ingestion` services, built
from those Dockerfiles, on the same compose network as
`postgres`/`mongodb`/`mosquitto`. Inside that network they reach each
other by service name (`postgres:5432`, `mongodb:27017`, `mosquitto:1883`)
— set via each service's `environment:` block, distinct from the
`localhost`-based `.env` used for `go run .` local dev, which keeps
working unchanged for day-to-day feature iteration outside Docker.

This is also what retires the workaround written into
`infra/prometheus/prometheus.yml` during Phase 4: on Docker Desktop +
WSL2, `host.docker.internal` can't reach a process bound inside the WSL2
distro, so Prometheus was pointed at the distro's own (unstable) IP
instead. Once `api`/`ingestion` are compose services, Prometheus scrapes
them by service name like everything else — the WSL2-specific note goes
away entirely.

## 2. CI: build + push images

New job in `.github/workflows/ci.yml`, triggered on push to `main` only
(not on PRs — a PR's code isn't `main` yet, nothing to publish). Builds
`api` and `ingestion` images and pushes to **GitHub Container Registry**
(`ghcr.io`): free, and authenticates with the workflow's own
`GITHUB_TOKEN` — no external registry account or secret to manage, which
matters for a solo practice project where every extra credential is
something to lose track of. Tagged with both the commit SHA (so a
specific build is always addressable, matching this project's "written
down, not silently skipped" ethos) and `latest`.

## 3. Rate limiting

Per-IP token bucket, in-memory, via `golang.org/x/time/rate` rather than
hand-rolling the bucket algorithm — this is exactly the kind of
concurrency-sensitive primitive (CLAUDE.md rule 7) where reusing a
well-tested stdlib-adjacent implementation beats a bespoke one. One
`*rate.Limiter` per client IP, held in a struct with a mutex (real state
to encapsulate — CLAUDE.md rule 1 says that's when a struct earns its
keep, unlike the pure-function alert-evaluation logic elsewhere in this
project).

- **Why per-IP and in-memory, not a shared/distributed limiter**: the API
  has no auth and runs as a single instance — there's no multi-instance
  fan-out problem to solve yet, and introducing Redis or similar just to
  coordinate a limiter across instances that don't exist would be the
  over-engineering CLAUDE.md rule 1 warns against. Revisit if/when the API
  is ever actually deployed behind more than one instance.
- **Client identification**: `r.RemoteAddr`. There's no reverse proxy in
  front of the API yet (no CD/live deploy — see above), so there's no
  `X-Forwarded-For` to trust or need to parse. Written down as a gap to
  close if/when a real deploy target puts a proxy in front, since blindly
  trusting a client-supplied `X-Forwarded-For` header without that proxy
  present would let anyone bypass the limiter by spoofing it.
- **Bounded memory**: a map that only grows (one entry per distinct IP
  ever seen, forever) is itself the kind of unbounded-growth problem
  CLAUDE.md rule 7 flags. A background goroutine sweeps entries whose
  limiter hasn't been touched in a while (tracked via a last-seen
  timestamp alongside each limiter) on a fixed interval.
- **Thresholds**: `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` env vars (CLAUDE.md
  rule 2 — tunable, not hardcoded), defaulting to values well above real
  frontend usage (the busiest poller, `/alerts`, refetches every 5s per
  open tab — see `frontend/src/hooks/useAlerts.ts`) but low enough to
  actually stop a naive flood.
- **Response**: `429 Too Many Requests` with a `Retry-After` header,
  logged at `Warn` (not `Error` — a client being throttled isn't a service
  fault) with the existing per-request correlation ID (Phase 4) so a
  throttled client's later retry can still be tied back to it.
- Applied inside the CORS wrapper (`withCORS(withCorrelationID(logger,
  withRateLimit(mux)))`), so a preflight `OPTIONS` request — which
  `withCORS` already answers directly without reaching the inner handler
  — never counts against a client's budget.

## Testing

- Unit: rate limiter allows up to burst, then rejects, then recovers
  after the refill window — pure logic against the limiter type directly,
  no HTTP server involved.
- Integration: real HTTP requests against the full middleware chain
  (`api/integration_test.go`) — confirms a burst past the configured
  limit gets a `429` with `Retry-After`, and that requests from a
  different simulated client IP aren't affected by another's usage.
- Manual: `docker compose up` with the new `api`/`ingestion` services,
  confirm `/healthz` succeeds inside the container, confirm Prometheus
  targets are `UP` via service name (no more WSL2 IP), confirm the CI
  workflow's new job produces a pullable image in GHCR.
