# SiteWatch device simulator

Publishes fake telemetry over MQTT for the 7 devices seeded in
`infra/postgres/init.sql`, following the contract in
[`docs/mqtt-contract.md`](../docs/mqtt-contract.md).

Each device's readings drift with a bounded random walk (not independent
random numbers each tick), and occasionally spike out of range to exercise
the seeded `alert_rules`, or go "offline" for a stretch of ticks to
simulate a dropped connection.

## Run

Requires the broker from `infra/docker-compose.yml` to be running.

```bash
npm install
npm run dev
```

Reads `MQTT_BROKER_URL` and `SIM_INTERVAL_MS` from the repo-root `.env`
(defaults: `tcp://localhost:1883`, `5000`).
