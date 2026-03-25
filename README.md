<p align="center">
  <h1 align="center">PingPong</h1>
  <p align="center">
    <strong>Self-hosted internet health monitoring for your homelab</strong>
  </p>
  <p align="center">
    Continuously measures ping, jitter, packet loss, speed, bufferbloat, DNS, traceroute, and connection uptime.
    <br />
    Automatically runs bandwidth tests when it detects network anomalies.
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
  - [Ping Collector](#ping-collector)
  - [NDT7 Speed Collector](#ndt7-speed-collector)
  - [Bufferbloat Collector](#bufferbloat-collector)
  - [Multi-Stream Throughput Collector](#multi-stream-throughput-collector)
  - [Bandwidth Orchestrator](#bandwidth-orchestrator)
  - [DNS Collector](#dns-collector)
  - [Traceroute Collector](#traceroute-collector)
  - [Connection State](#connection-state)
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
| **Speed** | NDT7 Speed Test | Pure-Go [M-Lab NDT7](https://www.measurementlab.net/tests/ndt/) speed test — download, upload, minimum RTT, and TCP retransmission rate. No proprietary binaries. |
| **Bufferbloat** | Latency Under Load | Measures how much your latency degrades during downloads, with A+ through F grading (like [Waveform](https://www.waveform.com/tools/bufferbloat)) |
| **Throughput** | Multi-Stream Max Speed | Parallel HTTP downloads to measure your connection's true maximum bandwidth |
| **Orchestrator** | Event-Triggered Testing | Automatically runs bandwidth tests when anomalies are detected — latency spikes, jitter, packet loss, DNS degradation, or connection recovery |
| **DNS** | Resolution Monitoring | Resolve multiple domains against multiple DNS servers (including system resolver) |
| **Route** | Traceroute | Per-hop latency and hop count to a target |
| **Alert** | 80+ Notification Services | Powered by [Apprise](https://github.com/caronc/apprise) — Discord, Slack, Telegram, ntfy, email, Pushover, Gotify, and [many more](https://github.com/caronc/apprise/wiki) |
| **UI** | Built-in Dashboard | Dark-themed real-time dashboard with SSE streaming, sparkline charts, bufferbloat grades, alert history, and config editor |
| **Grafana** | Pre-built Dashboard | Published to [grafana.com](https://grafana.com/grafana/dashboards/24995) (ID 24995) — import in one click |
| **Metrics** | 31 Prometheus Metrics | Expose everything to your existing monitoring stack via `/metrics` |
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

> **Note:** The first NDT7 speed test takes about 20 seconds. In the default **event** bandwidth mode, the orchestrator waits ~30 seconds after startup before running the first baseline test, and needs a few ping cycles before anomaly-based triggers become active. Speed metrics will be empty during this brief warmup. See [Bandwidth Orchestrator](#bandwidth-orchestrator) for details.

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

PingPong runs multiple measurement collectors in parallel. Some run on fixed intervals (ping, DNS, traceroute), while bandwidth tests can be driven by a smart orchestrator that reacts to network anomalies.

### Ping Collector

Sends ICMP pings to each target and measures:
- **Average latency** (ms) — round-trip time
- **Min / Max latency** (ms) — best and worst case
- **Jitter** (ms) — standard deviation of latency, indicating connection stability
- **Packet loss** (%) — percentage of dropped packets

Default targets: `1.1.1.1` (Cloudflare), `8.8.8.8` (Google), `208.67.222.222` (OpenDNS).

### NDT7 Speed Collector

Runs a single-stream speed test using [M-Lab's NDT7 protocol](https://www.measurementlab.net/tests/ndt/) — the same infrastructure behind Google's "internet speed test" and many academic network measurement studies. NDT7 is a pure-Go implementation, meaning no external binaries or proprietary dependencies.

Each test takes approximately 20 seconds (10s download + 10s upload) and captures:
- **Download speed** (Mbps) — single-stream throughput
- **Upload speed** (Mbps) — single-stream throughput
- **Minimum RTT** (ms) — the lowest round-trip time observed during the test (from TCP INFO), representing your connection's true baseline latency
- **TCP retransmission rate** (0.0–1.0) — fraction of bytes that were retransmitted (BytesRetrans / BytesSent), indicating network congestion or link errors
- **Server name** — the M-Lab server used for the test

> **Why NDT7 instead of Ookla?** NDT7 is open-source, runs as pure Go (no 30MB proprietary binary), provides richer TCP-level diagnostics (RTT, retransmissions), and uses M-Lab's globally distributed, research-grade infrastructure. The backward-compatible metrics `pingpong_download_speed_mbps` and `pingpong_upload_speed_mbps` continue to work — they're populated from NDT7 results.

### Bufferbloat Collector

Measures **latency under load** — how much your ping degrades when your connection is saturated with traffic. This is one of the most important indicators of internet quality that most monitoring tools miss.

The test works in three phases:
1. **Idle baseline** — sends 5 ICMP pings to establish your unloaded latency
2. **Load generation** — starts a large HTTP download from Cloudflare's CDN
3. **Loaded measurement** — sends 25 ICMP pings at 200ms intervals (~5s) *while the download is running* and takes the median

The result is a latency increase value and a letter grade:

| Grade | Latency Increase | What It Means |
|-------|------------------|---------------|
| **A+** | < 5ms | Excellent — no detectable bufferbloat |
| **A** | 5–30ms | Great — minimal impact under load |
| **B** | 30–60ms | Good — noticeable but manageable |
| **C** | 60–200ms | Fair — video calls and gaming will suffer under load |
| **D** | 200–400ms | Poor — significant degradation when bandwidth is used |
| **F** | > 400ms | Failing — your router's buffers are flooding with data |

> **What is bufferbloat?** When your router or ISP equipment has oversized network buffers, large downloads fill those buffers to capacity. This forces *all* traffic — including your video calls, games, and web browsing — to wait in line behind download packets. The result is massive latency spikes whenever someone on your network uses significant bandwidth. The fix is usually enabling SQM/QoS on your router or upgrading to a router with smart queue management. Learn more at [bufferbloat.net](https://www.bufferbloat.net/projects/bloat/wiki/).

The bufferbloat test uses `PINGPONG_BUFFERBLOAT_TARGET` for the ICMP pings (defaults to the first ping target) and downloads from `PINGPONG_BUFFERBLOAT_DOWNLOAD_URL` (defaults to Cloudflare's speed test CDN).

### Multi-Stream Throughput Collector

While NDT7 measures single-stream speed (limited by TCP congestion control), the throughput collector runs **parallel HTTP downloads** to saturate your connection and find its true maximum bandwidth. This is the closest equivalent to what browser-based speed tests (like fast.com or speedtest.net) report.

This collector is **opt-in** — set `PINGPONG_THROUGHPUT_DOWNLOAD_URL` to enable it (e.g., `https://speed.cloudflare.com/__down?bytes=250000000`).

- Downloads using 4 parallel streams (configurable, 1–16)
- Each test runs for 10 seconds (configurable)
- Reports the aggregate download speed across all streams

This is useful for comparing your connection's actual capacity against what your ISP advertises. A large gap between NDT7 single-stream and multi-stream throughput can indicate TCP tuning issues or high-latency paths where parallel streams help.

### Bandwidth Orchestrator

Instead of running bandwidth tests on a fixed schedule (which wastes resources when your connection is fine and misses problems between intervals), PingPong can react to network conditions in real time.

The orchestrator watches incoming ping and DNS measurements for anomalies and triggers bandwidth tests when something looks wrong:

| Trigger | Condition | What It Catches |
|---------|-----------|-----------------|
| **Latency spike** | Ping latency exceeds 2x the rolling baseline | Congestion, routing changes, ISP throttling |
| **Jitter spike** | Jitter exceeds 3x the rolling baseline | Network instability, wireless interference |
| **Packet loss** | Any loss above 1% | Connection degradation, failing hardware |
| **DNS degradation** | Resolution time exceeds 2x the rolling baseline | DNS server issues, hijacking |
| **Connection recovery** | Connection comes back after a period of downtime | Post-outage speed verification |
| **Baseline** | Scheduled periodic test (every 6 hours by default) | Trend tracking even when things are stable |

PingPong supports two bandwidth modes, controlled by `PINGPONG_BANDWIDTH_MODE`:

- **`event`** (default) — the orchestrator watches for anomalies and triggers tests reactively. Baseline tests still run on a configurable interval to maintain trend data. This mode is smarter about resource usage and catches problems faster.
- **`scheduled`** — bandwidth tests run on fixed intervals (like traditional speed test monitoring). Use this if you prefer predictable, clock-driven behavior.

The orchestrator also enforces minimum intervals between tests to avoid overwhelming your connection or M-Lab's servers:
- NDT7 tests: at most once every 4 hours (configurable)
- Bufferbloat tests: at most once every 1 hour (configurable)
- Trigger-based tests: at most once every 30 minutes (configurable)

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

### Collector Summary

| Collector | Key Measurements | Default Interval |
|-----------|-----------------|-----------------|
| Ping | Latency (avg/min/max), jitter, packet loss per target | 60s |
| NDT7 Speed | Download/upload (Mbps), min RTT, retransmission rate | Event-driven (baseline every 6h) |
| Bufferbloat | Latency under load, A+–F grade, idle/loaded latency | Event-driven (baseline every 6h) |
| Throughput | Max download speed (multi-stream parallel) | 24h |
| DNS | Resolution time, failure count per target/server | 5m |
| Traceroute | Hop count, per-hop latency per target | 15m |
| Connection | Up/down state, downtime total, flap count | Continuous (derived from ping) |

---

## Configuration Reference

All configuration is done through environment variables in the `.env` file. Copy `.env.example` to `.env` to get started — sensible defaults are provided for everything.

You can edit settings in two ways:

1. **Edit `.env` directly** on the host, then restart: `docker compose restart pingpong`
2. **Use the built-in config editor** at [http://localhost:4040/config](http://localhost:4040/config), then restart: `docker compose restart pingpong`

Both methods write to the same file. The default `docker-compose.yml` volume-mounts your host `.env` into the container at `/app/.env`, so the config editor's changes are preserved across container restarts.

> **Note:** A container restart is always required after changing configuration. If you've customized your Docker Compose setup and removed the `.env` volume mount, the config editor will write to a file inside the container that is lost on restart — see [Troubleshooting](#troubleshooting).

<details>
<summary><strong>Measurement Targets</strong> — what to monitor</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_PING_TARGETS` | Comma-separated list of IPs or hostnames to ping | `1.1.1.1,8.8.8.8,208.67.222.222` |
| `PINGPONG_PING_COUNT` | Number of ICMP packets per ping cycle | `25` |
| `PINGPONG_DNS_TARGETS` | Comma-separated domains to resolve | `google.com,cloudflare.com,github.com` |
| `PINGPONG_DNS_SERVERS` | Comma-separated DNS servers to test (empty = system resolver only) | _(empty)_ |
| `PINGPONG_TRACEROUTE_TARGET` | Host to traceroute | `1.1.1.1` |

**Examples:**

```env
# Monitor your router, ISP DNS, and a public DNS
PINGPONG_PING_TARGETS=192.168.1.1,1.1.1.1,8.8.8.8

# Test DNS resolution against Cloudflare and Google DNS
PINGPONG_DNS_SERVERS=1.1.1.1,8.8.8.8
```

</details>

<details>
<summary><strong>Measurement Intervals</strong> — how often to check</summary>

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_PING_INTERVAL` | Time between ping cycles | `60s` |
| `PINGPONG_SPEEDTEST_INTERVAL` | Time between NDT7 speed tests (scheduled mode only) | `30m` |
| `PINGPONG_DNS_INTERVAL` | Time between DNS checks | `5m` |
| `PINGPONG_TRACEROUTE_INTERVAL` | Time between traceroutes | `15m` |

Values use Go duration syntax: `30s`, `5m`, `1h`, `2h30m`.

> **Note:** `PINGPONG_SPEEDTEST_INTERVAL` only applies in `scheduled` bandwidth mode. In the default `event` mode, the orchestrator controls when speed tests run. See the Bandwidth & Bufferbloat section below.

</details>

<details>
<summary><strong>Bandwidth & Bufferbloat</strong> — speed testing and latency under load</summary>

**Bandwidth mode** controls how speed tests are scheduled:

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_BANDWIDTH_MODE` | `event` (anomaly-driven) or `scheduled` (fixed interval) | `event` |

**Bufferbloat settings:**

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_BUFFERBLOAT_TARGET` | IP/hostname for ICMP pings during load test (empty = first ping target) | _(empty)_ |
| `PINGPONG_BUFFERBLOAT_DOWNLOAD_URL` | URL for load-generation download | `https://speed.cloudflare.com/__down?bytes=100000000` |
| `PINGPONG_BUFFERBLOAT_INTERVAL` | Time between bufferbloat tests (scheduled mode only) | `6h` |

**Multi-stream throughput settings:**

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_THROUGHPUT_DOWNLOAD_URL` | URL for parallel downloads (empty = disabled; set to enable, e.g. `https://speed.cloudflare.com/__down?bytes=250000000`) | _(empty)_ |
| `PINGPONG_THROUGHPUT_STREAMS` | Number of parallel download streams (1–16) | `4` |
| `PINGPONG_THROUGHPUT_DURATION` | How long each throughput test runs | `10s` |
| `PINGPONG_THROUGHPUT_INTERVAL` | Time between throughput tests | `24h` |

**Event-mode orchestrator settings** (only apply when `PINGPONG_BANDWIDTH_MODE=event`):

| Variable | Description | Default |
|----------|-------------|---------|
| `PINGPONG_BANDWIDTH_BASELINE_INTERVAL` | How often to run baseline bandwidth tests | `6h` |
| `PINGPONG_BANDWIDTH_MIN_NDT7_INTERVAL` | Minimum time between NDT7 tests | `4h` |
| `PINGPONG_BANDWIDTH_MIN_BLOAT_INTERVAL` | Minimum time between bufferbloat tests | `1h` |
| `PINGPONG_BANDWIDTH_TRIGGER_COOLDOWN` | Minimum time between trigger-based tests | `30m` |

**Examples:**

```env
# Use event-driven testing (default) — tests run when anomalies are detected
PINGPONG_BANDWIDTH_MODE=event

# Or use scheduled testing — run speed tests every 30 minutes
PINGPONG_BANDWIDTH_MODE=scheduled
PINGPONG_SPEEDTEST_INTERVAL=30m
PINGPONG_BUFFERBLOAT_INTERVAL=6h

# Customize bufferbloat test to ping your router during load
PINGPONG_BUFFERBLOAT_TARGET=192.168.1.1

# Increase throughput streams for very fast connections (1 Gbps+)
PINGPONG_THROUGHPUT_STREAMS=8

# Run baseline bandwidth tests more frequently
PINGPONG_BANDWIDTH_BASELINE_INTERVAL=2h
```

</details>

<details>
<summary><strong>Alert Thresholds</strong> — when to send notifications</summary>

| Variable | Description | Unit | Default |
|----------|-------------|------|---------|
| `PINGPONG_ALERT_DOWNTIME_THRESHOLD` | Alert after connection is down for this long | duration | `60s` |
| `PINGPONG_ALERT_PACKET_LOSS_THRESHOLD` | Alert when packet loss exceeds this value | percent | `10` |
| `PINGPONG_ALERT_PING_THRESHOLD` | Alert when average ping latency exceeds this value | ms | `100` |
| `PINGPONG_ALERT_SPEED_THRESHOLD` | Alert when download speed drops below this value (NDT7) | Mbps | `50` |
| `PINGPONG_ALERT_JITTER_THRESHOLD` | Alert when jitter exceeds this value | ms | `30` |
| `PINGPONG_ALERT_BUFFERBLOAT_GRADE` | Alert when bufferbloat grade is at or below this level | grade | `D` |
| `PINGPONG_ALERT_COOLDOWN` | Suppress duplicate alerts for this duration | duration | `15m` |
| `PINGPONG_ALERT_MAX_RETRIES` | Max delivery attempts for a failed alert | count | `30` |
| `PINGPONG_ALERT_RETRY_INTERVAL` | Time between delivery retries | duration | `60s` |

**Set any numeric threshold to `0` to disable that alert type entirely.** To disable bufferbloat alerts, set the grade to `0` (any unrecognized grade value disables the alert).

Bufferbloat grade thresholds use letter grades: `A+`, `A`, `B`, `C`, `D`, `F`. Setting it to `D` (the default) means you'll be alerted when the grade is D or F. Setting it to `B` means you'll be alerted on B, C, D, or F.

```env
# Only alert on downtime and speed — disable the rest
PINGPONG_ALERT_DOWNTIME_THRESHOLD=60s
PINGPONG_ALERT_SPEED_THRESHOLD=25
PINGPONG_ALERT_PACKET_LOSS_THRESHOLD=0
PINGPONG_ALERT_PING_THRESHOLD=0
PINGPONG_ALERT_JITTER_THRESHOLD=0
PINGPONG_ALERT_BUFFERBLOAT_GRADE=0

# Alert on even moderate bufferbloat
PINGPONG_ALERT_BUFFERBLOAT_GRADE=C
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
| Low download speed | NDT7 download Mbps < threshold | 50 Mbps | Set to `0` |
| High jitter | Jitter > threshold (per target) | 30ms | Set to `0` |
| Bufferbloat detected | Grade at or below threshold | D | Set to `0` |

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

PingPong exposes 32 metrics at `/metrics`. All metrics use the `pingpong_` prefix.

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

**Speed metrics (backward-compatible aliases)**

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_download_speed_mbps` | gauge | Download speed in Mbps (populated from NDT7) |
| `pingpong_upload_speed_mbps` | gauge | Upload speed in Mbps (populated from NDT7) |

**NDT7 metrics**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `pingpong_ndt7_download_speed_mbps` | gauge | — | NDT7 download speed in Mbps (single stream) |
| `pingpong_ndt7_upload_speed_mbps` | gauge | — | NDT7 upload speed in Mbps (single stream) |
| `pingpong_ndt7_min_rtt_ms` | gauge | — | Minimum RTT observed during NDT7 test |
| `pingpong_ndt7_retransmission_rate` | gauge | — | TCP retransmission rate (0.0–1.0) |
| `pingpong_ndt7_failures_total` | counter | — | Total NDT7 test failures |
| `pingpong_ndt7_info` | gauge | `server_name` | NDT7 server metadata (value is always 1) |

**Bufferbloat metrics**

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_bufferbloat_latency_increase_ms` | gauge | Latency increase under load in milliseconds |
| `pingpong_bufferbloat_grade` | gauge | Bufferbloat grade as numeric value (A+=6, A=5, B=4, C=3, D=2, F=1) |
| `pingpong_bufferbloat_download_speed_mbps` | gauge | Download speed during bufferbloat test (byproduct) |
| `pingpong_bufferbloat_idle_latency_ms` | gauge | Idle latency before load in milliseconds |
| `pingpong_bufferbloat_loaded_latency_ms` | gauge | Loaded latency during download in milliseconds |
| `pingpong_bufferbloat_failures_total` | counter | Total bufferbloat test failures |

**Multi-stream throughput metrics**

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_max_download_speed_mbps` | gauge | Maximum download speed from multi-stream test in Mbps |
| `pingpong_throughput_streams` | gauge | Number of parallel streams used in throughput test |
| `pingpong_throughput_failures_total` | counter | Total multi-stream throughput test failures |

**Bandwidth orchestrator metrics** (labels: `reason`)

| Metric | Type | Description |
|--------|------|-------------|
| `pingpong_bandwidth_test_triggers_total` | counter | Total completed bandwidth tests by trigger reason |

Trigger reason labels: `baseline`, `latency_spike`, `jitter_spike`, `packet_loss`, `dns_slow`, `connection_recovery`.

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
│  │   Grafana   │             │  • NDT7 (M-Lab speed test) │ │
│  │    :3000    │             │  • Bufferbloat (load test)  │ │
│  └─────────────┘             │  • Throughput (multi-stream)│ │
│                              │  • DNS (multi-server)       │ │
│  ┌─────────────┐             │  • Traceroute               │ │
│  │    Loki     │             │                             │ │
│  │    :3100    │             │  Bandwidth Orchestrator:    │ │
│  └──────▲──────┘             │  • Anomaly detection        │ │
│         │                    │  • Event-triggered testing  │ │
│  ┌──────┴──────┐             │  • Rolling baselines        │ │
│  │  Promtail   │             │                             │ │
│  │  (logs)     │             │  Web UI:                    │ │
│  └─────────────┘             │  • Live dashboard (SSE)     │ │
│                              │  • Alert history            │ │
│                              │  • Config editor            │ │
│                              │                             │ │
│                              │  Alert engine:              │ │
│                              │  • Threshold evaluation     │ │
│                              │  • Per-target cooldowns     │ │
│                              │  • SQLite durable queue     │ │
│       Browser ◄─────────────►│  • Connection-aware retry   │ │
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

### Data flow

```
Ping/DNS collectors (every 60s / 5m)
  │
  ├──► Prometheus metrics (gauges, counters)
  │
  └──► Bandwidth Orchestrator (event mode)
        │
        ├── Anomaly detected? ──► NDT7 + Bufferbloat tests
        │                              │
        │                              ├──► Prometheus metrics
        │                              ├──► Alert engine (threshold check)
        │                              └──► Backward-compat speed gauges
        │
        └── Baseline interval? ──► Same as above

Throughput collector (every 24h) ──► Max download speed metric

Traceroute collector (every 15m) ──► Hop count + per-hop latency

SSE Broadcaster (every 5s)
  └──► Gathers all metrics ──► JSON snapshot ──► Browser dashboard
                                              └──► SQLite history (sparklines)
```

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
<summary><strong>Understanding bandwidth test modes</strong></summary>

PingPong gives you two fundamentally different approaches to bandwidth testing:

**Event mode** (`PINGPONG_BANDWIDTH_MODE=event`, the default) is the recommended approach for most users. Instead of blindly running speed tests every 30 minutes, PingPong watches your ping and DNS measurements for signs of trouble. When it detects a latency spike, packet loss, or DNS degradation, it *automatically* triggers an NDT7 speed test and a bufferbloat test to capture a full picture of your connection at that moment.

This is more efficient (fewer tests when things are fine) and more effective (catches problems as they happen, with full context about what triggered the test). The orchestrator maintains a rolling average (EMA) of your latency and DNS resolution times to establish baselines, then triggers when measurements deviate significantly.

Baseline tests still run every 6 hours (configurable) to ensure you have trend data even during stable periods.

**Scheduled mode** (`PINGPONG_BANDWIDTH_MODE=scheduled`) runs bandwidth tests on fixed intervals, like traditional speed test monitoring. Use this if:
- You want predictable, clock-driven behavior
- You're running comparative analysis and need evenly spaced data points
- You prefer simplicity over intelligence

In scheduled mode, `PINGPONG_SPEEDTEST_INTERVAL` and `PINGPONG_BUFFERBLOAT_INTERVAL` control the test frequencies.

</details>

<details>
<summary><strong>Comparing NDT7 vs multi-stream throughput results</strong></summary>

You may notice that NDT7 reports a different (usually lower) speed than the multi-stream throughput test. This is expected and informative:

- **NDT7** runs a single TCP stream. Its speed is bounded by TCP congestion control and your connection's latency. This is what a single application (video call, game server, file download) can realistically achieve.
- **Multi-stream throughput** runs 4+ parallel downloads simultaneously, saturating your connection. This is closer to what speed test websites report and represents your connection's total capacity.

A large gap between the two numbers (e.g., NDT7 shows 200 Mbps but throughput shows 800 Mbps) often means:
- **High latency** to the test server — TCP can't ramp up fast enough on a single stream
- **TCP buffer tuning issues** — your OS's TCP window settings may be limiting single-stream performance
- **Connection is capacity-limited per flow** — some ISPs shape traffic per TCP connection

If both numbers are close, your connection has good single-stream performance (low latency, well-tuned TCP), which means individual applications will see nearly full bandwidth.

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

PingPong sends ICMP packets for ping and bufferbloat measurements, which requires the `NET_RAW` Linux capability. This is already configured in the Docker Compose file:

```yaml
cap_add:
  - NET_RAW
```

If you're running PingPong outside Docker (Kubernetes, Podman, bare metal), you need to grant this capability explicitly. Without it, ping and bufferbloat measurements will fail with `socket: operation not permitted` errors.

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
<summary><strong>Migrating from Ookla Speedtest</strong></summary>

If you're upgrading from an older version of PingPong that used the Ookla Speedtest CLI, here's what changed:

**What's different:**
- Speed tests now use [M-Lab NDT7](https://www.measurementlab.net/tests/ndt/) instead of Ookla — no proprietary binary needed
- The Docker image is ~30MB smaller (no Ookla CLI)
- You get additional metrics: minimum RTT, TCP retransmission rate, server info
- `PINGPONG_SPEEDTEST_SERVER_ID` is no longer used (NDT7 auto-selects M-Lab servers)
- The legacy metrics `pingpong_speedtest_latency_ms`, `pingpong_speedtest_jitter_ms`, `pingpong_speedtest_failures_total`, and `pingpong_speedtest_info` have been removed (NDT7 provides `pingpong_ndt7_min_rtt_ms`, `pingpong_ndt7_failures_total`, and `pingpong_ndt7_info` instead)

**What's the same:**
- `pingpong_download_speed_mbps` and `pingpong_upload_speed_mbps` still work — they're populated from NDT7 results
- Existing Grafana dashboards using these metric names will continue to work
- Alert thresholds for speed (`PINGPONG_ALERT_SPEED_THRESHOLD`) work the same way

**Action required:**
- Remove `PINGPONG_SPEEDTEST_SERVER_ID` from your `.env` if present (it's ignored now)
- If you have Grafana panels using `pingpong_speedtest_latency_ms` or `pingpong_speedtest_jitter_ms`, update them to use `pingpong_ndt7_min_rtt_ms` and `pingpong_ndt7_retransmission_rate`
- Speed test results may differ between Ookla and NDT7 — this is normal. NDT7 single-stream results are often lower than Ookla's multi-stream tests. Use the new multi-stream throughput collector for a closer comparison.

</details>

<details>
<summary><strong>Building from source</strong></summary>

**Prerequisites:** Go 1.25+, Docker

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

Note: The traceroute collector shells out to the `traceroute` CLI binary which is only available inside the Docker image. Running the binary directly on your host will work for ping, DNS, NDT7 speed tests, bufferbloat, throughput, and the web UI, but traceroutes will fail unless that tool is installed separately.

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
<summary><strong>NDT7 speed test shows 0 or fails</strong></summary>

NDT7 is a pure-Go implementation that doesn't require external binaries, so it works both inside and outside Docker. If you're seeing failures:

**Check the logs:**

```bash
docker compose logs pingpong | grep -i ndt7
```

**Common causes:**
- **First run:** The first NDT7 test takes ~20 seconds. In event mode, the initial baseline starts ~30 seconds after startup, and anomaly-triggered tests only fire after enough warmup samples — so it's normal for speed metrics to be empty for the first minute or two.
- **M-Lab server unreachable:** NDT7 auto-discovers and connects to the nearest M-Lab server. If your network blocks non-standard ports or has aggressive firewalls, the test may fail.
- **Rate limiting:** M-Lab has usage policies. The default minimum interval of 4 hours between NDT7 tests respects these limits. Lowering `PINGPONG_BANDWIDTH_MIN_NDT7_INTERVAL` below 1 hour is not recommended.
- **In scheduled mode:** Make sure `PINGPONG_SPEEDTEST_INTERVAL` is set to a reasonable value (default `30m`).

</details>

<details>
<summary><strong>Bufferbloat test shows no data</strong></summary>

The bufferbloat collector requires both a valid ping target and a downloadable URL:

1. **Check that `PINGPONG_BUFFERBLOAT_TARGET` resolves** (or leave empty to use the first ping target):
   ```bash
   ping -c 3 1.1.1.1
   ```

2. **Verify the download URL is reachable:**
   ```bash
   curl -o /dev/null -w "%{http_code}" https://speed.cloudflare.com/__down?bytes=1000000
   ```

3. **Check for ICMP permission errors** — the bufferbloat test sends pings during the download, requiring `NET_RAW` capability (same as the ping collector).

4. **In event mode**, bufferbloat tests only run when triggered by anomalies or on the baseline interval (default 6 hours). Wait for a baseline run or manually check the logs for trigger events.

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
<summary><strong>Config editor changes don't take effect</strong></summary>

The config editor at `/config` writes to the `.env` file specified by `PINGPONG_ENV_FILE`. For this to work in Docker Compose, the host's `.env` must be mounted into the container. The default `docker-compose.yml` already includes this:

```yaml
volumes:
  - ./.env:/app/.env
```

If you've customized your setup and this mount is missing, the config editor writes to a file inside the container that doesn't survive restarts. To fix it:

1. **Add the volume mount** to your `docker-compose.yml` (see above)
2. **Ensure `PINGPONG_ENV_FILE`** is set to `/app/.env` in your `.env` file
3. **Restart the container:** `docker compose restart pingpong`

If you're running PingPong outside Docker, set `PINGPONG_ENV_FILE` to the path of your `.env` file (defaults to `.env` in the working directory).

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

<details>
<summary><strong>Bandwidth tests running too frequently or not at all</strong></summary>

**Tests not running in event mode:**
- The orchestrator needs ~5 ping cycles (5 minutes with default settings) to build a baseline before it can detect anomalies. Bandwidth tests triggered by anomalies will only start after this initial warmup period.
- Baseline tests run every 6 hours by default. If your connection is stable, you won't see triggered tests.
- Check `curl -s http://localhost:4040/metrics | grep bandwidth_test_triggers` to see trigger counts.

**Tests running too often:**
- Increase `PINGPONG_BANDWIDTH_TRIGGER_COOLDOWN` (default 30 minutes) to throttle trigger-based tests.
- Increase `PINGPONG_BANDWIDTH_MIN_NDT7_INTERVAL` (default 4 hours) or `PINGPONG_BANDWIDTH_MIN_BLOAT_INTERVAL` (default 1 hour).
- If your connection is genuinely unstable, event mode will trigger frequently. Consider switching to `PINGPONG_BANDWIDTH_MODE=scheduled` for predictable behavior.

</details>

---

<p align="center">
  <sub>Built with Go, SQLite, and a healthy distrust of ISP uptime claims.</sub>
</p>
