# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make              # Show all available targets
make up           # Start app + apprise
make up-all       # Start with monitoring stack
make down         # Stop default profile
make down-all     # Stop everything
make rebuild      # Rebuild image and restart
make logs         # Tail pingpong logs
make test         # Run tests (skip integration)
make check        # Pre-commit quality gate (vet + test + tidy check)
make env-setup    # Copy .env.example → .env if missing
make clean        # Remove binary + Docker volumes
```

Direct Go commands also work:

```bash
go build ./cmd/pingpong/         # Build the binary
go test ./...                    # Run all tests
go test -short ./...             # Skip long-running tests
go test ./internal/alerter/...   # Run tests for a single package
go test -run TestEngineName ./internal/alerter/...  # Run a single test
go vet ./...                     # Static analysis
go mod tidy                      # Tidy dependencies
```

## Architecture

PingPong is a single Go binary that monitors network quality (ping, DNS, speedtest, traceroute), exposes Prometheus metrics, and sends alert notifications via Apprise.

### Package Layout

- **`cmd/pingpong/main.go`** — Entrypoint. Wires together all components, runs 7 concurrent goroutines (4 collectors + alert retry + SSE broadcaster + HTTP server) managed by `sync.WaitGroup` + `context.Context` for graceful shutdown. Contains connection state machine (`connDown`/`downSince` behind a mutex). Opens a shared `*sqlx.DB` via `alerter.OpenDB()` and passes it to both `alerter.NewQueue()` and `web.NewHistoryStore()`.
- **`internal/collector/`** — Measurement collectors (ping, dns, speedtest, traceroute). Each has a `Collect(ctx)` method returning typed results. DNS collector supports multiple targets and servers (always includes system resolver). Speedtest and traceroute shell out to external binaries; ping uses `pro-bing`; DNS uses `net.Resolver`.
- **`internal/metrics/`** — Prometheus metric registration. Single `Metrics` struct with 19 metrics (gauges, counters, gauge vecs), created with `metrics.New(registry)`.
- **`internal/alerter/`** — Three components:
  - `OpenDB()` — Package-level function that opens a SQLite database with WAL mode and `busy_timeout=5000`, returning a shared `*sqlx.DB` handle.
  - `Queue` — SQLite-backed durable alert queue. `NewQueue(db)` accepts a `*sqlx.DB` (does not own the connection lifecycle). Provides `Enqueue()`, `ProcessQueue()`, `RecentAlerts()` (paginated), `AllCooldowns()`, and `SeedCooldowns()`.
  - `AppriseClient` — HTTP client posting to Apprise API `/notify` endpoint.
  - `Engine` — Threshold evaluation with per-target cooldowns (in-memory map keyed by `"alertType:target"`, seeded from DB on startup via `SeedCooldowns()`/`AllCooldowns()`). The `cooldown_key` is persisted in the alerts table so per-target cooldowns survive restarts. Calls `queue.Enqueue()` when thresholds crossed, `ProcessQueue()` drains pending alerts.
- **`internal/config/`** — Env-var-only config. All vars prefixed `PINGPONG_*`. Defaults hardcoded in `config.Load()`. DNS config supports plural env vars (`PINGPONG_DNS_TARGETS`, `PINGPONG_DNS_SERVERS`) with backwards-compat fallback to singular forms.
- **`internal/web/`** — Web UI and real-time metric streaming. Key components:
  - `Handler` — Registers all HTTP routes on a stdlib `ServeMux`. Serves HTML pages (dashboard, alerts, config) via embedded Go templates and a dark-theme CSS stylesheet. Embeds templates and static assets at compile time (`//go:embed`).
  - `Broadcaster` — SSE server that gathers Prometheus metrics every 5 seconds, serializes snapshots as JSON, and pushes to all connected clients. Also records history for sparkline metrics (ping latency, download/upload speed) via `HistoryStore`.
  - `HistoryStore` — SQLite-backed persistence for sparkline data in a `metric_history` table. Shares the same `*sqlx.DB` opened by `alerter.OpenDB()`. Provides `Record()`, `Load()`, `LoadAll()` (with window-function-based limiting), and `Prune()` (keeps latest 60 points per series, throttled to ~once per minute).
  - `config_io.go` — Reads/writes `.env` files with a merge-update strategy (preserves comments and ordering).

### Data Flow

```
main.go goroutine → collector.Collect(ctx) → main.go sets metrics.* gauges
  → main.go calls engine.Evaluate*() with results
  → engine checks threshold + cooldown → queue.Enqueue() (SQLite)
  → separate ProcessQueue() goroutine (every 30s) → AppriseClient.Send() → Apprise HTTP API

Broadcaster (every 5s) → gathers metrics from Prometheus registry → JSON snapshot
  → pushes to all SSE clients via /api/events
  → records ping_latency, download_speed, upload_speed to HistoryStore (SQLite)
  → prunes old history every ~60s (keeps 60 points per series)
```

### HTTP Endpoints

- `GET /metrics` — Prometheus scrape
- `GET /health` — Returns "ok"
- `GET /` — Web UI dashboard (live metrics via SSE, sparkline charts)
- `GET /alerts` — Paginated alert history page
- `GET /config` — Configuration editor (reads/writes `.env` file)
- `GET /api/events` — SSE stream of metric snapshots (JSON, every 5s)
- `GET /api/alerts` — Paginated alerts JSON (`{alerts, total, page, totalPages}`)
- `GET /api/config` — Current `.env` values as JSON
- `POST /api/config` — Update `.env` values (merge-update, requires container restart)
- `GET /api/history` — Sparkline history JSON (last 60 points per metric series)
- `GET /static/*` — Embedded CSS/JS assets

## Deployment

Docker Compose stack with 4 containers: `pingpong` (Go app, port 4040), `prometheus` (9090), `grafana` (3000), `apprise` (8000). The Go container requires `NET_RAW` capability for ICMP. Multi-stage Dockerfile installs `traceroute`, `iputils-ping`, and Ookla `speedtest` CLI in the runtime image.

## Key Conventions

- No web framework — stdlib `net/http` only
- Structured logging via `log/slog`
- Pure-Go SQLite (`modernc.org/sqlite`) — no CGO required
- Prometheus metrics use a dedicated registry (not the global default)
- Collectors are stateless; all state lives in metrics or the alert engine
- Traceroute hop latency metric uses separate `hop` (number) and `address` labels; `TracerouteHopLatency.Reset()` is called before each cycle to avoid stale series
- DNS resolution metric has `target` and `server` labels; failure counters track DNS, speedtest, and traceroute errors
- Connection flap counter (`pingpong_connection_flaps_total`) tracks up/down state transitions
- Speedtest info metric (`pingpong_speedtest_info`) exposes server name, location, and ISP as labels; `Reset()` is called before each update to avoid stale label sets
- Speedtest collector supports optional server pinning via `PINGPONG_SPEEDTEST_SERVER_ID`
- Speedtest and traceroute collectors shell out to CLI binaries only available inside the Docker image; their unit tests exercise output parsing only, not actual execution
- Ping integration tests require `CAP_NET_RAW` (root or Docker); use `-short` to skip them locally
- Web UI uses embedded templates and static assets (`//go:embed`); no external build step or bundler
- SSE clients receive an immediate snapshot on connect, then updates every 5s; slow clients have messages dropped (buffered channel, capacity 8)
- Dashboard JS fetches `/api/history` to seed sparkline buffers before connecting to SSE
- The shared SQLite database (`alerter.OpenDB()`) is opened once in `main.go` and passed to both `Queue` and `HistoryStore` via dependency injection
- Routes use Go 1.22+ method-pattern syntax (e.g., `"GET /health"`, `"POST /api/config"`)
