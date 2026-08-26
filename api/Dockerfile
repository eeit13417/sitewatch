# Build context is the repo root (see docker-compose.yml and
# docs/deployment-hardening-design.md) — api's go.mod pulls in shared/ via
# a `replace` directive, so both directories need to be visible here.

FROM golang:1.25 AS builder
WORKDIR /build

COPY shared/ ./shared
COPY api/ ./api
WORKDIR /build/api
RUN go mod download

# CGO_ENABLED=0: a static binary, which is what lets the runtime stage
# below have no libc at all.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api .

# No shell, no package manager, runs as a non-root UID by default —
# smallest attack surface available for a Go binary (CLAUDE.md rule 6).
# Trade-off: no `docker exec` into it — `docker logs` plus the app's own
# /healthz and /metrics (Phase 4) are what's available against a running
# container instead.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
