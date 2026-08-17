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

Phase 3 (frontend dashboard) — `frontend` is a React + TS + Vite app ([design](docs/frontend-design.md)) covering site overview, device list, a live-polled telemetry chart, and the full alert workflow (list/filter/acknowledge/resolve), backed by `api`'s REST surface ([`docs/openapi.yaml`](docs/openapi.yaml)). `ingestion` subscribes to the simulator's MQTT telemetry, writes it to MongoDB, and runs the alert engine ([`docs/alert-engine.md`](docs/alert-engine.md)). See `docs/PROJECT_PLAN.md` for what's next (Phase 4: observability).

## Prerequisites

- Docker Desktop with WSL integration enabled for this distro
- Go 1.22+
- Node 20+ (for the simulator now, and the frontend in Phase 3)

## Quick start

```bash
cp .env.example .env   # edit if needed
cd infra
docker compose --env-file ../.env up -d
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

Run `ingestion` (consumes that same MQTT traffic, writes to Mongo/Postgres, evaluates alerts):

```bash
cd ingestion
go run .
```

Run `api` in another terminal, then exercise it:

```bash
cd api
go run .
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
