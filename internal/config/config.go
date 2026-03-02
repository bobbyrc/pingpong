package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	PingTargets      []string
	PingCount        int
	DNSTarget        string
	DNSServer        string
	TracerouteTarget string

	PingInterval       time.Duration
	SpeedtestInterval  time.Duration
	DNSInterval        time.Duration
	TracerouteInterval time.Duration

	AlertDowntimeThreshold   time.Duration
	AlertPacketLossThreshold float64
	AlertPingThreshold       float64
	AlertSpeedThreshold      float64
	AlertJitterThreshold     float64
	AlertCooldown            time.Duration
	AlertMaxRetries          int
	AlertRetryInterval       time.Duration

	AppriseURL  string
	AppriseURLs string

	ListenAddr string
	DataDir    string
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
		if len(result) == 0 {
			return fallback
		}
		return result
	}
	return fallback
}
