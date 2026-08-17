# SiteWatch

Practice project for a Delta Electronics (Thailand) Software & Digital
Enablement application. Full context: `docs/PROJECT_PLAN.md` (phased plan +
JD mapping), `docs/mqtt-contract.md`, `docs/alert-engine.md`,
`docs/openapi.yaml`.

## Engineering standards (apply to every phase, not just when asked)

These were set explicitly by the project owner and are a standing checklist
— re-check all seven at the end of every phase/segment, not only when code
first lands.

1. **OOP where it fits, not by default.** Use a struct/class + methods when
   there's real state to encapsulate (e.g. `Store`, `DeviceSimulator`).
   Keep pure logic (e.g. `EvaluateRules`, `compareThreshold`) as plain
   functions — wrapping stateless logic in an object just to "be OOP" is
   the wrong call here.
2. **Design before code, and don't hardcode what should be configurable.**
   Non-trivial pieces (DB schema, MQTT contract, API surface, alert
   engine) get a short design note in `docs/` before implementation, same
   as Phase 1/2. Values that plausibly need tuning per environment (worker
   counts, pool sizes, debounce windows) belong in env vars or named
   constants with a comment, not buried magic numbers.
3. **Shared logic goes in one place.** `api` and `ingestion` are separate
   Go modules — don't let that become an excuse to copy-paste the same
   function into both. Cross-cutting code (env loading, DB connection
   setup) belongs in a shared internal module referenced by both via a Go
   workspace, not duplicated. Cross-language duplication (e.g. the MQTT
   topic convention existing in both Go and TypeScript) is unavoidable —
   keep it anchored to one documented contract (`docs/mqtt-contract.md`)
   instead.
4. **Code quality**: `gofmt` + `go vet` + `golangci-lint` (with `gosec`)
   must pass in CI for every Go change. Errors are wrapped with context
   (`fmt.Errorf("...: %w", err)`), not swallowed or returned bare.
5. **Efficiency at scale.** Every query path needs to have an index behind
   it before real data volume arrives — this includes MongoDB, which
   doesn't get one for free the way a Postgres primary key does. Think
   through what happens when a table/collection has 100k+ rows and a
   dashboard is querying it live, not just what makes the demo dataset
   fast.
6. **Security.** Parameterized queries only — never format user input into
   SQL. Don't leak raw DB/internal error text to HTTP responses (log it
   server-side, return a generic message). `golangci-lint`'s `gosec`
   linter is the automated backstop for this; known, deliberately deferred
   gaps (no auth yet, no rate limiting yet) must be written down in
   `docs/PROJECT_PLAN.md`, not silently skipped.
7. **Concurrency / multi-user correctness.** State transitions that
   multiple requests could race on (e.g. acknowledging an alert) must be
   a single atomic statement (`UPDATE ... WHERE status = X`), not a
   read-then-write. Background workers that fan out concurrent DB/network
   I/O need an explicit bound (worker pool, connection pool limit) so load
   degrades gracefully instead of unboundedly.

## Commands

```bash
# infra
cd infra && docker compose --env-file ../.env up -d

# per-service tests
cd api && go test ./... && go test -tags=integration ./...
cd ingestion && go test ./... && go test -tags=integration ./...
cd simulator && npm test

# db verification
PGPASSWORD=sitewatch ./scripts/verify-db.sh
```

## Conventions

Branch naming and commit style: see `CONTRIBUTING.md`. Branches are named
after what they contain, not the phase number/week.
