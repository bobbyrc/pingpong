# PingPong

Self-hosted internet health monitor. PingPong continuously measures the vitals of your internet connection — ping latency, jitter, packet loss, download/upload speed, DNS resolution time, traceroute hops, and connection uptime — exposes them as Prometheus metrics, and sends alerts via Apprise when things go wrong. A pre-built Grafana dashboard shows history at a glance.

## Quick Start

```bash
git clone <repo>
cp .env.example .env  # edit PINGPONG_APPRISE_URLS with your notification URLs
```

**Full stack** (includes Prometheus + Grafana):

```bash
docker compose --profile monitoring up -d
```

Then open Grafana at **http://localhost:3000** (admin / admin).

**Minimal** (PingPong + Apprise only — bring your own Prometheus/Grafana):

```bash
docker compose up -d
```

See [Using Your Own Monitoring Stack](#using-your-own-monitoring-stack) below.

## Using the Published Image

A pre-built image is published to the GitHub Container Registry at `ghcr.io/bcraig/pingpong:latest`, supporting both `linux/amd64` and `linux/arm64`. If you don't need to build from source, you can use this published image in your existing `docker-compose.yml` by replacing the `build: .` line with the image reference. Available tags are listed on the [GitHub Packages page](https://github.com/bcraig/pingpong/pkgs/container/pingpong).

```yaml
# docker-compose.yml — replace:
#   build: .
# with:
  image: ghcr.io/bcraig/pingpong:latest
```

## Using Your Own Monitoring Stack

If you already run Prometheus and Grafana, you can skip the bundled monitoring containers and point your existing tools at PingPong.

### Prometheus

Add PingPong as a scrape target in your Prometheus config:

```yaml
scrape_configs:
  - job_name: "pingpong"
    static_configs:
      - targets: ["<pingpong-host>:8080"]
    scrape_interval: 30s
```

### Grafana Dashboard

Import the pre-built dashboard into your Grafana:

1. Open Grafana → **Dashboards** → **Import**
2. Upload `grafana/dashboards/pingpong.json` from this repo
3. Click **Import**

The dashboard has a datasource dropdown at the top — select your Prometheus instance there. All panels update automatically.

## What It Monitors

| Metric | Description | Default Interval |
|--------|-------------|-----------------|
| Ping latency | Average round-trip time to each target (ms) | 60s |
| Jitter | Standard deviation of ping latency (ms) | 60s |
| Packet loss | Percentage of lost ICMP packets per target | 60s |
| Download speed | Download throughput via Ookla Speedtest CLI (Mbps) | 30m |
| Upload speed | Upload throughput via Ookla Speedtest CLI (Mbps) | 30m |
| DNS resolution | Time to resolve a domain against a configured DNS server (ms) | 5m |
| Traceroute | Hop count and per-hop latency to a target | 15m |
| Connection uptime | Up/down state derived from ping success, with downtime duration tracking | Continuous |

All intervals are configurable. See the Configuration section below.

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

4 containers: PingPong, Prometheus (optional), Grafana (optional), Apprise API. Use `--profile monitoring` to include Prometheus and Grafana.

## Configuration

Copy `.env.example` to `.env` and edit as needed. The file is organized into four sections:

**Targets** — which hosts to ping, which domain to resolve, which host to traceroute:
```env
PINGPONG_PING_TARGETS=1.1.1.1,8.8.8.8,208.67.222.222
PINGPONG_PING_COUNT=10
PINGPONG_DNS_TARGET=google.com
PINGPONG_DNS_SERVER=              # empty = use system default
PINGPONG_TRACEROUTE_TARGET=1.1.1.1
```

**Intervals** — how often each measurement runs:
```env
PINGPONG_PING_INTERVAL=60s
PINGPONG_SPEEDTEST_INTERVAL=30m
PINGPONG_DNS_INTERVAL=5m
PINGPONG_TRACEROUTE_INTERVAL=15m
```

**Thresholds** — what triggers an alert (set to `0` to disable):
```env
PINGPONG_ALERT_DOWNTIME_THRESHOLD=60s
PINGPONG_ALERT_PACKET_LOSS_THRESHOLD=10   # percent
PINGPONG_ALERT_PING_THRESHOLD=100         # ms
PINGPONG_ALERT_SPEED_THRESHOLD=50         # Mbps download
PINGPONG_ALERT_JITTER_THRESHOLD=30        # ms
```

**Notifications** — your Apprise notification URL(s):
```env
PINGPONG_APPRISE_URLS=discord://webhook_id/webhook_token
```

## Notifications

PingPong uses [Apprise](https://github.com/caronc/apprise) for notifications. Apprise supports 80+ services. Set one or more URLs in `PINGPONG_APPRISE_URLS` (comma-separated).

See the [Apprise wiki](https://github.com/caronc/apprise/wiki) for the full list of URL formats.

Common examples:

| Service | URL format |
|---------|-----------|
| Discord | `discord://webhook_id/webhook_token` |
| Slack | `slack://token_a/token_b/token_c` |
| Email | `mailto://user:pass@gmail.com` |
| Telegram | `tgram://bot_token/chat_id` |
| ntfy | `ntfy://topic` |

## Alert Rules

An alert fires when a threshold is crossed and the cooldown period has elapsed since the last alert of the same type.

| Alert | Threshold variable | Default |
|-------|--------------------|---------|
| Connection down | `PINGPONG_ALERT_DOWNTIME_THRESHOLD` | 60s |
| Packet loss | `PINGPONG_ALERT_PACKET_LOSS_THRESHOLD` | 10% |
| High ping latency | `PINGPONG_ALERT_PING_THRESHOLD` | 100ms |
| Low download speed | `PINGPONG_ALERT_SPEED_THRESHOLD` | 50 Mbps |
| High jitter | `PINGPONG_ALERT_JITTER_THRESHOLD` | 30ms |

**Cooldown** — After an alert fires, the same alert type is suppressed for `PINGPONG_ALERT_COOLDOWN` (default `15m`). This prevents notification spam during sustained outages.

Set any threshold to `0` to disable that alert entirely.

## Persistent Data

Alerts are queued in a SQLite database before being sent to Apprise. The database lives in a Docker volume, so unsent alerts survive container restarts and brief network outages. When PingPong comes back up, it replays any pending alerts automatically.

## Accessing Services

| Service | URL | Credentials | Profile |
|---------|-----|-------------|---------|
| Grafana | http://localhost:3000 | admin / admin | monitoring |
| Prometheus | http://localhost:9090 | — | monitoring |
| PingPong metrics | http://localhost:8080/metrics | — | always |
| PingPong health | http://localhost:8080/health | — | always |
