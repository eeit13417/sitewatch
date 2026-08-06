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

Phase 0 (infrastructure skeleton) — Postgres, MongoDB, and Mosquitto run via Docker Compose; `api` and `ingestion` are placeholder Go services with CI wired up. See `docs/PROJECT_PLAN.md` for what's next.

## Prerequisites

- Docker Desktop with WSL integration enabled for this distro
- Go 1.22+
- Node 20+ (for the frontend, added in Phase 3)

## Quick start

```bash
cd infra
cp ../.env.example ../.env   # edit if needed
docker compose --env-file ../.env up -d
docker compose ps
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
frontend/     React + TypeScript dashboard (Phase 3)
infra/        docker-compose.yml, Mosquitto config, Postgres init SQL
docs/         architecture notes, project plan, runbooks (added over time)
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for branch naming and commit conventions.
