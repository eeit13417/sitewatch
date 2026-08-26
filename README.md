# SiteWatch

A practice project simulating a smart energy / data center / building automation monitoring platform — built to rehearse the skills in Delta Electronics (Thailand) Software & Digital Enablement job description: full-stack development, MQTT/event-driven ingestion, observability, and production troubleshooting.

Full phased plan: [docs/PROJECT_PLAN.md](docs/PROJECT_PLAN.md)

## Architecture (target state)

```
[simulated devices] --MQTT--> [ingestion service (Go)] --> MongoDB (raw telemetry)
                                                        --> PostgreSQL (aggregates, devices, alerts)
                                        |
                                        v
                                 [alert engine]
                                        |
                          [REST API (Go)] <---- [React + TS dashboard]

Observability: Prometheus + Grafana, structured JSON logs with correlation IDs
```

## Status

Phase 5 (Docker images & CI hardening) — `api` and `ingestion` now run as real `docker-compose` services (multi-stage Dockerfiles, distroless runtime images — [design](docs/deployment-hardening-design.md)), CI publishes both images to GitHub Container Registry on every `main` commit, and `api` has a per-IP rate limiter closing a gap left open since Phase 2. They both export Prometheus metrics (`/metrics`: request rate/latency, MQTT throughput, alert-engine decisions, Postgres pool stats via a shared collector) and thread a correlation ID through every log line for a given request/message. Grafana ships pre-provisioned from files with one dashboard covering all of it ([design](docs/observability-design.md)). `frontend` is a React + TS + Vite app ([design](docs/frontend-design.md)) covering site overview, device list, a live-polled telemetry chart, and the full alert workflow (list/filter/acknowledge/resolve), backed by `api`'s REST surface ([`docs/openapi.yaml`](docs/openapi.yaml)). `ingestion` subscribes to the simulator's MQTT telemetry, writes it to MongoDB, and runs the alert engine ([`docs/alert-engine.md`](docs/alert-engine.md)). See `docs/PROJECT_PLAN.md` for what's next (Phase 6: production-incident drills + documentation).

## Prerequisites

- Docker Desktop with WSL integration enabled for this distro
- Go 1.22+
- Node 20+ (for the simulator now, and the frontend in Phase 3)

## Quick start

`api` and `ingestion` run as real `docker-compose` services as of Phase 5
([design](docs/deployment-hardening-design.md)) — this one command brings
up the whole backend: Postgres, MongoDB, Mosquitto, `api`, `ingestion`,
Prometheus, and Grafana.

```bash
cp .env.example .env   # edit if needed
cd infra
docker compose --env-file ../.env up -d --build
docker compose ps
```

Check the seeded data landed:

```bash
docker exec sitewatch-postgres psql -U sitewatch -d sitewatch -c "\dt"
docker exec sitewatch-postgres psql -U sitewatch -d sitewatch -c "SELECT name, type FROM sites;"
```

Run the device simulator (publishes telemetry over MQTT every 5s):

```bash
cd simulator
npm install
npm run dev
```

Watch the raw MQTT traffic in another terminal:

```bash
docker exec sitewatch-mqtt mosquitto_sub -t 'sitewatch/#' -v
```

Exercise the API (already running as part of the compose stack above):

```bash
curl localhost:8080/sites
curl localhost:8080/devices
curl "localhost:8080/alerts?status=open"
```

Run the frontend (needs `ingestion` + `api` + infra all already running):

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

Open `http://localhost:5173`.

Prometheus (`http://localhost:9090`) and Grafana (`http://localhost:3000`,
anonymous viewer access for local dev) come up as part of `docker compose
up` above and scrape `api`/`ingestion` by their compose service name — see
[`docs/observability-design.md`](docs/observability-design.md) for what's
exported.

### Iterating on `api`/`ingestion` outside Docker

For a faster edit/run loop than rebuilding an image each time, stop the
compose-managed instance and run the binary directly against the same
infra — `POSTGRES_URL`/`MONGO_URL`/`MQTT_BROKER_URL` in `.env` already
point at `localhost`, which is what a bare `go run` needs:

```bash
cd infra && docker compose stop api      # or: docker compose stop ingestion
cd ../api && go run .                    # or: cd ../ingestion && go run .
```

## Testing

Simulator unit tests (no Docker required — pure logic):

```bash
cd simulator
npm test
```

Database schema/seed-data/constraint checks (requires the Postgres container to be running with `psql` on the host, or run it inside the container: `docker cp scripts/verify-db.sh sitewatch-postgres:/tmp/ && docker exec sitewatch-postgres bash /tmp/verify-db.sh`):

```bash
PGPASSWORD=sitewatch ./scripts/verify-db.sh
```

`ingestion`/`api` unit tests (pure logic, no Docker):

```bash
cd ingestion && go test ./...   # alert engine rule evaluation
cd api && go test ./...         # no pure-logic tests yet — see integration tests below
```

`ingestion`/`api` integration tests — real Postgres (seeded from the actual `infra/postgres/init.sql`) and MongoDB via `testcontainers-go`, no docker-compose needed, just a running Docker daemon:

```bash
cd ingestion && go test -tags=integration ./... -v
cd api && go test -tags=integration ./... -v
```

Frontend E2E (Playwright, against the real running stack — infra + `ingestion` + `api` + the Vite dev server all need to already be up, see Quick start above):

```bash
cd frontend
npx playwright install --with-deps chromium   # once
npm run test:e2e
```

All of the above run in CI on every push — see `.github/workflows/ci.yml` (`lint-and-build`, `integration-test`, `simulator-test`, `db-schema`, `frontend-build`, `frontend-e2e` jobs).

## Repository layout

```
api/          Go REST API service
ingestion/    Go MQTT ingestion service
shared/       Go module shared by api/ingestion (env loading, DB connection setup)
simulator/    Node + TypeScript device simulator (publishes fake telemetry over MQTT)
frontend/     React + TypeScript dashboard (Vite, TanStack Query, Playwright E2E)
infra/        docker-compose.yml, Mosquitto config, Postgres schema + seed data
scripts/      repeatable verification scripts (e.g. verify-db.sh)
docs/         architecture notes, project plan, MQTT contract, runbooks (added over time)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming and commit conventions.
