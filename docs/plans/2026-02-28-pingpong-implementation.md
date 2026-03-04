# PingPong Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a self-hosted internet health monitor that measures connection vitals, alerts on problems, and graphs history in Grafana.

**Architecture:** Single Go binary running measurement loops (ping, speed, DNS, traceroute), exposing Prometheus metrics on `:8080/metrics`. Alert engine evaluates thresholds and pushes to Apprise API via SQLite-backed queue. Deployed as a 4-container Docker Compose stack (app, Prometheus, Grafana, Apprise).

**Tech Stack:** Go, Prometheus client_golang, go-ping, modernc.org/sqlite, Ookla Speedtest CLI, Docker Compose, Grafana, Apprise API.

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/pingpong/main.go`
- Create: directory structure for `internal/`

**Step 1: Initialize Go module**

Run: `go mod init github.com/bcraig/pingpong`

**Step 2: Create directory structure**

```bash
mkdir -p cmd/pingpong internal/config internal/collector internal/alerter internal/metrics
```

**Step 3: Create minimal main.go**

```go
package main

import "fmt"

func main() {
	fmt.Println("pingpong starting...")
}
```

**Step 4: Verify it compiles and runs**

Run: `go run cmd/pingpong/main.go`
Expected: `pingpong starting...`

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: scaffold project structure"
```

---

### Task 2: Configuration Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env vars that might be set
	for _, key := range []string{
		"PINGPONG_PING_TARGETS",
		"PINGPONG_PING_COUNT",
		"PINGPONG_PING_INTERVAL",
		"PINGPONG_SPEEDTEST_INTERVAL",
		"PINGPONG_DNS_TARGET",
		"PINGPONG_DNS_SERVER",
		"PINGPONG_DNS_INTERVAL",
		"PINGPONG_TRACEROUTE_TARGET",
		"PINGPONG_TRACEROUTE_INTERVAL",
		"PINGPONG_ALERT_DOWNTIME_THRESHOLD",
		"PINGPONG_ALERT_PACKET_LOSS_THRESHOLD",
		"PINGPONG_ALERT_PING_THRESHOLD",
		"PINGPONG_ALERT_SPEED_THRESHOLD",
		"PINGPONG_ALERT_JITTER_THRESHOLD",
		"PINGPONG_ALERT_COOLDOWN",
		"PINGPONG_ALERT_MAX_RETRIES",
		"PINGPONG_ALERT_RETRY_INTERVAL",
		"PINGPONG_APPRISE_URL",
		"PINGPONG_APPRISE_URLS",
		"PINGPONG_LISTEN_ADDR",
		"PINGPONG_DATA_DIR",
	} {
		os.Unsetenv(key)
	}

	cfg := Load()

	// Verify defaults
	if len(cfg.PingTargets) != 3 {
		t.Errorf("expected 3 default ping targets, got %d", len(cfg.PingTargets))
	}
	if cfg.PingTargets[0] != "1.1.1.1" {
		t.Errorf("expected first target 1.1.1.1, got %s", cfg.PingTargets[0])
	}
	if cfg.PingCount != 10 {
		t.Fatalf("expected ping count 10, got %d", cfg.PingCount)
	}
	if cfg.PingInterval != 60*time.Second {
		t.Fatalf("expected ping interval 60s, got %v", cfg.PingInterval)
	}
	if cfg.SpeedtestInterval != 30*time.Minute {
		t.Fatalf("expected speedtest interval 30m, got %v", cfg.SpeedtestInterval)
	}
	if cfg.DNSTarget != "google.com" {
		t.Fatalf("expected DNS target google.com, got %s", cfg.DNSTarget)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected listen addr :8080, got %s", cfg.ListenAddr)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("expected data dir /data, got %s", cfg.DataDir)
	}
	if cfg.AlertCooldown != 15*time.Minute {
		t.Fatalf("expected alert cooldown 15m, got %v", cfg.AlertCooldown)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PINGPONG_PING_TARGETS", "8.8.8.8,9.9.9.9")
	os.Setenv("PINGPONG_PING_COUNT", "5")
	os.Setenv("PINGPONG_PING_INTERVAL", "30s")
	os.Setenv("PINGPONG_LISTEN_ADDR", ":9090")
	defer func() {
		os.Unsetenv("PINGPONG_PING_TARGETS")
		os.Unsetenv("PINGPONG_PING_COUNT")
		os.Unsetenv("PINGPONG_PING_INTERVAL")
		os.Unsetenv("PINGPONG_LISTEN_ADDR")
	}()

	cfg := Load()

	if len(cfg.PingTargets) != 2 {
		t.Fatalf("expected 2 ping targets, got %d", len(cfg.PingTargets))
	}
	if cfg.PingTargets[0] != "8.8.8.8" {
		t.Fatalf("expected first target 8.8.8.8, got %s", cfg.PingTargets[0])
	}
	if cfg.PingCount != 5 {
		t.Fatalf("expected ping count 5, got %d", cfg.PingCount)
	}
	if cfg.PingInterval != 30*time.Second {
		t.Fatalf("expected ping interval 30s, got %v", cfg.PingInterval)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("expected listen addr :9090, got %s", cfg.ListenAddr)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `Load` not defined

**Step 3: Write the implementation**

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Measurement targets
	PingTargets      []string
	PingCount        int
	DNSTarget        string
	DNSServer        string
	TracerouteTarget string

	// Measurement intervals
	PingInterval       time.Duration
	SpeedtestInterval  time.Duration
	DNSInterval        time.Duration
	TracerouteInterval time.Duration

	// Alert thresholds (0 = disabled)
	AlertDowntimeThreshold   time.Duration
	AlertPacketLossThreshold float64
	AlertPingThreshold       float64
	AlertSpeedThreshold      float64
	AlertJitterThreshold     float64
	AlertCooldown            time.Duration
	AlertMaxRetries          int
	AlertRetryInterval       time.Duration

	// Notifications
	AppriseURL  string
	AppriseURLs string

	// Server
	ListenAddr string

	// Data
	DataDir string
}

func Load() *Config {
	return &Config{
		PingTargets:              getStringSlice("PINGPONG_PING_TARGETS", []string{"1.1.1.1", "8.8.8.8", "208.67.222.222"}),
		PingCount:                getInt("PINGPONG_PING_COUNT", 10),
		DNSTarget:                getString("PINGPONG_DNS_TARGET", "google.com"),
		DNSServer:                getString("PINGPONG_DNS_SERVER", ""),
		TracerouteTarget:         getString("PINGPONG_TRACEROUTE_TARGET", "1.1.1.1"),
		PingInterval:             getDuration("PINGPONG_PING_INTERVAL", 60*time.Second),
		SpeedtestInterval:        getDuration("PINGPONG_SPEEDTEST_INTERVAL", 30*time.Minute),
		DNSInterval:              getDuration("PINGPONG_DNS_INTERVAL", 5*time.Minute),
		TracerouteInterval:       getDuration("PINGPONG_TRACEROUTE_INTERVAL", 15*time.Minute),
		AlertDowntimeThreshold:   getDuration("PINGPONG_ALERT_DOWNTIME_THRESHOLD", 60*time.Second),
		AlertPacketLossThreshold: getFloat("PINGPONG_ALERT_PACKET_LOSS_THRESHOLD", 10),
		AlertPingThreshold:       getFloat("PINGPONG_ALERT_PING_THRESHOLD", 100),
		AlertSpeedThreshold:      getFloat("PINGPONG_ALERT_SPEED_THRESHOLD", 50),
		AlertJitterThreshold:     getFloat("PINGPONG_ALERT_JITTER_THRESHOLD", 30),
		AlertCooldown:            getDuration("PINGPONG_ALERT_COOLDOWN", 15*time.Minute),
		AlertMaxRetries:          getInt("PINGPONG_ALERT_MAX_RETRIES", 100),
		AlertRetryInterval:       getDuration("PINGPONG_ALERT_RETRY_INTERVAL", 30*time.Second),
		AppriseURL:               getString("PINGPONG_APPRISE_URL", "http://apprise:8000"),
		AppriseURLs:              getString("PINGPONG_APPRISE_URLS", ""),
		ListenAddr:               getString("PINGPONG_LISTEN_ADDR", ":8080"),
		DataDir:                  getString("PINGPONG_DATA_DIR", "/data"),
	}
}

func getString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getStringSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return fallback
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add configuration package with env var parsing"
```

---

### Task 3: Prometheus Metrics Definitions

**Files:**
- Create: `internal/metrics/metrics.go`
- Create: `internal/metrics/metrics_test.go`

**Step 1: Write the failing test**

```go
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	if m.PingLatency == nil {
		t.Fatal("PingLatency gauge should not be nil")
	}
	if m.PingMin == nil {
		t.Fatal("PingMin gauge should not be nil")
	}
	if m.PingMax == nil {
		t.Fatal("PingMax gauge should not be nil")
	}
	if m.Jitter == nil {
		t.Fatal("Jitter gauge should not be nil")
	}
	if m.PacketLoss == nil {
		t.Fatal("PacketLoss gauge should not be nil")
	}
	if m.DownloadSpeed == nil {
		t.Fatal("DownloadSpeed gauge should not be nil")
	}
	if m.UploadSpeed == nil {
		t.Fatal("UploadSpeed gauge should not be nil")
	}
	if m.SpeedtestLatency == nil {
		t.Fatal("SpeedtestLatency gauge should not be nil")
	}
	if m.DNSResolution == nil {
		t.Fatal("DNSResolution gauge should not be nil")
	}
	if m.ConnectionUp == nil {
		t.Fatal("ConnectionUp gauge should not be nil")
	}
	if m.DowntimeTotal == nil {
		t.Fatal("DowntimeTotal counter should not be nil")
	}
	if m.TracerouteHops == nil {
		t.Fatal("TracerouteHops gauge should not be nil")
	}
	if m.TracerouteHopLatency == nil {
		t.Fatal("TracerouteHopLatency gauge should not be nil")
	}

	// Verify metrics were actually registered by gathering
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	// All gauges start at 0 so they won't appear until set,
	// but the registration itself should succeed without panic
	_ = families
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/metrics/ -v`
Expected: FAIL — `New` not defined

**Step 3: Install dependency and write implementation**

Run: `go get github.com/prometheus/client_golang/prometheus`

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	PingLatency        *prometheus.GaugeVec
	PingMin            *prometheus.GaugeVec
	PingMax            *prometheus.GaugeVec
	Jitter             *prometheus.GaugeVec
	PacketLoss         *prometheus.GaugeVec
	DownloadSpeed      prometheus.Gauge
	UploadSpeed        prometheus.Gauge
	SpeedtestLatency   prometheus.Gauge
	DNSResolution      *prometheus.GaugeVec
	ConnectionUp       prometheus.Gauge
	DowntimeTotal      prometheus.Counter
	TracerouteHops     *prometheus.GaugeVec
	TracerouteHopLatency *prometheus.GaugeVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		PingLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_latency_ms",
			Help: "Average ping latency in milliseconds",
		}, []string{"target"}),
		PingMin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_min_ms",
			Help: "Minimum ping latency in milliseconds",
		}, []string{"target"}),
		PingMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_max_ms",
			Help: "Maximum ping latency in milliseconds",
		}, []string{"target"}),
		Jitter: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_jitter_ms",
			Help: "Ping jitter (standard deviation) in milliseconds",
		}, []string{"target"}),
		PacketLoss: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_packet_loss_percent",
			Help: "Packet loss percentage",
		}, []string{"target"}),
		DownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_download_speed_mbps",
			Help: "Download speed in Mbps",
		}),
		UploadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_upload_speed_mbps",
			Help: "Upload speed in Mbps",
		}),
		SpeedtestLatency: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_speedtest_latency_ms",
			Help: "Latency reported by speed test in milliseconds",
		}),
		DNSResolution: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_dns_resolution_ms",
			Help: "DNS resolution time in milliseconds",
		}, []string{"target"}),
		ConnectionUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_connection_up",
			Help: "Whether the internet connection is up (1) or down (0)",
		}),
		DowntimeTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_downtime_seconds_total",
			Help: "Total downtime in seconds",
		}),
		TracerouteHops: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_traceroute_hops",
			Help: "Number of hops in traceroute",
		}, []string{"target"}),
		TracerouteHopLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_traceroute_hop_latency_ms",
			Help: "Latency per traceroute hop in milliseconds",
		}, []string{"target", "hop"}),
	}

	reg.MustRegister(
		m.PingLatency, m.PingMin, m.PingMax,
		m.Jitter, m.PacketLoss,
		m.DownloadSpeed, m.UploadSpeed, m.SpeedtestLatency,
		m.DNSResolution,
		m.ConnectionUp, m.DowntimeTotal,
		m.TracerouteHops, m.TracerouteHopLatency,
	)

	return m
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/metrics/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/metrics/ go.mod go.sum
git commit -m "feat: add Prometheus metrics definitions"
```

---

### Task 4: Ping Collector

**Files:**
- Create: `internal/collector/ping.go`
- Create: `internal/collector/ping_test.go`

**Step 1: Write the failing test**

Test the `PingResult` struct and the result calculation logic (not actual ICMP — that requires network access).

```go
package collector

import (
	"testing"
	"time"
)

func TestCalculatePingResult(t *testing.T) {
	rtts := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		15 * time.Millisecond,
		25 * time.Millisecond,
		30 * time.Millisecond,
	}

	result := calculatePingResult("1.1.1.1", rtts, 5, 0)

	if result.Target != "1.1.1.1" {
		t.Fatalf("expected target 1.1.1.1, got %s", result.Target)
	}
	if result.AvgMs != 20.0 {
		t.Fatalf("expected avg 20.0, got %f", result.AvgMs)
	}
	if result.MinMs != 10.0 {
		t.Fatalf("expected min 10.0, got %f", result.MinMs)
	}
	if result.MaxMs != 30.0 {
		t.Fatalf("expected max 30.0, got %f", result.MaxMs)
	}
	if result.PacketLoss != 0.0 {
		t.Fatalf("expected 0%% packet loss, got %f", result.PacketLoss)
	}
	// Jitter should be stddev of [10,20,15,25,30] ≈ 7.07
	if result.JitterMs < 7.0 || result.JitterMs > 7.2 {
		t.Fatalf("expected jitter ~7.07, got %f", result.JitterMs)
	}
}

func TestCalculatePingResultWithLoss(t *testing.T) {
	rtts := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
	}

	// 2 received out of 5 sent, 3 lost
	result := calculatePingResult("8.8.8.8", rtts, 5, 3)

	if result.PacketLoss != 60.0 {
		t.Fatalf("expected 60%% packet loss, got %f", result.PacketLoss)
	}
}

func TestCalculatePingResultAllLost(t *testing.T) {
	result := calculatePingResult("8.8.8.8", nil, 5, 5)

	if result.PacketLoss != 100.0 {
		t.Fatalf("expected 100%% packet loss, got %f", result.PacketLoss)
	}
	if result.AvgMs != 0 {
		t.Fatalf("expected avg 0 with no responses, got %f", result.AvgMs)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/ -v -run TestCalculatePing`
Expected: FAIL

**Step 3: Write the implementation**

```go
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type PingResult struct {
	Target     string
	AvgMs      float64
	MinMs      float64
	MaxMs      float64
	JitterMs   float64
	PacketLoss float64
}

func calculatePingResult(target string, rtts []time.Duration, sent int, lost int) PingResult {
	result := PingResult{
		Target:     target,
		PacketLoss: float64(lost) / float64(sent) * 100,
	}

	if len(rtts) == 0 {
		return result
	}

	var sum float64
	min := float64(rtts[0].Microseconds()) / 1000.0
	max := min

	for _, rtt := range rtts {
		ms := float64(rtt.Microseconds()) / 1000.0
		sum += ms
		if ms < min {
			min = ms
		}
		if ms > max {
			max = ms
		}
	}

	avg := sum / float64(len(rtts))
	result.AvgMs = avg
	result.MinMs = min
	result.MaxMs = max

	// Jitter = standard deviation
	var varianceSum float64
	for _, rtt := range rtts {
		ms := float64(rtt.Microseconds()) / 1000.0
		diff := ms - avg
		varianceSum += diff * diff
	}
	result.JitterMs = math.Sqrt(varianceSum / float64(len(rtts)))

	return result
}

type PingCollector struct {
	targets []string
	count   int
}

func NewPingCollector(targets []string, count int) *PingCollector {
	return &PingCollector{targets: targets, count: count}
}

func (p *PingCollector) Collect(ctx context.Context) []PingResult {
	results := make([]PingResult, 0, len(p.targets))
	for _, target := range p.targets {
		result, err := p.ping(ctx, target)
		if err != nil {
			slog.Error("ping failed", "target", target, "error", err)
			results = append(results, PingResult{
				Target:     target,
				PacketLoss: 100,
			})
			continue
		}
		results = append(results, result)
	}
	return results
}

func (p *PingCollector) ping(ctx context.Context, target string) (PingResult, error) {
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return PingResult{}, fmt.Errorf("create pinger: %w", err)
	}

	pinger.Count = p.count
	pinger.Timeout = time.Duration(p.count) * 2 * time.Second
	pinger.SetPrivileged(true)

	err = pinger.RunWithContext(ctx)
	if err != nil {
		return PingResult{}, fmt.Errorf("run ping: %w", err)
	}

	stats := pinger.Statistics()
	return calculatePingResult(
		target,
		stats.Rtts,
		stats.PacketsSent,
		stats.PacketsSent-stats.PacketsRecv,
	), nil
}
```

**Step 4: Install dependency and run tests**

Run: `go get github.com/prometheus-community/pro-bing`
Run: `go test ./internal/collector/ -v -run TestCalculatePing`
Expected: PASS

**Note:** We use `prometheus-community/pro-bing` which is the maintained fork of `go-ping/ping`. It's the same API but actively maintained.

**Step 5: Commit**

```bash
git add internal/collector/ping.go internal/collector/ping_test.go go.mod go.sum
git commit -m "feat: add ping collector with jitter and packet loss"
```

---

### Task 5: DNS Collector

**Files:**
- Create: `internal/collector/dns.go`
- Create: `internal/collector/dns_test.go`

**Step 1: Write the failing test**

```go
package collector

import (
	"context"
	"testing"
)

func TestDNSCollectorResolves(t *testing.T) {
	// This is an integration test — it hits real DNS
	if testing.Short() {
		t.Skip("skipping DNS integration test in short mode")
	}

	c := NewDNSCollector("google.com", "")
	result, err := c.Collect(ctx(t))
	if err != nil {
		t.Fatalf("DNS resolve failed: %v", err)
	}
	if result.Target != "google.com" {
		t.Fatalf("expected target google.com, got %s", result.Target)
	}
	if result.ResolutionMs <= 0 {
		t.Fatalf("expected positive resolution time, got %f", result.ResolutionMs)
	}
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	t.Cleanup(cancel)
	return ctx
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/ -v -run TestDNSCollector`
Expected: FAIL

**Step 3: Write the implementation**

```go
package collector

import (
	"context"
	"fmt"
	"net"
	"time"
)

type DNSResult struct {
	Target       string
	ResolutionMs float64
}

type DNSCollector struct {
	target   string
	resolver *net.Resolver
}

func NewDNSCollector(target, server string) *DNSCollector {
	var resolver *net.Resolver
	if server != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", server+":53")
			},
		}
	} else {
		resolver = net.DefaultResolver
	}
	return &DNSCollector{target: target, resolver: resolver}
}

func (d *DNSCollector) Collect(ctx context.Context) (DNSResult, error) {
	start := time.Now()
	_, err := d.resolver.LookupHost(ctx, d.target)
	elapsed := time.Since(start)

	if err != nil {
		return DNSResult{}, fmt.Errorf("dns lookup %s: %w", d.target, err)
	}

	return DNSResult{
		Target:       d.target,
		ResolutionMs: float64(elapsed.Microseconds()) / 1000.0,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/collector/ -v -run TestDNSCollector`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/collector/dns.go internal/collector/dns_test.go
git commit -m "feat: add DNS resolution collector"
```

---

### Task 6: Speedtest Collector

**Files:**
- Create: `internal/collector/speedtest.go`
- Create: `internal/collector/speedtest_test.go`

**Step 1: Write the failing test**

We test the JSON parsing logic, not the actual speedtest execution (which requires the CLI binary).

```go
package collector

import (
	"testing"
)

func TestParseSpeedtestOutput(t *testing.T) {
	// Sample JSON output from `speedtest --format=json`
	output := `{
		"type": "result",
		"timestamp": "2026-02-28T12:00:00Z",
		"ping": {"jitter": 1.234, "latency": 12.345, "low": 10.0, "high": 15.0},
		"download": {"bandwidth": 12500000, "bytes": 125000000, "elapsed": 10000},
		"upload": {"bandwidth": 6250000, "bytes": 62500000, "elapsed": 10000},
		"isp": "Test ISP",
		"interface": {"internalIp": "192.168.1.100", "name": "eth0", "macAddr": "00:00:00:00:00:00", "isVpn": false, "externalIp": "1.2.3.4"},
		"server": {"id": 1234, "host": "speedtest.example.com", "port": 8080, "name": "Test Server", "location": "Test City", "country": "US", "ip": "5.6.7.8"},
		"result": {"id": "abc-123", "url": "https://www.speedtest.net/result/abc-123", "persisted": true}
	}`

	result, err := parseSpeedtestOutput([]byte(output))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// bandwidth is in bytes/sec, convert to Mbps: 12500000 * 8 / 1000000 = 100
	if result.DownloadMbps != 100.0 {
		t.Fatalf("expected download 100 Mbps, got %f", result.DownloadMbps)
	}
	// 6250000 * 8 / 1000000 = 50
	if result.UploadMbps != 50.0 {
		t.Fatalf("expected upload 50 Mbps, got %f", result.UploadMbps)
	}
	if result.LatencyMs != 12.345 {
		t.Fatalf("expected latency 12.345, got %f", result.LatencyMs)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/ -v -run TestParseSpeedtest`
Expected: FAIL

**Step 3: Write the implementation**

```go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
)

type SpeedtestResult struct {
	DownloadMbps float64
	UploadMbps   float64
	LatencyMs    float64
}

type speedtestJSON struct {
	Ping struct {
		Latency float64 `json:"latency"`
	} `json:"ping"`
	Download struct {
		Bandwidth int64 `json:"bandwidth"` // bytes per second
	} `json:"download"`
	Upload struct {
		Bandwidth int64 `json:"bandwidth"` // bytes per second
	} `json:"upload"`
}

func parseSpeedtestOutput(data []byte) (SpeedtestResult, error) {
	var raw speedtestJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return SpeedtestResult{}, fmt.Errorf("parse speedtest json: %w", err)
	}

	return SpeedtestResult{
		DownloadMbps: float64(raw.Download.Bandwidth) * 8 / 1_000_000,
		UploadMbps:   float64(raw.Upload.Bandwidth) * 8 / 1_000_000,
		LatencyMs:    raw.Ping.Latency,
	}, nil
}

type SpeedtestCollector struct{}

func NewSpeedtestCollector() *SpeedtestCollector {
	return &SpeedtestCollector{}
}

func (s *SpeedtestCollector) Collect(ctx context.Context) (SpeedtestResult, error) {
	slog.Info("running speed test...")
	cmd := exec.CommandContext(ctx, "speedtest", "--format=json", "--accept-license")
	output, err := cmd.Output()
	if err != nil {
		return SpeedtestResult{}, fmt.Errorf("run speedtest: %w", err)
	}

	result, err := parseSpeedtestOutput(output)
	if err != nil {
		return SpeedtestResult{}, err
	}

	slog.Info("speed test complete",
		"download_mbps", fmt.Sprintf("%.1f", result.DownloadMbps),
		"upload_mbps", fmt.Sprintf("%.1f", result.UploadMbps),
		"latency_ms", fmt.Sprintf("%.1f", result.LatencyMs),
	)
	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/collector/ -v -run TestParseSpeedtest`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/collector/speedtest.go internal/collector/speedtest_test.go
git commit -m "feat: add speedtest collector with Ookla CLI wrapper"
```

---

### Task 7: Traceroute Collector

**Files:**
- Create: `internal/collector/traceroute.go`
- Create: `internal/collector/traceroute_test.go`

**Step 1: Write the failing test**

Test the output parsing logic.

```go
package collector

import (
	"testing"
)

func TestParseTracerouteOutput(t *testing.T) {
	// Simulated traceroute output
	output := `traceroute to 1.1.1.1 (1.1.1.1), 30 hops max, 60 byte packets
 1  gateway (192.168.1.1)  1.234 ms  1.345 ms  1.456 ms
 2  10.0.0.1 (10.0.0.1)  5.678 ms  5.789 ms  5.890 ms
 3  * * *
 4  one.one.one.one (1.1.1.1)  12.345 ms  12.456 ms  12.567 ms`

	result := parseTracerouteOutput("1.1.1.1", output)

	if result.Target != "1.1.1.1" {
		t.Fatalf("expected target 1.1.1.1, got %s", result.Target)
	}
	if result.HopCount != 4 {
		t.Fatalf("expected 4 hops, got %d", result.HopCount)
	}
	if len(result.Hops) != 4 {
		t.Fatalf("expected 4 hop entries, got %d", len(result.Hops))
	}

	// Check first hop
	if result.Hops[0].Number != 1 {
		t.Fatalf("expected hop 1, got %d", result.Hops[0].Number)
	}
	if result.Hops[0].Address != "192.168.1.1" {
		t.Fatalf("expected address 192.168.1.1, got %s", result.Hops[0].Address)
	}
	// Average of 1.234, 1.345, 1.456 ≈ 1.345
	if result.Hops[0].LatencyMs < 1.3 || result.Hops[0].LatencyMs > 1.4 {
		t.Fatalf("expected hop 1 latency ~1.345, got %f", result.Hops[0].LatencyMs)
	}

	// Check timeout hop
	if result.Hops[2].Address != "*" {
		t.Fatalf("expected timeout hop address *, got %s", result.Hops[2].Address)
	}
	if result.Hops[2].LatencyMs != 0 {
		t.Fatalf("expected timeout hop latency 0, got %f", result.Hops[2].LatencyMs)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/collector/ -v -run TestParseTraceroute`
Expected: FAIL

**Step 3: Write the implementation**

```go
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type TracerouteHop struct {
	Number    int
	Address   string
	LatencyMs float64
}

type TracerouteResult struct {
	Target   string
	HopCount int
	Hops     []TracerouteHop
}

var (
	hopLineRe  = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
	latencyRe  = regexp.MustCompile(`([\d.]+)\s*ms`)
	addressRe  = regexp.MustCompile(`\(([\d.]+)\)`)
)

func parseTracerouteOutput(target, output string) TracerouteResult {
	lines := strings.Split(output, "\n")
	result := TracerouteResult{Target: target}

	for _, line := range lines {
		match := hopLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		hopNum, _ := strconv.Atoi(match[1])
		rest := match[2]

		hop := TracerouteHop{Number: hopNum}

		// Check for timeout
		if strings.TrimSpace(rest) == "* * *" {
			hop.Address = "*"
			result.Hops = append(result.Hops, hop)
			continue
		}

		// Extract address from parentheses
		addrMatch := addressRe.FindStringSubmatch(rest)
		if addrMatch != nil {
			hop.Address = addrMatch[1]
		}

		// Extract latencies and average them
		latMatches := latencyRe.FindAllStringSubmatch(rest, -1)
		if len(latMatches) > 0 {
			var sum float64
			for _, m := range latMatches {
				val, _ := strconv.ParseFloat(m[1], 64)
				sum += val
			}
			hop.LatencyMs = sum / float64(len(latMatches))
		}

		result.Hops = append(result.Hops, hop)
	}

	result.HopCount = len(result.Hops)
	return result
}

type TracerouteCollector struct {
	target string
}

func NewTracerouteCollector(target string) *TracerouteCollector {
	return &TracerouteCollector{target: target}
}

func (tr *TracerouteCollector) Collect(ctx context.Context) (TracerouteResult, error) {
	slog.Info("running traceroute", "target", tr.target)
	cmd := exec.CommandContext(ctx, "traceroute", "-n", "-w", "2", tr.target)
	output, err := cmd.Output()
	if err != nil {
		// traceroute may exit non-zero even on partial success
		if len(output) == 0 {
			return TracerouteResult{}, fmt.Errorf("run traceroute: %w", err)
		}
	}

	result := parseTracerouteOutput(tr.target, string(output))
	slog.Info("traceroute complete", "target", tr.target, "hops", result.HopCount)
	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/collector/ -v -run TestParseTraceroute`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/collector/traceroute.go internal/collector/traceroute_test.go
git commit -m "feat: add traceroute collector with output parsing"
```

---

### Task 8: Alert Queue (SQLite)

**Files:**
- Create: `internal/alerter/queue.go`
- Create: `internal/alerter/queue_test.go`

**Step 1: Write the failing test**

```go
package alerter

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueEnqueueAndPending(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	err = q.Enqueue("downtime", "Connection Down", "Internet has been down for 2 minutes")
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	pending, err := q.Pending()
	if err != nil {
		t.Fatalf("failed to get pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].AlertType != "downtime" {
		t.Fatalf("expected alert type downtime, got %s", pending[0].AlertType)
	}
	if pending[0].Title != "Connection Down" {
		t.Fatalf("expected title 'Connection Down', got %s", pending[0].Title)
	}
	if pending[0].Status != "pending" {
		t.Fatalf("expected status pending, got %s", pending[0].Status)
	}
}

func TestQueueMarkSent(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("latency", "High Latency", "Ping is 200ms")

	pending, _ := q.Pending()
	err = q.MarkSent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark sent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after mark sent, got %d", len(pending))
	}
}

func TestQueueIncrementRetry(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("packet_loss", "Packet Loss", "50% packet loss")
	pending, _ := q.Pending()

	err = q.IncrementRetry(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to increment retry: %v", err)
	}

	pending, _ = q.Pending()
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", pending[0].RetryCount)
	}
}

func TestQueueMarkFailedPermanent(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	q.Enqueue("speed", "Slow Speed", "Download is 5 Mbps")
	pending, _ := q.Pending()

	err = q.MarkFailedPermanent(pending[0].ID)
	if err != nil {
		t.Fatalf("failed to mark failed permanent: %v", err)
	}

	pending, _ = q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after permanent fail, got %d", len(pending))
	}
}

func TestQueueLastSentTime(t *testing.T) {
	dir := t.TempDir()
	q, err := NewQueue(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer q.Close()

	// No alerts sent yet
	_, found, err := q.LastSentTime("downtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no last sent time for new queue")
	}

	// Send an alert
	q.Enqueue("downtime", "Down", "down")
	pending, _ := q.Pending()
	q.MarkSent(pending[0].ID)

	lastSent, found, err := q.LastSentTime("downtime")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected to find last sent time")
	}
	if time.Since(lastSent) > 5*time.Second {
		t.Fatalf("last sent time should be recent, got %v", lastSent)
	}
}

func TestQueuePersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open, enqueue, close
	q1, _ := NewQueue(dbPath)
	q1.Enqueue("downtime", "Down", "Internet down")
	q1.Close()

	// Re-open, verify alert is still there
	q2, _ := NewQueue(dbPath)
	defer q2.Close()
	pending, _ := q2.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert after reopen, got %d", len(pending))
	}
}

func init() {
	// Ensure test temp dirs don't conflict
	os.MkdirAll(os.TempDir(), 0755)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/alerter/ -v -run TestQueue`
Expected: FAIL

**Step 3: Install dependency and write implementation**

Run: `go get modernc.org/sqlite`
Run: `go get github.com/jmoiron/sqlx`

```go
package alerter

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type Alert struct {
	ID         int64     `db:"id"`
	CreatedAt  time.Time `db:"created_at"`
	SentAt     *time.Time `db:"sent_at"`
	Status     string    `db:"status"`
	AlertType  string    `db:"alert_type"`
	Title      string    `db:"title"`
	Body       string    `db:"body"`
	RetryCount int       `db:"retry_count"`
}

type Queue struct {
	db *sqlx.DB
}

func NewQueue(dbPath string) (*Queue, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	db.MustExec("PRAGMA journal_mode=WAL")

	// Create table
	db.MustExec(`CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		sent_at TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'pending',
		alert_type TEXT NOT NULL,
		title TEXT NOT NULL,
		body TEXT NOT NULL,
		retry_count INTEGER NOT NULL DEFAULT 0
	)`)

	return &Queue{db: db}, nil
}

func (q *Queue) Close() error {
	return q.db.Close()
}

func (q *Queue) Enqueue(alertType, title, body string) error {
	_, err := q.db.Exec(
		"INSERT INTO alerts (alert_type, title, body) VALUES (?, ?, ?)",
		alertType, title, body,
	)
	return err
}

func (q *Queue) Pending() ([]Alert, error) {
	var alerts []Alert
	err := q.db.Select(&alerts,
		"SELECT * FROM alerts WHERE status = 'pending' ORDER BY created_at ASC",
	)
	return alerts, err
}

func (q *Queue) MarkSent(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET status = 'sent', sent_at = CURRENT_TIMESTAMP WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) IncrementRetry(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET retry_count = retry_count + 1 WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) MarkFailedPermanent(id int64) error {
	_, err := q.db.Exec(
		"UPDATE alerts SET status = 'failed_permanent' WHERE id = ?",
		id,
	)
	return err
}

func (q *Queue) LastSentTime(alertType string) (time.Time, bool, error) {
	var sentAt *time.Time
	err := q.db.Get(&sentAt,
		"SELECT sent_at FROM alerts WHERE alert_type = ? AND status = 'sent' ORDER BY sent_at DESC LIMIT 1",
		alertType,
	)
	if err != nil || sentAt == nil {
		return time.Time{}, false, nil
	}
	return *sentAt, true, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/alerter/ -v -run TestQueue`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/alerter/queue.go internal/alerter/queue_test.go go.mod go.sum
git commit -m "feat: add SQLite-backed persistent alert queue"
```

---

### Task 9: Apprise Client

**Files:**
- Create: `internal/alerter/apprise.go`
- Create: `internal/alerter/apprise_test.go`

**Step 1: Write the failing test**

Use a local HTTP test server to mock the Apprise API.

```go
package alerter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppriseSendSuccess(t *testing.T) {
	var received appriseRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/notify" {
			t.Fatalf("expected /notify path, got %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAppriseClient(server.URL, "discord://webhook/token")
	err := client.Send("Test Title", "Test Body")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if received.Title != "Test Title" {
		t.Fatalf("expected title 'Test Title', got %s", received.Title)
	}
	if received.Body != "Test Body" {
		t.Fatalf("expected body 'Test Body', got %s", received.Body)
	}
	if received.URLs != "discord://webhook/token" {
		t.Fatalf("expected urls to match, got %s", received.URLs)
	}
}

func TestAppriseSendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewAppriseClient(server.URL, "discord://webhook/token")
	err := client.Send("Title", "Body")
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestAppriseSendConnectionError(t *testing.T) {
	// Point at a non-existent server
	client := NewAppriseClient("http://127.0.0.1:1", "discord://webhook/token")
	err := client.Send("Title", "Body")
	if err == nil {
		t.Fatal("expected error on connection failure")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/alerter/ -v -run TestApprise`
Expected: FAIL

**Step 3: Write the implementation**

```go
package alerter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AppriseClient struct {
	baseURL string
	urls    string
	client  *http.Client
}

type appriseRequest struct {
	URLs  string `json:"urls"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Type  string `json:"type"`
}

func NewAppriseClient(baseURL, urls string) *AppriseClient {
	return &AppriseClient{
		baseURL: baseURL,
		urls:    urls,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *AppriseClient) Send(title, body string) error {
	payload := appriseRequest{
		URLs:  a.urls,
		Title: title,
		Body:  body,
		Type:  "warning",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal apprise request: %w", err)
	}

	resp, err := a.client.Post(
		a.baseURL+"/notify",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("send apprise notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apprise returned status %d", resp.StatusCode)
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/alerter/ -v -run TestApprise`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/alerter/apprise.go internal/alerter/apprise_test.go
git commit -m "feat: add Apprise API notification client"
```

---

### Task 10: Alert Engine

**Files:**
- Create: `internal/alerter/engine.go`
- Create: `internal/alerter/engine_test.go`

**Step 1: Write the failing test**

```go
package alerter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
)

func TestEngineEvaluatePacketLoss(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	// Apprise client pointed at nothing — we only test enqueuing
	engine := NewEngine(q, nil, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:           1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 15.0, AvgMs: 20},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for high packet loss, got %d", len(pending))
	}
	if pending[0].AlertType != "packet_loss" {
		t.Fatalf("expected alert type packet_loss, got %s", pending[0].AlertType)
	}
}

func TestEngineEvaluateNoAlert(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertPingThreshold:       100,
		AlertCooldown:            1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 5.0, AvgMs: 20},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts for normal values, got %d", len(pending))
	}
}

func TestEngineCooldown(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            5 * time.Minute,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0, AvgMs: 20},
	}

	// First evaluation — should alert
	engine.EvaluatePing(results)
	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert on first eval, got %d", len(pending))
	}

	// Second evaluation immediately — should NOT alert (cooldown)
	engine.EvaluatePing(results)
	pending, _ = q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected still 1 alert (cooldown active), got %d", len(pending))
	}
}

func TestEngineEvaluateSpeed(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
		AlertSpeedThreshold: 50,
		AlertCooldown:       1 * time.Second,
	})

	result := collector.SpeedtestResult{
		DownloadMbps: 25.0,
		UploadMbps:   10.0,
	}

	engine.EvaluateSpeed(result)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for slow speed, got %d", len(pending))
	}
	if pending[0].AlertType != "speed" {
		t.Fatalf("expected alert type speed, got %s", pending[0].AlertType)
	}
}

func TestEngineDisabledThresholds(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	// All thresholds set to 0 = disabled
	engine := NewEngine(q, nil, &config.Config{
		AlertPacketLossThreshold: 0,
		AlertPingThreshold:       0,
		AlertSpeedThreshold:      0,
		AlertJitterThreshold:     0,
		AlertCooldown:            1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 100, AvgMs: 999, JitterMs: 999},
	}
	engine.EvaluatePing(results)

	speed := collector.SpeedtestResult{DownloadMbps: 0.1}
	engine.EvaluateSpeed(speed)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts with disabled thresholds, got %d", len(pending))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/alerter/ -v -run TestEngine`
Expected: FAIL

**Step 3: Write the implementation**

```go
package alerter

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
)

type Engine struct {
	queue    *Queue
	apprise *AppriseClient
	cfg     *config.Config

	mu            sync.Mutex
	lastAlertTime map[string]time.Time // alert_type -> last fired time
}

func NewEngine(queue *Queue, apprise *AppriseClient, cfg *config.Config) *Engine {
	return &Engine{
		queue:         queue,
		apprise:       apprise,
		cfg:           cfg,
		lastAlertTime: make(map[string]time.Time),
	}
}

func (e *Engine) EvaluatePing(results []collector.PingResult) {
	for _, r := range results {
		// Packet loss check
		if e.cfg.AlertPacketLossThreshold > 0 && r.PacketLoss >= e.cfg.AlertPacketLossThreshold {
			e.fireAlert("packet_loss",
				fmt.Sprintf("High Packet Loss: %s", r.Target),
				fmt.Sprintf("Packet loss to %s is %.1f%% (threshold: %.1f%%)",
					r.Target, r.PacketLoss, e.cfg.AlertPacketLossThreshold),
			)
		}

		// Latency check
		if e.cfg.AlertPingThreshold > 0 && r.AvgMs >= e.cfg.AlertPingThreshold {
			e.fireAlert("latency",
				fmt.Sprintf("High Latency: %s", r.Target),
				fmt.Sprintf("Ping to %s is %.1fms (threshold: %.1fms)",
					r.Target, r.AvgMs, e.cfg.AlertPingThreshold),
			)
		}

		// Jitter check
		if e.cfg.AlertJitterThreshold > 0 && r.JitterMs >= e.cfg.AlertJitterThreshold {
			e.fireAlert("jitter",
				fmt.Sprintf("High Jitter: %s", r.Target),
				fmt.Sprintf("Jitter to %s is %.1fms (threshold: %.1fms)",
					r.Target, r.JitterMs, e.cfg.AlertJitterThreshold),
			)
		}
	}
}

func (e *Engine) EvaluateSpeed(result collector.SpeedtestResult) {
	if e.cfg.AlertSpeedThreshold > 0 && result.DownloadMbps < e.cfg.AlertSpeedThreshold {
		e.fireAlert("speed",
			"Slow Download Speed",
			fmt.Sprintf("Download speed is %.1f Mbps (threshold: %.1f Mbps)",
				result.DownloadMbps, e.cfg.AlertSpeedThreshold),
		)
	}
}

func (e *Engine) EvaluateDowntime(isDown bool, downSince time.Time) {
	if !isDown || e.cfg.AlertDowntimeThreshold == 0 {
		return
	}

	downtime := time.Since(downSince)
	if downtime >= e.cfg.AlertDowntimeThreshold {
		e.fireAlert("downtime",
			"Internet Connection Down",
			fmt.Sprintf("Connection has been down for %s (threshold: %s)",
				downtime.Round(time.Second), e.cfg.AlertDowntimeThreshold),
		)
	}
}

func (e *Engine) fireAlert(alertType, title, body string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check cooldown
	if last, ok := e.lastAlertTime[alertType]; ok {
		if time.Since(last) < e.cfg.AlertCooldown {
			slog.Debug("alert cooldown active", "type", alertType)
			return
		}
	}

	if err := e.queue.Enqueue(alertType, title, body); err != nil {
		slog.Error("failed to enqueue alert", "type", alertType, "error", err)
		return
	}

	e.lastAlertTime[alertType] = time.Now()
	slog.Warn("alert fired", "type", alertType, "title", title)
}

// ProcessQueue attempts to send all pending alerts via Apprise.
// Called periodically by the retry loop.
func (e *Engine) ProcessQueue() {
	if e.apprise == nil {
		return
	}

	pending, err := e.queue.Pending()
	if err != nil {
		slog.Error("failed to get pending alerts", "error", err)
		return
	}

	for _, alert := range pending {
		err := e.apprise.Send(alert.Title, alert.Body)
		if err != nil {
			slog.Error("failed to send alert", "id", alert.ID, "error", err)
			e.queue.IncrementRetry(alert.ID)

			if alert.RetryCount+1 >= e.cfg.AlertMaxRetries {
				slog.Error("alert exceeded max retries, marking permanent failure",
					"id", alert.ID, "type", alert.AlertType)
				e.queue.MarkFailedPermanent(alert.ID)
			}
			continue
		}

		e.queue.MarkSent(alert.ID)
		slog.Info("alert sent successfully", "id", alert.ID, "type", alert.AlertType)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/alerter/ -v -run TestEngine`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/alerter/engine.go internal/alerter/engine_test.go
git commit -m "feat: add alert engine with threshold evaluation and cooldown"
```

---

### Task 11: Main Entry Point — Wiring Everything Together

**Files:**
- Modify: `cmd/pingpong/main.go`

**Step 1: Write the full main.go**

This wires together all the components: config, metrics, collectors, alert engine, HTTP server, and measurement loops.

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/bcraig/pingpong/internal/alerter"
	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
	"github.com/bcraig/pingpong/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	slog.Info("pingpong starting",
		"ping_targets", cfg.PingTargets,
		"ping_interval", cfg.PingInterval,
		"speedtest_interval", cfg.SpeedtestInterval,
		"dns_interval", cfg.DNSInterval,
		"traceroute_interval", cfg.TracerouteInterval,
	)

	// Prometheus registry
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	// Collectors
	pingCollector := collector.NewPingCollector(cfg.PingTargets, cfg.PingCount)
	speedCollector := collector.NewSpeedtestCollector()
	dnsCollector := collector.NewDNSCollector(cfg.DNSTarget, cfg.DNSServer)
	traceCollector := collector.NewTracerouteCollector(cfg.TracerouteTarget)

	// Alert queue
	os.MkdirAll(cfg.DataDir, 0755)
	queue, err := alerter.NewQueue(filepath.Join(cfg.DataDir, "alerts.db"))
	if err != nil {
		slog.Error("failed to open alert queue", "error", err)
		os.Exit(1)
	}
	defer queue.Close()

	// Apprise client (nil if not configured)
	var appriseClient *alerter.AppriseClient
	if cfg.AppriseURLs != "" {
		appriseClient = alerter.NewAppriseClient(cfg.AppriseURL, cfg.AppriseURLs)
		slog.Info("apprise notifications enabled")
	} else {
		slog.Warn("apprise not configured — notifications disabled")
	}

	engine := alerter.NewEngine(queue, appriseClient, cfg)

	// HTTP server for /metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// Connection state tracking
	var (
		connMu    sync.Mutex
		connDown  bool
		downSince time.Time
	)

	// Ping loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PingInterval)
		defer ticker.Stop()

		// Run immediately on start
		runPing := func() {
			results := pingCollector.Collect(ctx)
			for _, r := range results {
				m.PingLatency.WithLabelValues(r.Target).Set(r.AvgMs)
				m.PingMin.WithLabelValues(r.Target).Set(r.MinMs)
				m.PingMax.WithLabelValues(r.Target).Set(r.MaxMs)
				m.Jitter.WithLabelValues(r.Target).Set(r.JitterMs)
				m.PacketLoss.WithLabelValues(r.Target).Set(r.PacketLoss)
			}

			// Connection status: down if ALL targets have 100% loss
			allDown := len(results) > 0
			for _, r := range results {
				if r.PacketLoss < 100 {
					allDown = false
					break
				}
			}

			connMu.Lock()
			if allDown && !connDown {
				connDown = true
				downSince = time.Now()
				m.ConnectionUp.Set(0)
				slog.Warn("connection detected as DOWN")
			} else if !allDown && connDown {
				downtime := time.Since(downSince)
				m.DowntimeTotal.Add(downtime.Seconds())
				connDown = false
				m.ConnectionUp.Set(1)
				slog.Info("connection restored", "downtime", downtime.Round(time.Second))
			} else if !allDown {
				m.ConnectionUp.Set(1)
			}
			isDown := connDown
			ds := downSince
			connMu.Unlock()

			engine.EvaluatePing(results)
			if isDown {
				engine.EvaluateDowntime(true, ds)
			}
		}

		runPing()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runPing()
			}
		}
	}()

	// Speedtest loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.SpeedtestInterval)
		defer ticker.Stop()

		runSpeed := func() {
			result, err := speedCollector.Collect(ctx)
			if err != nil {
				slog.Error("speedtest failed", "error", err)
				return
			}
			m.DownloadSpeed.Set(result.DownloadMbps)
			m.UploadSpeed.Set(result.UploadMbps)
			m.SpeedtestLatency.Set(result.LatencyMs)

			engine.EvaluateSpeed(result)
		}

		runSpeed()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSpeed()
			}
		}
	}()

	// DNS loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.DNSInterval)
		defer ticker.Stop()

		runDNS := func() {
			result, err := dnsCollector.Collect(ctx)
			if err != nil {
				slog.Error("DNS check failed", "error", err)
				return
			}
			m.DNSResolution.WithLabelValues(result.Target).Set(result.ResolutionMs)
		}

		runDNS()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDNS()
			}
		}
	}()

	// Traceroute loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.TracerouteInterval)
		defer ticker.Stop()

		runTrace := func() {
			result, err := traceCollector.Collect(ctx)
			if err != nil {
				slog.Error("traceroute failed", "error", err)
				return
			}
			m.TracerouteHops.WithLabelValues(result.Target).Set(float64(result.HopCount))
			for _, hop := range result.Hops {
				if hop.Address != "*" {
					m.TracerouteHopLatency.WithLabelValues(
						result.Target,
						fmt.Sprintf("%d_%s", hop.Number, hop.Address),
					).Set(hop.LatencyMs)
				}
			}
		}

		runTrace()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runTrace()
			}
		}
	}()

	// Alert retry loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.AlertRetryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				engine.ProcessQueue()
			}
		}
	}()

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("HTTP server starting", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	wg.Wait()
	slog.Info("pingpong stopped")
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/pingpong/`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add cmd/pingpong/main.go
git commit -m "feat: wire all components together in main entry point"
```

---

### Task 12: Dockerfile

**Files:**
- Create: `Dockerfile`

**Step 1: Write the Dockerfile**

```dockerfile
FROM golang:1.23-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pingpong ./cmd/pingpong/

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        traceroute \
        iputils-ping \
    && rm -rf /var/lib/apt/lists/*

# Install Ookla Speedtest CLI
RUN curl -s https://packagecloud.io/install/repositories/ookla/speedtest-cli/script.deb.sh | bash && \
    apt-get install -y speedtest

COPY --from=builder /pingpong /usr/local/bin/pingpong

RUN mkdir -p /data

ENTRYPOINT ["pingpong"]
```

**Step 2: Verify it builds**

Run: `docker build -t pingpong:dev .`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add Dockerfile
git commit -m "feat: add multi-stage Dockerfile with speedtest CLI"
```

---

### Task 13: Prometheus Configuration

**Files:**
- Create: `prometheus.yml`

**Step 1: Write the config**

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "pingpong"
    static_configs:
      - targets: ["pingpong:8080"]
    scrape_interval: 30s
```

**Step 2: Commit**

```bash
git add prometheus.yml
git commit -m "feat: add Prometheus scrape configuration"
```

---

### Task 14: Grafana Provisioning

**Files:**
- Create: `grafana/provisioning/datasources/prometheus.yml`
- Create: `grafana/provisioning/dashboards/dashboard.yml`
- Create: `grafana/dashboards/pingpong.json`

**Step 1: Create datasource provisioning**

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
```

**Step 2: Create dashboard provisioning config**

```yaml
apiVersion: 1

providers:
  - name: "PingPong"
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: false
```

**Step 3: Create the Grafana dashboard JSON**

This is a large JSON file. Create it with panels for:
- Connection Status (stat panel showing up/down)
- Download Speed (time series)
- Upload Speed (time series)
- Ping Latency (time series with multiple targets)
- Jitter (time series)
- Packet Loss (bar gauge)
- DNS Resolution Time (time series)
- Traceroute Hops (table)
- Uptime percentages (stat panels for 24h, 7d)

The dashboard JSON should use Prometheus as the datasource and reference the metric names defined in Task 3.

**Reference doc:** [Grafana Dashboard JSON Model](https://grafana.com/docs/grafana/latest/dashboards/json-model/) — the JSON structure uses `panels[]` with each panel having a `type`, `targets[]` (Prometheus queries), `gridPos`, and `fieldConfig`.

**Step 4: Commit**

```bash
git add grafana/
git commit -m "feat: add Grafana provisioning and pre-built dashboard"
```

---

### Task 15: Docker Compose

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`

**Step 1: Write docker-compose.yml**

```yaml
services:
  pingpong:
    build: .
    container_name: pingpong
    restart: unless-stopped
    cap_add:
      - NET_RAW
    env_file:
      - .env
    volumes:
      - pingpong-data:/data
    ports:
      - "8080:8080"
    depends_on:
      - apprise

  prometheus:
    image: prom/prometheus:latest
    container_name: pingpong-prometheus
    restart: unless-stopped
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"

  grafana:
    image: grafana/grafana:latest
    container_name: pingpong-grafana
    restart: unless-stopped
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
      - ./grafana/dashboards:/var/lib/grafana/dashboards:ro
      - grafana-data:/var/lib/grafana
    ports:
      - "3000:3000"
    depends_on:
      - prometheus

  apprise:
    image: caronc/apprise:latest
    container_name: pingpong-apprise
    restart: unless-stopped
    ports:
      - "8000:8000"

volumes:
  pingpong-data:
  prometheus-data:
  grafana-data:
```

**Step 2: Write .env.example**

```env
# PingPong Configuration
# Copy this file to .env and customize as needed.

# === Measurement Targets ===
# Comma-separated list of IP addresses or hostnames to ping
PINGPONG_PING_TARGETS=1.1.1.1,8.8.8.8,208.67.222.222
# Number of pings per check
PINGPONG_PING_COUNT=10
# Domain to resolve for DNS check
PINGPONG_DNS_TARGET=google.com
# DNS server to use (empty = system default)
PINGPONG_DNS_SERVER=
# Target for traceroute
PINGPONG_TRACEROUTE_TARGET=1.1.1.1

# === Measurement Intervals ===
# How often to run each check (Go duration format: 30s, 5m, 1h)
PINGPONG_PING_INTERVAL=60s
PINGPONG_SPEEDTEST_INTERVAL=30m
PINGPONG_DNS_INTERVAL=5m
PINGPONG_TRACEROUTE_INTERVAL=15m

# === Alert Thresholds ===
# Set to 0 to disable any alert type
# Minimum downtime before alerting
PINGPONG_ALERT_DOWNTIME_THRESHOLD=60s
# Packet loss percentage to trigger alert
PINGPONG_ALERT_PACKET_LOSS_THRESHOLD=10
# Ping latency in ms to trigger alert
PINGPONG_ALERT_PING_THRESHOLD=100
# Download speed in Mbps below which to alert
PINGPONG_ALERT_SPEED_THRESHOLD=50
# Jitter in ms to trigger alert
PINGPONG_ALERT_JITTER_THRESHOLD=30
# Minimum time between repeated alerts of the same type
PINGPONG_ALERT_COOLDOWN=15m
# Maximum send retries before giving up on an alert
PINGPONG_ALERT_MAX_RETRIES=100
# How often to retry failed alerts
PINGPONG_ALERT_RETRY_INTERVAL=30s

# === Notifications ===
# Apprise API server URL (default works with the bundled container)
PINGPONG_APPRISE_URL=http://apprise:8000
# Apprise notification URL(s) — see https://github.com/caronc/apprise/wiki
# Examples:
#   Discord:  discord://webhook_id/webhook_token
#   Slack:    slack://token_a/token_b/token_c
#   Email:    mailto://user:pass@gmail.com
#   Telegram: tgram://bot_token/chat_id
#   Ntfy:     ntfy://topic
PINGPONG_APPRISE_URLS=

# === Server ===
PINGPONG_LISTEN_ADDR=:8080

# === Data ===
PINGPONG_DATA_DIR=/data
```

**Step 3: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "feat: add Docker Compose stack and .env.example"
```

---

### Task 16: Run Full Stack and Verify

**Step 1: Copy .env.example to .env**

```bash
cp .env.example .env
```

**Step 2: Build and start the stack**

Run: `docker compose up --build -d`
Expected: All 4 containers start

**Step 3: Verify PingPong is running**

Run: `curl http://localhost:8080/health`
Expected: `ok`

**Step 4: Verify metrics endpoint**

Run: `curl -s http://localhost:8080/metrics | head -20`
Expected: Prometheus format metrics with `pingpong_` prefix

**Step 5: Verify Prometheus is scraping**

Run: `curl -s http://localhost:9090/api/v1/targets | python3 -m json.tool`
Expected: Shows pingpong target with state "up"

**Step 6: Verify Grafana dashboard**

Open `http://localhost:3000` in browser (admin/admin). Verify the PingPong dashboard is pre-loaded and showing data.

**Step 7: Commit any final adjustments**

```bash
git add -A
git commit -m "chore: final adjustments from integration testing"
```

---

### Task 17: README

**Files:**
- Modify: `README.md`

**Step 1: Write the README**

Include:
- Project description (what it does, what it monitors)
- Quick start (3 steps: clone, copy .env, docker compose up)
- Configuration reference (link to .env.example)
- Architecture diagram (from design doc)
- Screenshots placeholder
- Notification setup (link to Apprise wiki for URL formats)
- Accessing Grafana (default URL and credentials)

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with setup instructions"
```
