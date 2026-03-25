package collector

import (
	"math"
	"testing"

	ndt7 "github.com/m-lab/ndt7-client-go"
	"github.com/m-lab/ndt7-client-go/spec"
)

func TestParseNDT7Results(t *testing.T) {
	dl := &ndt7.LatestMeasurements{
		Client: spec.Measurement{
			AppInfo: &spec.AppInfo{
				NumBytes:    125_000_000, // 125 MB
				ElapsedTime: 10_000_000,  // 10 seconds in microseconds
			},
		},
		Server: spec.Measurement{
			TCPInfo: &spec.TCPInfo{
				ElapsedTime: 10_000_000,
			},
		},
	}
	// Set TCP INFO fields via the embedded LinuxTCPInfo
	dl.Server.TCPInfo.MinRTT = 8200       // 8.2ms in microseconds
	dl.Server.TCPInfo.BytesSent = 1000000  // 1 MB
	dl.Server.TCPInfo.BytesRetrans = 1000  // 1 KB retransmitted

	ul := &ndt7.LatestMeasurements{
		Server: spec.Measurement{
			TCPInfo: &spec.TCPInfo{
				ElapsedTime: 10_000_000, // 10 seconds
			},
		},
	}
	ul.Server.TCPInfo.BytesReceived = 12_500_000 // 12.5 MB

	result := parseNDT7Results(dl, ul, "mlab-nyc01.example.com")

	// Download: 125MB * 8 / 10s / 1e6 = 100 Mbps
	if math.Abs(result.DownloadMbps-100.0) > 0.01 {
		t.Fatalf("expected download ~100 Mbps, got %.2f", result.DownloadMbps)
	}

	// Upload: 12.5MB * 8 / 10s / 1e6 = 10 Mbps
	if math.Abs(result.UploadMbps-10.0) > 0.01 {
		t.Fatalf("expected upload ~10 Mbps, got %.2f", result.UploadMbps)
	}

	// MinRTT: 8200us / 1000 = 8.2ms
	if math.Abs(result.MinRTTMs-8.2) > 0.01 {
		t.Fatalf("expected min RTT ~8.2ms, got %.2f", result.MinRTTMs)
	}

	// Retransmission rate: 1000 / 1000000 = 0.001
	if math.Abs(result.RetransRate-0.001) > 0.0001 {
		t.Fatalf("expected retrans rate ~0.001, got %.4f", result.RetransRate)
	}

	if result.ServerName != "mlab-nyc01.example.com" {
		t.Fatalf("expected server mlab-nyc01.example.com, got %s", result.ServerName)
	}
}

func TestParseNDT7Results_NilMeasurements(t *testing.T) {
	result := parseNDT7Results(nil, nil, "test-server")

	if result.DownloadMbps != 0 {
		t.Fatalf("expected 0 download with nil measurements, got %.2f", result.DownloadMbps)
	}
	if result.UploadMbps != 0 {
		t.Fatalf("expected 0 upload with nil measurements, got %.2f", result.UploadMbps)
	}
	if result.MinRTTMs != 0 {
		t.Fatalf("expected 0 min RTT with nil measurements, got %.2f", result.MinRTTMs)
	}
	if result.RetransRate != 0 {
		t.Fatalf("expected 0 retrans rate with nil measurements, got %.4f", result.RetransRate)
	}
	if result.ServerName != "test-server" {
		t.Fatalf("expected server test-server, got %s", result.ServerName)
	}
}

func TestParseNDT7Results_NilAppInfo(t *testing.T) {
	dl := &ndt7.LatestMeasurements{
		Client: spec.Measurement{AppInfo: nil},
		Server: spec.Measurement{TCPInfo: nil},
	}
	ul := &ndt7.LatestMeasurements{
		Server: spec.Measurement{TCPInfo: nil},
	}

	result := parseNDT7Results(dl, ul, "server")

	if result.DownloadMbps != 0 || result.UploadMbps != 0 {
		t.Fatalf("expected 0 speeds with nil AppInfo/TCPInfo, got dl=%.2f ul=%.2f",
			result.DownloadMbps, result.UploadMbps)
	}
}

func TestParseNDT7Results_ZeroElapsedTime(t *testing.T) {
	dl := &ndt7.LatestMeasurements{
		Client: spec.Measurement{
			AppInfo: &spec.AppInfo{
				NumBytes:    125_000_000,
				ElapsedTime: 0, // zero elapsed = no throughput
			},
		},
	}

	result := parseNDT7Results(dl, nil, "server")

	if result.DownloadMbps != 0 {
		t.Fatalf("expected 0 download with zero elapsed time, got %.2f", result.DownloadMbps)
	}
}

func TestParseNDT7Results_ZeroBytesRetrans(t *testing.T) {
	dl := &ndt7.LatestMeasurements{
		Client: spec.Measurement{
			AppInfo: &spec.AppInfo{
				NumBytes:    50_000_000,
				ElapsedTime: 5_000_000,
			},
		},
		Server: spec.Measurement{
			TCPInfo: &spec.TCPInfo{
				ElapsedTime: 5_000_000,
			},
		},
	}
	dl.Server.TCPInfo.BytesSent = 50_000_000
	dl.Server.TCPInfo.BytesRetrans = 0
	dl.Server.TCPInfo.MinRTT = 5000 // 5ms

	result := parseNDT7Results(dl, nil, "server")

	if result.RetransRate != 0 {
		t.Fatalf("expected 0 retrans rate with zero retrans, got %.4f", result.RetransRate)
	}
	if math.Abs(result.MinRTTMs-5.0) > 0.01 {
		t.Fatalf("expected min RTT 5.0ms, got %.2f", result.MinRTTMs)
	}
}
