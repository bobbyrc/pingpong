<p align="center">
  <h1 align="center">PingPong</h1>
  <p align="center">
    <strong>Self-hosted internet health monitoring for your homelab</strong>
  </p>
  <p align="center">
    Continuously measures ping, jitter, packet loss, speed, DNS, traceroute, and connection uptime.
    <br />
    Sends alerts when things go wrong. Ships with a real-time dashboard and a Grafana dashboard.
  </p>
  <p align="center">
    <a href="https://github.com/bobbyrc/pingpong/pkgs/container/pingpong"><img src="https://img.shields.io/badge/ghcr.io-pingpong-blue?logo=docker" alt="Docker Image" /></a>
    <a href="https://github.com/bobbyrc/pingpong/actions/workflows/ci.yml"><img src="https://github.com/bobbyrc/pingpong/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
    <img src="https://img.shields.io/github/go-mod/go-version/bobbyrc/pingpong" alt="Go Version" />
    <a href="https://grafana.com/grafana/dashboards/24995"><img src="https://img.shields.io/badge/Grafana-Dashboard%2024995-F46800?logo=grafana" alt="Grafana Dashboard" /></a>
    <img src="https://img.shields.io/badge/platform-amd64%20%7C%20arm64-lightgrey" alt="Platform" />
  </p>
</p>

<p align="center">
  <img src="docs/images/web-ui.png" alt="PingPong Dashboard" width="700" />
</p>
<p align="center">
  <em>Built-in real-time dashboard with live metrics, sparkline history, and connection status</em>
</p>
<p align="center">
  <img src="docs/images/grafana-dashboard.png" alt="Grafana Dashboard" width="700" />
</p>
<p align="center">
  <em>Pre-built Grafana dashboard for long-term historical analysis (ID <a href="https://grafana.com/grafana/dashboards/24995">24995</a>)</em>
</p>

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Using the Published Docker Image](#using-the-published-docker-image)
- [What It Monitors](#what-it-monitors)
- [Configuration Reference](#configuration-reference)
- [Notifications & Alerts](#notifications--alerts)
- [Integrating with Your Existing Stack](#integrating-with-your-existing-stack)
- [Architecture](#architecture)
- [Accessing Services](#accessing-services)
- [Advanced Topics](#advanced-topics)
- [Troubleshooting](#troubleshooting)

---

## Features

| | Feature | Description |
|---|---|---|
| **Ping** | Latency, Jitter & Packet Loss | ICMP ping to multiple targets with avg/min/max latency, jitter (stddev), and packet loss tracking |
| **Speed** | Download & Upload | Ookla Speedtest CLI integration with optional server pinning |
| **DNS** | Resolution Monitoring | Resolve multiple domains against multiple DNS servers (including system resolver) |
| **Route** | Traceroute | Per-hop latency and hop count to a target |
| **Alert** | 80+ Notification Services | Powered by [Apprise](https://github.com/caronc/apprise) — Discord, Slack, Telegram, ntfy, email, Pushover, Gotify, and [many more](https://github.com/caronc/apprise/wiki) |
| **UI** | Built-in Dashboard | Dark-themed real-time dashboard with SSE streaming, sparkline charts, alert history, and config editor |
| **Grafana** | Pre-built Dashboard | Published to [grafana.com](https://grafana.com/grafana/dashboards/24995) (ID 24995) — import in one click |
| **Metrics** | 19 Prometheus Metrics | Expose everything to your existing monitoring stack via `/metrics` |
| **Data** | SQLite Persistence | Alerts survive restarts, sparkline history is preserved, pending alerts auto-retry on reconnect |
| **Deploy** | Multi-arch Docker | `linux/amd64` and `linux/arm64` — runs on Raspberry Pi, NAS, VM, bare metal |

---

## Quick Start

### 1. Clone and configure

```bash
git clone https://github.com/bobbyrc/pingpong.git
cd pingpong
cp .env.example .env
```

Open `.env` in your editor. The only setting you **need** to change is `PINGPONG_APPRISE_URLS` — this tells PingPong where to send alert notifications. For example, to get alerts in Discord:

```env
PINGPONG_APPRISE_URLS=discord://webhook_id/webhook_token
```

> **Tip:** You can skip this for now and add notification URLs later. PingPong will still collect metrics and display them in the dashboard — you just won't receive alerts.

### 2. Choose your deployment mode

**Full stack** — PingPong + Apprise + Prometheus + Grafana + Loki (recommended for new users):

```bash
docker compose --profile monitoring up -d
```

This gives you the complete experience: the built-in dashboard, Grafana for long-term history, and centralized log collection.

**Minimal** — PingPong + Apprise only (bring your own Prometheus/Grafana):

```bash
docker compose up -d
```

This starts only PingPong and Apprise. Metrics are exposed at `/metrics` for your existing stack to scrape. See [Integrating with Your Existing Stack](#integrating-with-your-existing-stack).

### 3. Open the dashboards

| Service | URL | Notes |
|---------|-----|-------|
| PingPong Dashboard | [http://localhost:4040](http://localhost:4040) | Live metrics appear within 60 seconds |
| Grafana | [http://localhost:3000](http://localhost:3000) | Login with `admin` / `admin` (full stack only) |

The PingPong dashboard updates in real time via server-sent events. Give it a minute for the first ping cycle to complete and you'll see data start flowing in.

> **Note:** The first speedtest takes ~30 seconds to run. You'll see download/upload speeds populate after the first cycle completes.

---

## Using the Published Docker Image

A pre-built multi-arch image is published to the GitHub Container Registry:

```
ghcr.io/bobbyrc/pingpong:latest
```

It supports `linux/amd64` and `linux/arm64` (Raspberry Pi, ARM NAS, etc.).

If you don't want to build from source, replace the `build: .` directive in `docker-compose.yml`:

```yaml
services:
  pingpong:
    # build: .                                    # comment out or remove
    image: ghcr.io/bobbyrc/pingpong:latest        # use the published image
```

Available tags: `latest`, `X.Y.Z` (specific version), `X.Y` (minor version). Tags are listed on the [GitHub Packages page](https://github.com/bobbyrc/pingpong/pkgs/container/pingpong).

---

## What It Monitors

PingPong runs four measurement collectors in parallel, each on its own configurable interval:

### Ping Collector

Sends ICMP pings to each target and measures:
- **Average latency** (ms) — round-trip time
- **Min / Max latency** (ms) — best and worst case
- **Jitter** (ms) — standard deviation of latency, indicating connection stability
- **Packet loss** (%) — percentage of dropped packets

Default targets: `1.1.1.1` (Cloudflare), `8.8.8.8` (Google), `208.67.222.222` (OpenDNS).

### Speed Collector

Runs the [Ookla Speedtest CLI](https://www.speedtest.net/apps/cli) and captures:
- **Download speed** (Mbps)
- **Upload speed** (Mbps)
- **Speedtest latency and jitter** (ms)
- **Server name, location, and ISP** (as metric labels)

### DNS Collector

Resolves configured domains against configured DNS servers and measures:
- **Resolution time** (ms) per target/server combination
- **Failure count** per target/server

The system resolver is always included, so you get a baseline even if you don't configure additional servers.

### Traceroute Collector

Runs `traceroute` to a target and records:
- **Hop count** — number of network hops to the target
- **Per-hop latency** (ms) — latency at each hop with the hop's IP address

### Connection State

Derived from ping results (not a separate collector):
- **Connection up/down** — binary state (1 = up, 0 = down)
- **Total downtime** (seconds) — cumulative counter
- **Connection flaps** — number of up/down transitions

| Collector | Metrics | Default Interval |
|-----------|---------|-----------------|
| Ping | Latency (avg/min/max), jitter, packet loss per target | 60s |
| Speed | Download, upload, speedtest latency/jitter, server info | 30m |
| DNS | Resolution time, failure count per target/server | 5m |
| Traceroute | Hop count, per-hop latency per target | 15m |
| Connection | Up/down state, downtime total, flap count | Continuous (derived from ping) |

---

## Configuration Reference

All configuration is done through environment variables in the `.env` file. Copy `.env.example` to `.env` to get started — sensible defaults are provided for everything.

> **Note:** After changing `.env` values, restart the PingPong container for changes to take effect: `docker compose restart pingpong`. You can also edit configuration live from the built-in config editor at [http://localhost:4040/config](http://localhost:4040/config), though a restart is still required.

<details>
<summary><strong>Measurement Targets</strong> — what to monitor</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_PING_TARGETS` | Comma-separated list of IPs or hostnames to ping | `1.1.1.1,8.8.8.8,208.67.222.222` |
| `PINGPONG_PING_COUNT` | Number of ICMP packets per ping cycle | `25` |
| `PINGPONG_DNS_TARGETS` | Comma-separated domains to resolve | `google.com,cloudflare.com,github.com` |
| `PINGPONG_DNS_SERVERS` | Comma-separated DNS servers to test (empty = system resolver only) | _(empty)_ |
| `PINGPONG_TRACEROUTE_TARGET` | Host to traceroute | `1.1.1.1` |
| `PINGPONG_SPEEDTEST_SERVER_ID` | Pin a specific Ookla speedtest server (empty = auto-select) | _(empty)_ |

**Examples:**

```env
# Monitor your router, ISP DNS, and a public DNS
PINGPONG_PING_TARGETS=192.168.1.1,1.1.1.1,8.8.8.8

# Test DNS resolution against Cloudflare and Google DNS
PINGPONG_DNS_SERVERS=1.1.1.1,8.8.8.8

# Pin speedtest to a specific server (find IDs at speedtest.net)
PINGPONG_SPEEDTEST_SERVER_ID=12345
```

</details>

<details>
<summary><strong>Measurement Intervals</strong> — how often to check</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_PING_INTERVAL` | Time between ping cycles | `60s` |
| `PINGPONG_SPEEDTEST_INTERVAL` | Time between speed tests | `30m` |
| `PINGPONG_DNS_INTERVAL` | Time between DNS checks | `5m` |
| `PINGPONG_TRACEROUTE_INTERVAL` | Time between traceroutes | `15m` |

Values use Go duration syntax: `30s`, `5m`, `1h`, `2h30m`.

> **Tip:** Speed tests consume bandwidth and take ~30 seconds to run. Setting this below `10m` is not recommended for most home connections.

</details>

<details>
<summary><strong>Alert Thresholds</strong> — when to send notifications</summary>

| Variable | Description | Unit | Default |
|----------|-------------|------|---------|
| `PINGPONG_ALERT_DOWNTIME_THRESHOLD` | Alert after connection is down for this long | duration | `60s` |
| `PINGPONG_ALERT_PACKET_LOSS_THRESHOLD` | Alert when packet loss exceeds this value | percent | `10` |
| `PINGPONG_ALERT_PING_THRESHOLD` | Alert when average ping latency exceeds this value | ms | `100` |
| `PINGPONG_ALERT_SPEED_THRESHOLD` | Alert when download speed drops below this value | Mbps | `50` |
| `PINGPONG_ALERT_JITTER_THRESHOLD` | Alert when jitter exceeds this value | ms | `30` |
| `PINGPONG_ALERT_COOLDOWN` | Suppress duplicate alerts for this duration | duration | `15m` |
| `PINGPONG_ALERT_MAX_RETRIES` | Max delivery attempts for a failed alert | count | `30` |
| `PINGPONG_ALERT_RETRY_INTERVAL` | Time between delivery retries | duration | `60s` |

**Set any threshold to `0` to disable that alert type entirely.**

```env
# Only alert on downtime and speed — disable the rest
PINGPONG_ALERT_DOWNTIME_THRESHOLD=60s
PINGPONG_ALERT_SPEED_THRESHOLD=25
PINGPONG_ALERT_PACKET_LOSS_THRESHOLD=0
PINGPONG_ALERT_PING_THRESHOLD=0
PINGPONG_ALERT_JITTER_THRESHOLD=0
```

</details>

<details>
<summary><strong>Notifications</strong> — where to send alerts</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_APPRISE_URL` | Apprise API server URL | `http://apprise:8000` |
| `PINGPONG_APPRISE_URLS` | Notification destination URL(s) — comma-separated for multiple | _(empty)_ |

```env
# Single destination
PINGPONG_APPRISE_URLS=discord://webhook_id/webhook_token

# Multiple destinations (comma-separated)
PINGPONG_APPRISE_URLS=discord://webhook_id/webhook_token,tgram://bot_token/chat_id,ntfy://my-topic
```

See the [Notifications & Alerts](#notifications--alerts) section for URL format examples.

</details>

<details>
<summary><strong>Server</strong> — network and data settings</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_LISTEN_ADDR` | Address and port for the HTTP server | `:4040` |
| `PINGPONG_DATA_DIR` | Directory for the SQLite database | `/data` |
| `PINGPONG_ENV_FILE` | Path to the `.env` file (used by the config editor) | `.env` |

You generally don't need to change these unless you're running outside Docker or have a port conflict.

</details>

---

## Notifications & Alerts

### How alerts work

```
Collector measures a value
        │
        ▼
Engine compares against threshold
        │
        ▼ (threshold crossed)
Cooldown check — was this alert type:target sent recently?
        │
        ▼ (cooldown expired)
Alert queued in SQLite (durable)
        │
        ▼
Retry loop delivers to Apprise → notification service
```

1. **Threshold evaluation** — Each measurement is compared against its configured threshold. Alerts are per-target, so if you ping three hosts, each one can trigger independently.

2. **Cooldown** — After an alert fires for a specific type and target (e.g., "high latency on 1.1.1.1"), that exact combination is suppressed for the cooldown period (default 15 minutes). This prevents notification spam during sustained issues.

3. **Durable queue** — Alerts are written to SQLite before delivery is attempted. If delivery fails (Apprise is down, network issue), the alert is retried automatically on a configurable interval. Pending alerts survive container restarts.

4. **Connection-aware retry** — When the connection is down, PingPong pauses alert delivery retries (they'd fail anyway). When the connection comes back up, all pending alerts are flushed immediately.

### Notification services

PingPong uses [Apprise](https://github.com/caronc/apprise) to deliver notifications to 80+ services. Set your destination(s) in `PINGPONG_APPRISE_URLS`.

| Service | URL Format | Example |
|---------|-----------|---------|
| Discord | `discord://webhook_id/webhook_token` | `discord://123456/abcdef...` |
| Slack | `slack://token_a/token_b/token_c` | `slack://xoxb-.../...` |
| Telegram | `tgram://bot_token/chat_id` | `tgram://123:ABC.../456789` |
| ntfy | `ntfy://topic` or `ntfy://user:pass@ntfy.sh/topic` | `ntfy://pingpong-alerts` |
| Pushover | `pover://user_key@app_token` | `pover://abc123@def456` |
| Gotify | `gotify://hostname/token` | `gotify://gotify.local/AbCdEf` |
| Email (SMTP) | `mailto://user:pass@host` | `mailto://me:pwd@gmail.com` |
| Home Assistant | `hassio://host/accesstoken` | `hassio://ha.local/eyJ...` |

For the complete list of 80+ supported services, see the [Apprise Wiki](https://github.com/caronc/apprise/wiki).

**Multiple destinations** — separate URLs with commas:

```env
PINGPONG_APPRISE_URLS=discord://webhook_id/token,ntfy://my-alerts,tgram://bot/chat
```

### Alert rules

| Alert | Condition | Default Threshold | Disable |
|-------|-----------|------------------|---------|
| Connection down | Ping fails for longer than threshold | 60s | Set to `0` |
| High packet loss | Loss % exceeds threshold (per target) | 10% | Set to `0` |
| High latency | Avg ping > threshold (per target) | 100ms | Set to `0` |
| Low download speed | Download Mbps < threshold | 50 Mbps | Set to `0` |
| High jitter | Jitter > threshold (per target) | 30ms | Set to `0` |

---

## Integrating with Your Existing Stack

Already running Prometheus and Grafana? Skip the bundled monitoring containers and point your existing tools at PingPong.

### Prometheus scrape config

Add PingPong as a scrape target in your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "pingpong"
    scrape_interval: 30s
    static_configs:
      - targets: ["<pingpong-host>:4040"]
```

Replace `<pingpong-host>` with the hostname or IP where PingPong is running.

### Grafana dashboard

Import the pre-built dashboard in one click:

1. Open Grafana → **Dashboards** → **Import**
2. Enter dashboard ID **`24995`**
3. Select your Prometheus datasource and click **Import**

The dashboard is published on [grafana.com](https://grafana.com/grafana/dashboards/24995). Alternatively, upload `grafana/dashboards/pingpong.json` from this repo.

### Docker network integration

If PingPong and your Prometheus are in separate Docker Compose stacks, they need a shared network:

```bash
docker network create monitoring
```

Add the shared network to your PingPong compose file:

```yaml
services:
  pingpong:
    networks:
      - default
      - monitoring

networks:
  monitoring:
    external: true
```

Then use `pingpong` as the scrape target hostname in your Prometheus config.

### Prometheus metrics reference

PingPong exposes 19 metrics at `/metrics`. All metrics use the `pingpong_` prefix.

<details>
<summary><strong>Click to expand full metrics list</strong></summary>

**Ping metrics** (labels: `target`)

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_ping_latency_ms` | gauge | Average ping latency in milliseconds |
| `pingpong_ping_min_ms` | gauge | Minimum ping latency in milliseconds |
| `pingpong_ping_max_ms` | gauge | Maximum ping latency in milliseconds |
| `pingpong_jitter_ms` | gauge | Ping jitter (standard deviation) in milliseconds |
| `pingpong_packet_loss_percent` | gauge | Packet loss percentage |

**Speed metrics**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `pingpong_download_speed_mbps` | gauge | — | Download speed in Mbps |
| `pingpong_upload_speed_mbps` | gauge | — | Upload speed in Mbps |
| `pingpong_speedtest_latency_ms` | gauge | — | Latency reported by speed test |
| `pingpong_speedtest_jitter_ms` | gauge | — | Jitter reported by speed test |
| `pingpong_speedtest_info` | gauge | `server_name`, `server_location`, `isp` | Speedtest server metadata (value is always 1) |

**DNS metrics** (labels: `target`, `server`)

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_dns_resolution_ms` | gauge | DNS resolution time in milliseconds |
| `pingpong_dns_failures_total` | counter | Total DNS lookup failures |

**Traceroute metrics**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `pingpong_traceroute_hops` | gauge | `target` | Number of hops in traceroute |
| `pingpong_traceroute_hop_latency_ms` | gauge | `target`, `hop`, `address` | Latency per traceroute hop |

**Connection & reliability metrics**

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_connection_up` | gauge | Whether the connection is up (1) or down (0) |
| `pingpong_downtime_seconds_total` | counter | Total downtime in seconds |
| `pingpong_connection_flaps_total` | counter | Total up/down state transitions |
| `pingpong_speedtest_failures_total` | counter | Total speedtest execution failures |
| `pingpong_traceroute_failures_total` | counter | Total traceroute execution failures |

</details>

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       Docker Compose                         │
│                                                              │
│  ┌─────────────┐   scrapes   ┌─────────────────────────────┐ │
│  │ Prometheus  │◄────────────│       PingPong (Go)         │ │
│  │    :9090    │   /metrics  │          :4040              │ │
│  └──────┬──────┘             │                             │ │
│         │                    │  Collectors:                │ │
│  ┌──────▼──────┐             │  • Ping (ICMP, multi-target)│ │
│  │   Grafana   │             │  • Speed (Ookla CLI)        │ │
│  │    :3000    │             │  • DNS (multi-server)       │ │
│  └─────────────┘             │  • Traceroute               │ │
│                              │                             │ │
│  ┌─────────────┐             │  Web UI:                    │ │
│  │    Loki     │             │  • Live dashboard (SSE)     │ │
│  │    :3100    │             │  • Alert history            │ │
│  └──────▲──────┘             │  • Config editor            │ │
│         │                    │                             │ │
│  ┌──────┴──────┐             │  Alert engine:              │ │
│  │  Promtail   │             │  • Threshold evaluation     │ │
│  │  (logs)     │             │  • Per-target cooldowns     │ │
│  └─────────────┘             │  • SQLite durable queue     │ │
│                              │  • Connection-aware retry   │ │
│       Browser ◄─────────────►│                             │ │
│    (dashboard,               └──────────────┬──────────────┘ │
│     alerts,                                 │ POST /notify   │
│     config)                  ┌──────────────▼──────────────┐ │
│                              │       Apprise API           │ │
│                              │          :8000              │ │
│                              │   → Discord, Slack, etc.    │ │
│                              └─────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

**Full stack** (6 containers): PingPong, Apprise, Prometheus, Grafana, Loki, Promtail
**Minimal** (2 containers): PingPong, Apprise

The `--profile monitoring` flag controls whether Prometheus, Grafana, Loki, and Promtail are started. PingPong and Apprise always run.

---

## Accessing Services

| Service | URL | Credentials | Profile |
|---------|-----|-------------|---------|
| PingPong Dashboard | [http://localhost:4040](http://localhost:4040) | — | default |
| Alert History | [http://localhost:4040/alerts](http://localhost:4040/alerts) | — | default |
| Config Editor | [http://localhost:4040/config](http://localhost:4040/config) | — | default |
| Prometheus Metrics | [http://localhost:4040/metrics](http://localhost:4040/metrics) | — | default |
| Health Check | [http://localhost:4040/health](http://localhost:4040/health) | — | default |
| Grafana | [http://localhost:3000](http://localhost:3000) | `admin` / `admin` | monitoring |
| Prometheus UI | [http://localhost:9090](http://localhost:9090) | — | monitoring |
| Loki | [http://localhost:3100](http://localhost:3100) | — | monitoring |

---

## Advanced Topics

<details>
<summary><strong>Persistent data & SQLite</strong></summary>

PingPong stores all data in a single SQLite database at `{PINGPONG_DATA_DIR}/alerts.db` (default: `/data/alerts.db` inside the container).

The database contains two tables:
- **alerts** — Durable alert queue. Pending alerts survive restarts and are auto-retried when the connection comes back.
- **metric_history** — Sparkline data for the web dashboard (ping latency, download/upload speed). Pruned to the last 60 data points per metric series.

The Docker Compose file mounts a named volume (`pingpong-data`) to `/data`, so your data persists across container restarts and image updates.

</details>

<details>
<summary><strong>Speedtest server pinning</strong></summary>

By default, the Ookla Speedtest CLI auto-selects the nearest server. If you want consistent results for trend analysis, you can pin a specific server:

```env
PINGPONG_SPEEDTEST_SERVER_ID=12345
```

To find server IDs, run:

```bash
docker exec pingpong speedtest --servers
```

Pick a server from the list and use its ID. Pinning a server eliminates variability from server selection and gives you cleaner historical data.

</details>

<details>
<summary><strong>Running on ARM64 (Raspberry Pi, etc.)</strong></summary>

The published Docker image supports `linux/arm64` natively. No special configuration is needed:

```bash
docker compose up -d  # works on Raspberry Pi 4/5, ARM NAS, etc.
```

If building from source on ARM, the multi-stage Dockerfile handles cross-compilation automatically.

</details>

<details>
<summary><strong>NET_RAW capability</strong></summary>

PingPong sends ICMP packets for ping measurements, which requires the `NET_RAW` Linux capability. This is already configured in the Docker Compose file:

```yaml
cap_add:
  - NET_RAW
```

If you're running PingPong outside Docker (Kubernetes, Podman, bare metal), you need to grant this capability explicitly. Without it, ping measurements will fail with `socket: operation not permitted` errors.

**Kubernetes:** Add to your pod security context:
```yaml
securityContext:
  capabilities:
    add: ["NET_RAW"]
```

**Podman:** Same as Docker — `--cap-add=NET_RAW`

**Bare metal:** Run with `sudo` or set the capability on the binary:
```bash
sudo setcap cap_net_raw+ep ./pingpong
```

</details>

<details>
<summary><strong>Upgrading from Loki 2.x</strong></summary>

If you previously ran the monitoring profile with Loki 2.x, the existing `loki-data` Docker volume is owned by root. Loki 3.x runs as a non-root user (UID 10001) and will fail to start with a `permission denied` error.

To fix this, remove the old volume and let Docker recreate it:

```bash
docker compose --profile monitoring down
docker volume rm $(docker volume ls -q --filter name=loki-data)
docker compose --profile monitoring up -d
```

This deletes stored Loki log data. Prometheus metrics and Grafana dashboards are unaffected.

</details>

<details>
<summary><strong>Building from source</strong></summary>

**Prerequisites:** Go 1.24+, Docker

```bash
# Build the Docker image locally
docker compose build

# Or build just the Go binary (for development)
go build ./cmd/pingpong/

# Run tests
go test -short ./...        # skip integration tests
go test ./...               # all tests (needs CAP_NET_RAW for ping tests)

# Pre-commit quality gate
make check                  # runs vet + test + tidy check
```

Note: Speedtest and traceroute collectors shell out to CLI binaries that are only available inside the Docker image. Running the binary directly on your host will work for ping, DNS, and the web UI, but speed tests and traceroutes will fail unless those tools are installed.

</details>

<details>
<summary><strong>Makefile reference</strong></summary>

| Target | Description |
|--------|-------------|
| `make up` | Start PingPong + Apprise (default profile) |
| `make up-all` | Start everything including monitoring stack |
| `make down` | Stop default profile |
| `make down-all` | Stop everything including monitoring stack |
| `make rebuild` | Rebuild Docker image and restart |
| `make logs` | Tail PingPong container logs |
| `make logs-all` | Tail all container logs |
| `make test` | Run tests (skip integration) |
| `make test-all` | Run all tests (needs `CAP_NET_RAW`) |
| `make check` | Pre-commit quality gate (vet + test + tidy) |
| `make env-setup` | Copy `.env.example` → `.env` if missing |
| `make clean` | Remove binary + Docker volumes |

</details>

---

## Troubleshooting

<details>
<summary><strong>Ping measurements show "ping failed" errors</strong></summary>

PingPong requires the `NET_RAW` Linux capability to send ICMP packets. Make sure your Docker Compose file includes:

```yaml
cap_add:
  - NET_RAW
```

If running rootless Docker, Kubernetes, or Podman, see the [NET_RAW capability](#advanced-topics) section.

</details>

<details>
<summary><strong>Speedtest shows 0 or fails</strong></summary>

The Ookla Speedtest CLI is only available inside the Docker container. If you're running the Go binary directly on your host, speedtest will fail unless you install the CLI yourself.

Inside Docker, check the logs:

```bash
docker compose logs pingpong | grep -i speedtest
```

Common causes:
- **First run:** The first speedtest takes ~30 seconds. Wait for one full cycle.
- **Network issue:** The Ookla servers may be unreachable from your network.
- **Rate limiting:** Running speed tests too frequently can trigger rate limits. Keep the interval at 10m or higher.

</details>

<details>
<summary><strong>Alerts not sending</strong></summary>

1. **Check that `PINGPONG_APPRISE_URLS` is set** in your `.env` file. If it's empty, alerts are queued but have nowhere to go.

2. **Verify Apprise is running:**
   ```bash
   curl http://localhost:8000/status
   ```

3. **Check PingPong logs for delivery errors:**
   ```bash
   docker compose logs pingpong | grep -i alert
   ```

4. **Test Apprise directly:**
   ```bash
   curl -X POST http://localhost:8000/notify \
     -d "urls=discord://webhook_id/token" \
     -d "title=Test" \
     -d "body=Hello from PingPong"
   ```

5. **Check the alert queue** at [http://localhost:4040/alerts](http://localhost:4040/alerts) — pending alerts will show a "Pending" badge.

</details>

<details>
<summary><strong>Grafana shows "No data"</strong></summary>

1. **Verify Prometheus is scraping PingPong:**
   - Open [http://localhost:9090/targets](http://localhost:9090/targets)
   - Look for the `pingpong` job — it should show as `UP`

2. **Check that PingPong metrics are being generated:**
   ```bash
   curl -s http://localhost:4040/metrics | grep pingpong_ping_latency
   ```

3. **Select the correct datasource** — The Grafana dashboard has a Prometheus datasource dropdown at the top. Make sure it points to your Prometheus instance.

4. **Wait for data** — Grafana needs at least one scrape interval (~30s) plus one measurement cycle to show data.

</details>

<details>
<summary><strong>Loki/Promtail won't start</strong></summary>

**"permission denied" error:** If upgrading from Loki 2.x, the volume permissions are incompatible. See [Upgrading from Loki 2.x](#advanced-topics).

**Loki crash-loops:** Check the logs:
```bash
docker compose --profile monitoring logs loki
```

Common causes:
- Volume permissions (see above)
- Port 3100 already in use by another service
- Insufficient disk space for the TSDB store

</details>

<details>
<summary><strong>Dashboard not updating / SSE connection issues</strong></summary>

The dashboard uses Server-Sent Events (SSE) to receive real-time updates. If metrics aren't updating:

1. **Check the browser console** (F12) for connection errors to `/api/events`
2. **Verify PingPong is healthy:**
   ```bash
   curl http://localhost:4040/health
   ```
3. **Reverse proxy configuration** — If you're running PingPong behind nginx, Caddy, or Traefik, make sure your proxy is configured to pass SSE connections (disable buffering for `/api/events`).

   **nginx example:**
   ```nginx
   location /api/events {
       proxy_pass http://pingpong:4040;
       proxy_set_header Connection '';
       proxy_http_version 1.1;
       chunked_transfer_encoding off;
       proxy_buffering off;
       proxy_cache off;
   }
   ```

</details>

---

<p align="center">
  <sub>Built with Go, SQLite, and a healthy distrust of ISP uptime claims.</sub>
</p>
