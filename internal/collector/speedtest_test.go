package collector

import (
	"testing"
)

func TestParseSpeedtestOutput(t *testing.T) {
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

	// bandwidth is bytes/sec, convert to Mbps: 12500000 * 8 / 1000000 = 100
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
