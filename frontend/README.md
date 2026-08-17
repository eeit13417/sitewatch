# SiteWatch frontend

React + TypeScript + Vite dashboard. Design and rationale: [`../docs/frontend-design.md`](../docs/frontend-design.md).

## Run

Needs `infra`'s docker-compose stack, `ingestion`, and `api` already running (see the repo root [README](../README.md)).

```bash
cp .env.example .env
npm install
npm run dev
```

## Test

```bash
npm run lint     # oxlint
npm run build    # tsc + vite build
npx playwright install --with-deps chromium   # once
npm run test:e2e
```

E2E tests (`e2e/`) run against the real stack — they publish real MQTT
messages to trigger real alerts through `ingestion`, not a mocked API.
