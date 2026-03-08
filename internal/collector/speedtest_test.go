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

	if result.DownloadMbps != 100.0 {
		t.Fatalf("expected download 100 Mbps, got %f", result.DownloadMbps)
	}
	if result.UploadMbps != 50.0 {
		t.Fatalf("expected upload 50 Mbps, got %f", result.UploadMbps)
	}
	if result.LatencyMs != 12.345 {
		t.Fatalf("expected latency 12.345, got %f", result.LatencyMs)
	}
	if result.ServerName != "Test Server" {
		t.Fatalf("expected server name 'Test Server', got %q", result.ServerName)
	}
	if result.ServerLocation != "Test City" {
		t.Fatalf("expected server location 'Test City', got %q", result.ServerLocation)
	}
	if result.ISP != "Test ISP" {
		t.Fatalf("expected ISP 'Test ISP', got %q", result.ISP)
	}
}

func TestParseSpeedtestOutputMissingMetadata(t *testing.T) {
	output := `{
		"ping": {"latency": 10.0},
		"download": {"bandwidth": 1000000},
		"upload": {"bandwidth": 500000}
	}`

	result, err := parseSpeedtestOutput([]byte(output))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if result.ServerName != "" {
		t.Fatalf("expected empty server name, got %q", result.ServerName)
	}
	if result.ISP != "" {
		t.Fatalf("expected empty ISP, got %q", result.ISP)
	}
}
