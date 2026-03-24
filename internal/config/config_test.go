package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMain ensures no test accidentally reads a local .env file.
func TestMain(m *testing.M) {
	os.Setenv("PINGPONG_ENV_FILE", filepath.Join(os.TempDir(), "pingpong-test-no-such-env"))
	os.Exit(m.Run())
}

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{
		"PINGPONG_PING_TARGETS",
		"PINGPONG_PING_COUNT",
		"PINGPONG_PING_INTERVAL",
		"PINGPONG_SPEEDTEST_INTERVAL",
		"PINGPONG_DNS_TARGET",
		"PINGPONG_DNS_TARGETS",
		"PINGPONG_DNS_SERVER",
		"PINGPONG_DNS_SERVERS",
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
		"PINGPONG_SPEEDTEST_SERVER_ID",
		"PINGPONG_LISTEN_ADDR",
		"PINGPONG_DATA_DIR",
	} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if len(cfg.PingTargets) != 3 {
		t.Errorf("expected 3 default ping targets, got %d", len(cfg.PingTargets))
	}
	if cfg.PingTargets[0] != "1.1.1.1" {
		t.Errorf("expected first target 1.1.1.1, got %s", cfg.PingTargets[0])
	}
	if cfg.PingCount != 25 {
		t.Fatalf("expected ping count 25, got %d", cfg.PingCount)
	}
	if cfg.PingInterval != 60*time.Second {
		t.Fatalf("expected ping interval 60s, got %v", cfg.PingInterval)
	}
	if cfg.SpeedtestInterval != 30*time.Minute {
		t.Fatalf("expected speedtest interval 30m, got %v", cfg.SpeedtestInterval)
	}
	if len(cfg.DNSTargets) != 3 {
		t.Fatalf("expected 3 default DNS targets, got %d: %v", len(cfg.DNSTargets), cfg.DNSTargets)
	}
	if cfg.DNSTargets[0] != "google.com" {
		t.Fatalf("expected first DNS target google.com, got %s", cfg.DNSTargets[0])
	}
	if cfg.ListenAddr != ":4040" {
		t.Fatalf("expected listen addr :4040, got %s", cfg.ListenAddr)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("expected data dir /data, got %s", cfg.DataDir)
	}
	if cfg.AlertCooldown != 15*time.Minute {
		t.Fatalf("expected alert cooldown 15m, got %v", cfg.AlertCooldown)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PINGPONG_PING_TARGETS", "8.8.8.8,9.9.9.9")
	t.Setenv("PINGPONG_PING_COUNT", "5")
	t.Setenv("PINGPONG_PING_INTERVAL", "30s")
	t.Setenv("PINGPONG_LISTEN_ADDR", ":9090")

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

func TestLoadDNSTargetsPlural(t *testing.T) {
	t.Setenv("PINGPONG_DNS_TARGETS", "google.com,github.com")
	t.Setenv("PINGPONG_DNS_TARGET", "")
	t.Setenv("PINGPONG_DNS_SERVERS", "1.1.1.1,8.8.8.8")
	t.Setenv("PINGPONG_DNS_SERVER", "")
	cfg := Load()
	if len(cfg.DNSTargets) != 2 {
		t.Fatalf("expected 2 DNS targets, got %d", len(cfg.DNSTargets))
	}
	if cfg.DNSTargets[0] != "google.com" {
		t.Fatalf("expected first DNS target google.com, got %s", cfg.DNSTargets[0])
	}
	if len(cfg.DNSServers) != 2 {
		t.Fatalf("expected 2 DNS servers, got %d", len(cfg.DNSServers))
	}
	if cfg.DNSServers[0] != "1.1.1.1" {
		t.Fatalf("expected first DNS server 1.1.1.1, got %s", cfg.DNSServers[0])
	}
}

func TestLoadDNSTargetsFallback(t *testing.T) {
	t.Setenv("PINGPONG_DNS_TARGETS", "")
	t.Setenv("PINGPONG_DNS_TARGET", "example.com")
	t.Setenv("PINGPONG_DNS_SERVERS", "")
	t.Setenv("PINGPONG_DNS_SERVER", "9.9.9.9")
	cfg := Load()
	if len(cfg.DNSTargets) != 1 || cfg.DNSTargets[0] != "example.com" {
		t.Fatalf("expected fallback to singular DNS target, got %v", cfg.DNSTargets)
	}
	if len(cfg.DNSServers) != 1 || cfg.DNSServers[0] != "9.9.9.9" {
		t.Fatalf("expected fallback to singular DNS server, got %v", cfg.DNSServers)
	}
}

func TestLoadDNSTargetsDefaults(t *testing.T) {
	t.Setenv("PINGPONG_DNS_TARGETS", "")
	t.Setenv("PINGPONG_DNS_TARGET", "")
	t.Setenv("PINGPONG_DNS_SERVERS", "")
	t.Setenv("PINGPONG_DNS_SERVER", "")
	cfg := Load()
	if len(cfg.DNSTargets) != 3 {
		t.Fatalf("expected 3 default DNS targets, got %d: %v", len(cfg.DNSTargets), cfg.DNSTargets)
	}
	if len(cfg.DNSServers) != 0 {
		t.Fatalf("expected 0 default DNS servers (system only), got %d", len(cfg.DNSServers))
	}
}

func TestLoadInvalidInt(t *testing.T) {
	t.Setenv("PINGPONG_PING_COUNT", "abc")
	cfg := Load()
	if cfg.PingCount != 25 {
		t.Fatalf("expected ping count to fall back to default 25, got %d", cfg.PingCount)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	t.Setenv("PINGPONG_PING_INTERVAL", "notaduration")
	cfg := Load()
	if cfg.PingInterval != 60*time.Second {
		t.Fatalf("expected ping interval to fall back to default 60s, got %v", cfg.PingInterval)
	}
}

func TestLoadInvalidFloat(t *testing.T) {
	t.Setenv("PINGPONG_ALERT_PING_THRESHOLD", "notanumber")
	cfg := Load()
	if cfg.AlertPingThreshold != 100 {
		t.Fatalf("expected alert ping threshold to fall back to default 100, got %v", cfg.AlertPingThreshold)
	}
}

func TestLoadEmptyTargets(t *testing.T) {
	t.Setenv("PINGPONG_PING_TARGETS", ",, ,")
	cfg := Load()
	if len(cfg.PingTargets) != 3 {
		t.Fatalf("expected 3 default ping targets, got %d: %v", len(cfg.PingTargets), cfg.PingTargets)
	}
	if cfg.PingTargets[0] != "1.1.1.1" {
		t.Fatalf("expected first target 1.1.1.1, got %s", cfg.PingTargets[0])
	}
}

func TestLoadAlertRetryInterval(t *testing.T) {
	t.Setenv("PINGPONG_ALERT_RETRY_INTERVAL", "45s")
	cfg := Load()
	if cfg.AlertRetryInterval != 45*time.Second {
		t.Fatalf("expected alert retry interval 45s, got %v", cfg.AlertRetryInterval)
	}
}

func TestLoadAlertMaxRetries(t *testing.T) {
	t.Setenv("PINGPONG_ALERT_MAX_RETRIES", "5")
	cfg := Load()
	if cfg.AlertMaxRetries != 5 {
		t.Fatalf("expected alert max retries 5, got %d", cfg.AlertMaxRetries)
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Setenv("PINGPONG_PING_TARGETS", "")
	t.Setenv("PINGPONG_PING_COUNT", "")
	t.Setenv("PINGPONG_PING_INTERVAL", "")
	t.Setenv("PINGPONG_LISTEN_ADDR", "")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("PINGPONG_PING_TARGETS=9.9.9.9\nPINGPONG_PING_COUNT=3\nPINGPONG_PING_INTERVAL=45s\nPINGPONG_LISTEN_ADDR=:8080\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINGPONG_ENV_FILE", envPath)

	cfg := Load()

	if len(cfg.PingTargets) != 1 || cfg.PingTargets[0] != "9.9.9.9" {
		t.Fatalf("expected ping targets [9.9.9.9], got %v", cfg.PingTargets)
	}
	if cfg.PingCount != 3 {
		t.Fatalf("expected ping count 3, got %d", cfg.PingCount)
	}
	if cfg.PingInterval != 45*time.Second {
		t.Fatalf("expected ping interval 45s, got %v", cfg.PingInterval)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected listen addr :8080, got %s", cfg.ListenAddr)
	}
}

func TestLoadFromFile_StripsQuotesAndExportPrefix(t *testing.T) {
	t.Setenv("PINGPONG_PING_TARGETS", "")
	t.Setenv("PINGPONG_PING_COUNT", "")
	t.Setenv("PINGPONG_LISTEN_ADDR", "")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	content := "PINGPONG_PING_TARGETS=\"9.9.9.9\"\nexport PINGPONG_PING_COUNT=3\nexport PINGPONG_LISTEN_ADDR=\":8080\"\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINGPONG_ENV_FILE", envPath)

	cfg := Load()

	if len(cfg.PingTargets) != 1 || cfg.PingTargets[0] != "9.9.9.9" {
		t.Fatalf("expected ping targets [9.9.9.9], got %v", cfg.PingTargets)
	}
	if cfg.PingCount != 3 {
		t.Fatalf("expected ping count 3, got %d", cfg.PingCount)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected listen addr :8080, got %s", cfg.ListenAddr)
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte("PINGPONG_PING_COUNT=3\nPINGPONG_LISTEN_ADDR=:8080\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINGPONG_ENV_FILE", envPath)
	t.Setenv("PINGPONG_PING_COUNT", "99")
	t.Setenv("PINGPONG_LISTEN_ADDR", ":5555")

	cfg := Load()

	if cfg.PingCount != 99 {
		t.Fatalf("expected ping count 99 (env override), got %d", cfg.PingCount)
	}
	if cfg.ListenAddr != ":5555" {
		t.Fatalf("expected listen addr :5555 (env override), got %s", cfg.ListenAddr)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	t.Setenv("PINGPONG_ENV_FILE", filepath.Join(t.TempDir(), "nonexistent-env"))
	t.Setenv("PINGPONG_PING_COUNT", "")

	cfg := Load()

	if cfg.PingCount != 25 {
		t.Fatalf("expected ping count 25 (default), got %d", cfg.PingCount)
	}
}
