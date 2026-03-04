# PingPong — Internet Health Monitor Design

## Overview

PingPong is a self-hosted application that passively monitors the health of your internet connection. It regularly measures connection vitals, sends notifications when something seems wrong, and integrates with Grafana for historical visualization.

Written in Go. Deployed via Docker Compose.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Docker Compose                      │
│                                                      │
│  ┌───────────┐   scrapes    ┌──────────────────────┐ │
│  │ Prometheus│◄────────────│    PingPong (Go)      │ │
│  │   :9090   │   /metrics  │       :8080           │ │
│  └─────┬─────┘             │                       │ │
│        │                   │  Measurement loops:    │ │
│  ┌─────▼─────┐             │  • Ping / jitter      │ │
│  │  Grafana  │             │  • Packet loss         │ │
│  │   :3000   │             │  • Speed test (Ookla)  │ │
│  └───────────┘             │  • DNS resolution      │ │
│                            │  • Traceroute          │ │
│                            │                       │ │
│                            │  Alert engine:         │ │
│                            │  • Threshold eval      │ │
│                            │  • SQLite alert queue   │ │
│                            │  • Retry loop          │ │
│                            └───────────┬───────────┘ │
│                                        │ POST        │
│                            ┌───────────▼───────────┐ │
│                            │    Apprise API        │ │
│                            │       :8000           │ │
│                            └───────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

**4 containers**: PingPong, Prometheus, Grafana, Apprise API.

## Metrics

| Metric | Method | Default Interval | Notes |
|--------|--------|-----------------|-------|
| Ping latency | ICMP ping to configurable targets (default: 1.1.1.1, 8.8.8.8) | 60s | Reports avg, min, max, stddev |
| Jitter | Calculated from ping variance | 60s | Derived from ping samples |
| Packet loss | % of failed pings per batch | 60s | Sends N pings per check (default 10) |
| Download speed | Ookla Speedtest CLI | 30m | Mbps |
| Upload speed | Ookla Speedtest CLI | 30m | Mbps |
| Speed test latency | Ookla Speedtest CLI | 30m | As reported by Ookla |
| DNS resolution time | Resolve a known domain | 5m | Tests configured DNS server |
| Connection uptime | Derived from ping success | Continuous | Tracks up/down transitions and duration |
| Traceroute hops | Traceroute to a target | 15m | Hop count + per-hop latency |

All intervals are configurable via environment variables.

## Prometheus Metrics Exposition

PingPong exposes a `/metrics` endpoint in Prometheus format. Key metric names:

- `pingpong_ping_latency_ms{target="..."}` — gauge, per-target
- `pingpong_ping_min_ms{target="..."}` — gauge
- `pingpong_ping_max_ms{target="..."}` — gauge
- `pingpong_jitter_ms{target="..."}` — gauge
- `pingpong_packet_loss_percent{target="..."}` — gauge
- `pingpong_download_speed_mbps` — gauge
- `pingpong_upload_speed_mbps` — gauge
- `pingpong_speedtest_latency_ms` — gauge
- `pingpong_dns_resolution_ms{target="..."}` — gauge
- `pingpong_connection_up` — gauge (1 = up, 0 = down)
- `pingpong_downtime_seconds_total` — counter
- `pingpong_traceroute_hops{target="..."}` — gauge
- `pingpong_traceroute_hop_latency_ms{target="...", hop="..."}` — gauge

## Alert Engine

### Threshold Rules

Configurable alert conditions (all optional, disabled by setting to 0):

- Connection down for > N seconds
- Packet loss > N%
- Ping latency > N ms
- Download speed < N Mbps
- Jitter > N ms

### Persistent Alert Queue

Alerts are queued in a local SQLite database to survive connection outages and container restarts.

Flow:
1. Alert condition triggers — write alert to SQLite with status "pending"
2. Attempt to POST to Apprise API
3. On success: mark as "sent", record timestamp
4. On failure: leave as "pending"
5. Retry loop checks for pending alerts periodically (default every 30s)
6. On startup: check for unsent alerts from previous runs

SQLite schema:
```sql
CREATE TABLE alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'pending',  -- 'pending', 'sent', 'failed_permanent'
    alert_type TEXT NOT NULL,                -- 'downtime', 'packet_loss', 'latency', etc.
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0
);
```

Alerts are marked `failed_permanent` after a configurable max retry count (default 100) to avoid infinite retries on malformed alerts.

### Cooldown

Each alert type has a cooldown period (default 15 minutes). After firing an alert, the same condition won't re-alert until the cooldown expires. This prevents notification spam during sustained issues.

## Grafana Dashboard

Pre-provisioned via Grafana's provisioning system (JSON dashboard + datasource YAML mounted into the Grafana container).

Dashboard panels:
- **Connection Status**: Current up/down indicator with uptime percentage (24h, 7d, 30d)
- **Speed Test History**: Download/upload speed over time (line chart)
- **Ping Latency**: Latency graph with jitter shown as error bands
- **Packet Loss**: Percentage over time (bar chart, highlights > 0%)
- **DNS Resolution Time**: Line chart
- **Traceroute**: Table showing current route with per-hop latency
- **Alert History**: Annotations on all panels marking when alerts fired

## Configuration

All configuration via environment variables with sensible defaults. A `.env.example` ships with the project.

```env
# Measurement targets
PINGPONG_PING_TARGETS=1.1.1.1,8.8.8.8,208.67.222.222
PINGPONG_PING_COUNT=10
PINGPONG_DNS_TARGET=google.com
PINGPONG_DNS_SERVER=              # empty = use system default
PINGPONG_TRACEROUTE_TARGET=1.1.1.1

# Measurement intervals
PINGPONG_PING_INTERVAL=60s
PINGPONG_SPEEDTEST_INTERVAL=30m
PINGPONG_DNS_INTERVAL=5m
PINGPONG_TRACEROUTE_INTERVAL=15m

# Alert thresholds (set to 0 to disable)
PINGPONG_ALERT_DOWNTIME_THRESHOLD=60s
PINGPONG_ALERT_PACKET_LOSS_THRESHOLD=10
PINGPONG_ALERT_PING_THRESHOLD=100
PINGPONG_ALERT_SPEED_THRESHOLD=50
PINGPONG_ALERT_JITTER_THRESHOLD=30
PINGPONG_ALERT_COOLDOWN=15m
PINGPONG_ALERT_MAX_RETRIES=100
PINGPONG_ALERT_RETRY_INTERVAL=30s

# Notifications (Apprise)
PINGPONG_APPRISE_URL=http://apprise:8000
PINGPONG_APPRISE_URLS=discord://webhook_id/webhook_token

# Server
PINGPONG_LISTEN_ADDR=:8080

# Data
PINGPONG_DATA_DIR=/data
```

## Go Project Structure

```
pingpong/
├── cmd/
│   └── pingpong/
│       └── main.go              # Entry point, wires everything together
├── internal/
│   ├── config/
│   │   └── config.go            # Env var parsing, validation, defaults
│   ├── collector/
│   │   ├── ping.go              # ICMP ping + jitter + packet loss
│   │   ├── speedtest.go         # Ookla CLI wrapper
│   │   ├── dns.go               # DNS resolution timing
│   │   └── traceroute.go        # Traceroute execution + parsing
│   ├── alerter/
│   │   ├── engine.go            # Threshold evaluation, cooldown logic
│   │   ├── queue.go             # SQLite-backed persistent queue
│   │   └── apprise.go           # Apprise API client
│   └── metrics/
│       └── metrics.go           # Prometheus metric definitions
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── prometheus.yml   # Auto-configure Prometheus datasource
│   │   └── dashboards/
│   │       └── dashboard.yml    # Dashboard provisioning config
│   └── dashboards/
│       └── pingpong.json        # The pre-built dashboard
├── docker-compose.yml
├── Dockerfile
├── prometheus.yml               # Prometheus scrape config
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

## Docker Compose Services

1. **pingpong** — The Go application. Needs `NET_RAW` capability for ICMP. Mounts a volume for SQLite data.
2. **prometheus** — Scrapes PingPong's `/metrics` endpoint. Mounts config and data volume.
3. **grafana** — Pre-provisioned with datasource and dashboard. Mounts provisioning files.
4. **apprise** — [caronc/apprise](https://hub.docker.com/r/caronc/apprise) API server. Receives notification POSTs from PingPong.

## Key Dependencies (Go)

- `github.com/prometheus/client_golang` — Prometheus metrics
- `github.com/go-ping/ping` — ICMP ping (no need for raw sockets with setcap)
- `modernc.org/sqlite` — Pure Go SQLite (no CGO required)
- Ookla Speedtest CLI — installed in Docker image (not a Go dependency)

## Non-Goals

- No web UI (Grafana handles all visualization)
- No user accounts or authentication (single-user self-hosted)
- No historical data beyond what Prometheus retains
- No multi-site monitoring (single vantage point)
