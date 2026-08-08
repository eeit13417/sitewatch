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

Phase 1 (data layer + device simulator) — PostgreSQL schema and seed data are live (`sites`, `devices`, `alert_rules`, `alerts`, `users`), and the device simulator publishes realistic telemetry for all 7 seeded devices over MQTT, following [`docs/mqtt-contract.md`](docs/mqtt-contract.md). `api` and `ingestion` are still placeholder Go services. See `docs/PROJECT_PLAN.md` for what's next (Phase 2: ingestion + REST API + alert engine).

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

Run the API service locally:

```bash
cd api
go run .
curl localhost:8080/healthz
```

## Repository layout

```
api/          Go REST API service
ingestion/    Go MQTT ingestion service
simulator/    Node + TypeScript device simulator (publishes fake telemetry over MQTT)
frontend/     React + TypeScript dashboard (Phase 3)
infra/        docker-compose.yml, Mosquitto config, Postgres schema + seed data
docs/         architecture notes, project plan, MQTT contract, runbooks (added over time)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming and commit conventions.
