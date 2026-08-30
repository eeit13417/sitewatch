# RCA 2: Leaked per-connection goroutine + unbounded MongoDB pool

This incident bundles two related "memory grows for no obvious reason"
causes — a deliberately injected goroutine leak (demonstrated with
`pprof`), and a real, pre-existing unbounded connection pool (fixed
permanently, not just demonstrated).

## Part A: goroutine leak on MQTT reconnect (drilled)

### Symptom

A service whose memory/goroutine count creeps up over time with no
matching increase in message volume — the classic "looks fine at deploy,
degrades over days" pattern that's hard to catch without actually
profiling it.

### Injection

`ingestion`'s `opts.OnConnect` callback fires every time the MQTT client
connects — including every *reconnect*, since `SetAutoReconnect(true)` is
set. A plausible real mistake: spawning a per-connection background
goroutine (e.g. a heartbeat/stats reporter tied to that connection)
without ever cancelling it when the connection drops:

```go
opts.OnConnect = func(c mqtt.Client) {
    logger.Info("connected to broker, subscribing", "broker", brokerURL)
    go func() {
        for {
            time.Sleep(time.Hour)
        }
    }()
    token := c.Subscribe(...)
    ...
}
```

### Investigation

Added `net/http/pprof` to `ingestion`'s existing metrics HTTP server
(`ingestion/metrics.go`, `/debug/pprof/*`) — left in permanently as
standard operational tooling, not just for this drill.

Baseline goroutine count after one connect: **29**
(`curl http://localhost:8081/debug/pprof/goroutine?debug=1 | head -1`).

Forced 5 broker reconnects (`docker compose restart mosquitto`, several
times, `SetAutoReconnect` handles rejoining) — ingestion's own logs show
each disconnect/reconnect cycle:

```
{"level":"WARN","msg":"mqtt connection lost","error":"EOF"}
{"level":"INFO","msg":"connected to broker, subscribing", ...}
```

Goroutine count after 6 total connects (1 initial + 5 reconnects): **34**
— exactly `+5` for `+5` reconnects, a clean 1:1 leak rate. Confirmed the
exact leaking stack via the profile:

```
6 @ 0x47cc6e 0x480e65 0xb0047d 0x484b81
#   time.Sleep+0x164          runtime/time.go:363
#   main.main.func3.1+0x1c    ingestion/main.go:138
```

Six goroutines, all parked in the injected `time.Sleep`, one per connect
— the profile doesn't just show *that* something's growing, it points
directly at the exact line responsible.

### Root cause

`OnConnect` is not "ran once at startup" — it's "runs once per
connection establishment," which for a client with `SetAutoReconnect`
means potentially many times over a service's lifetime (broker restarts,
network blips, anything that drops the TCP connection). Any resource
allocated inside it needs a matching per-connection cleanup, or it
accumulates once per reconnect for the life of the process.

### Fix

Removed the injected goroutine entirely — there was no real need for it
in the first place (it existed purely to demonstrate the failure mode).
Re-verified after rebuilding: baseline 28 after one connect, still 28
after 3 more forced reconnects (4 total) — flat, no growth.

### Prevention

- Anything spawned inside `OnConnect` needs an explicit answer to "what
  cancels this when the connection drops," not just "what starts it."
  `OnConnectionLost` is the natural place to wire that cancellation if a
  genuine per-connection background task is ever needed.
- `/debug/pprof` stays wired into `ingestion`'s metrics server going
  forward — the cost of leaving it in is near zero, and this incident is
  exactly the class of bug it exists to catch.

## Part B: unbounded MongoDB connection pool (real, fixed permanently)

### Symptom (would-be)

Nothing broken today — but a latent gap: `shared/db.go`'s
`NewMongoClient` called `mongo.Connect` with only `ApplyURI(...)`, no pool
size configured at all. The MongoDB Go driver defaults to a max pool size
of 100 connections per client when left unset — not "unbounded" in the
literal sense, but unbounded *in practice*: nobody chose 100, it's just
whatever the driver ships with, and it was never revisited as usage grew.
Contrast with `shared/db.go`'s own Postgres pool, which has always had an
explicit, tunable `POSTGRES_MAX_CONNS` (default 10) — the Mongo side of
the same file quietly didn't get the same treatment.

### Fix

```go
const defaultMongoMaxPoolSize = 20

func NewMongoClient(ctx context.Context) (*mongo.Client, error) {
    poolSize := uint64(defaultMongoMaxPoolSize)
    if raw := os.Getenv("MONGO_MAX_POOL_SIZE"); raw != "" {
        if n, err := strconv.ParseUint(raw, 10, 32); err == nil && n > 0 {
            poolSize = n
        }
    }
    opts := options.Client().ApplyURI(os.Getenv("MONGO_URL")).SetMaxPoolSize(poolSize)
    ...
}
```

Mirrors the existing `POSTGRES_MAX_CONNS` pattern exactly — same default
philosophy (a deliberate, documented number instead of an accident of the
driver's own default), same env-var override mechanism.

### Prevention

CLAUDE.md rule 7 already calls for an explicit bound on anything that
fans out concurrent I/O; this was simply a spot the rule hadn't been
applied to yet. No new rule needed — just closing the gap.
