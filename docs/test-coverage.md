# Test coverage report

Generated via `go test ./... -coverprofile=...` /
`go test -tags=integration ./... -coverprofile=...` per module. The
`-tags=integration` run is the meaningful "full suite" number — it
compiles and runs the untagged (pure-logic, no I/O) unit tests *and* the
`//go:build integration` (real Postgres/MongoDB via `testcontainers-go`)
tests together in one binary, same as CI's `integration-test` job.

| Module | Unit-only (no Docker) | Full suite (`-tags=integration`) |
|---|---|---|
| `shared` | 5.8% | 5.8%* |
| `api` | 5.4% | 50.4% |
| `ingestion` | 9.9% | 32.3% |

\* `shared`'s only untested lines are `PostgresPoolCollector`'s
`Describe`/`Collect` (the `prometheus.Collector` interface methods) —
exercised implicitly by every `api`/`ingestion` integration test that
registers a real Postgres pool's collector into a registry and reads
`/metrics`, just not captured as `shared`'s own coverage number since
that exercise happens in a different module's test binary.

## Reading the low unit-only numbers correctly

This isn't under-tested — it's what CLAUDE.md rule 1 (pure logic as plain
functions, I/O behind a thin layer) produces when measured by line
coverage: `api`'s and `ingestion`'s pure decision logic
(`ratelimit.go`'s token bucket, `alerts.go`'s `EvaluateRules` and its
debounce state machine, `compareThreshold`) is fully unit tested with no
database, but that's a small fraction of each module's total lines — most
of the rest is direct DB/HTTP I/O (`sites.go`, `devices.go`, `store.go`,
etc.), which this project deliberately tests against real containers
(`docs/rca/*.md`'s own testing philosophy: no mocks) rather than unit
tests with a fake driver. The full-suite column is the number that
reflects actual exercised behavior.

## Notable 100%/near-100% coverage, by design

- `ingestion/alerts.go`'s `EvaluateRules` and `BreachState` debounce logic
  — every branch of the decision table (create, no-op-while-active,
  auto-resolve, debounce-withheld, streak-reset) has a dedicated unit
  test (`ingestion/alerts_test.go`).
- `api/ratelimit.go`'s token-bucket `allow`/`cleanupLoop` — burst
  exhaustion, per-client independence, refill-over-time, and idle
  eviction each have a dedicated unit test
  (`api/ratelimit_test.go`), plus an integration test proving the real
  HTTP middleware chain returns `429`/`Retry-After` correctly.

## Known gaps in coverage

- `main()` functions (`api/main.go`, `ingestion/main.go`) are
  intentionally close to 0% — they're wiring (env loading, server
  startup, signal handling), exercised in practice by every integration
  test indirectly building the same handler chain (`routes()`,
  `setupTestApp`), but `main` itself isn't a unit under test the way the
  functions it calls are.
- `applyDecisionsWithRetry`'s *successful* retry path (a transient
  failure that recovers within the backoff window before `maxAttempts`)
  isn't separately covered — only the "gives up after every attempt
  fails" case is (`TestApplyDecisionsWithRetry_GivesUpAfterMaxAttempts`).
  The mechanism generating a successful mid-retry recovery deterministically
  in a test — vs. giving up entirely — would need a fake/injectable
  failure sequence rather than the real `pgxpool.Pool` this project has
  consistently preferred testing against; judged not worth the added
  complexity for the marginal coverage gained. The retry-then-succeed
  behavior is a straightforward corollary of the loop's structure (`if
  err = ...; err == nil { return nil }`), not an independent piece of
  logic likely to have its own bug.
