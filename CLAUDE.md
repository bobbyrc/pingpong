# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
go build ./cmd/pingpong/         # Build the binary
go test ./...                    # Run all tests
go test -short ./...             # Skip long-running tests
go test ./internal/alerter/...   # Run tests for a single package
go test -run TestEngineName ./internal/alerter/...  # Run a single test
go vet ./...                     # Static analysis
go mod tidy                      # Tidy dependencies
```

No Makefile or task runner — use standard Go toolchain directly.

## Architecture

PingPong is a single Go binary that monitors network quality (ping, DNS, speedtest, traceroute), exposes Prometheus metrics, and sends alert notifications via Apprise.

### Package Layout

- **`cmd/pingpong/main.go`** — Entrypoint. Wires together all components, runs 6 concurrent goroutines (4 collectors + alert retry + HTTP server) managed by `sync.WaitGroup` + `context.Context` for graceful shutdown. Contains connection state machine (`connDown`/`downSince` behind a mutex).
- **`internal/collector/`** — Measurement collectors (ping, dns, speedtest, traceroute). Each has a `Collect(ctx)` method returning typed results. Speedtest and traceroute shell out to external binaries; ping uses `pro-bing`; DNS uses `net.Resolver`.
- **`internal/metrics/`** — Prometheus metric registration. Single `Metrics` struct with all gauges/counters, created with `metrics.New(registry)`.
- **`internal/alerter/`** — Three components:
  - `Queue` — SQLite-backed durable alert queue (WAL mode, via `sqlx` + `modernc.org/sqlite`).
  - `AppriseClient` — HTTP client posting to Apprise API `/notify` endpoint.
  - `Engine` — Threshold evaluation with per-target cooldowns (in-memory map keyed by `"alertType:target"`, seeded from DB on startup via `SeedCooldowns()`). Calls `queue.Enqueue()` when thresholds crossed, `ProcessQueue()` drains pending alerts.
- **`internal/config/`** — Env-var-only config. All vars prefixed `PINGPONG_*`. Defaults hardcoded in `config.Load()`.

### Data Flow

```
main.go goroutine → collector.Collect(ctx) → main.go sets metrics.* gauges
  → main.go calls engine.Evaluate*() with results
  → engine checks threshold + cooldown → queue.Enqueue() (SQLite)
  → separate ProcessQueue() goroutine (every 30s) → AppriseClient.Send() → Apprise HTTP API
```

### HTTP Endpoints

- `GET /metrics` — Prometheus scrape
- `GET /health` — Returns "ok"

## Deployment

Docker Compose stack with 4 containers: `pingpong` (Go app, port 8080), `prometheus` (9090), `grafana` (3000), `apprise` (8000). The Go container requires `NET_RAW` capability for ICMP. Multi-stage Dockerfile installs `traceroute`, `iputils-ping`, and Ookla `speedtest` CLI in the runtime image.

## Key Conventions

- No web framework — stdlib `net/http` only
- Structured logging via `log/slog`
- Pure-Go SQLite (`modernc.org/sqlite`) — no CGO required
- Prometheus metrics use a dedicated registry (not the global default)
- Collectors are stateless; all state lives in metrics or the alert engine
- Traceroute hop labels use `{hop_number}_{hop_address}` format to avoid label cardinality issues
- Speedtest and traceroute collectors shell out to CLI binaries only available inside the Docker image; their unit tests exercise output parsing only, not actual execution
- Ping integration tests require `CAP_NET_RAW` (root or Docker); use `-short` to skip them locally
