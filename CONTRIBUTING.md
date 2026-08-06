# Contributing conventions

Even as a solo practice project, follow these so the Git history itself is something you can show in an interview.

## Branches

- `main` — always deployable/working
- `feat/<short-name>` — new functionality (e.g. `feat/mqtt-ingestion`)
- `fix/<short-name>` — bug fixes
- `chore/<short-name>` — tooling, CI, docs

## Commits

[Conventional Commits](https://www.conventionalcommits.org/):

```
feat(api): add device CRUD endpoints
fix(ingestion): handle MQTT reconnect on broker restart
docs: add troubleshooting guide for alert storms
test(api): add integration tests for alert rules
```

## Pull requests (even against yourself)

Open a PR from `feat/*` into `main` instead of committing directly, so CI runs and you build the habit:

1. What changed and why
2. How it was tested
3. Any follow-ups deliberately left out of scope

Squash-merge to keep `main` history linear.
