package collector

import (
	"context"
	"fmt"
	"log/slog"

	ndt7 "github.com/m-lab/ndt7-client-go"
	"github.com/m-lab/ndt7-client-go/spec"
)

// NDT7Result holds the results of an NDT7 speed test.
type NDT7Result struct {
	DownloadMbps float64
	UploadMbps   float64
	MinRTTMs     float64 // Minimum RTT observed during test (from TCP INFO)
	RetransRate  float64 // TCP retransmission rate (0.0-1.0)
	ServerName   string  // M-Lab server FQDN
}

// NDT7Collector runs single-stream speed tests using M-Lab's NDT7 protocol.
type NDT7Collector struct {
	clientName    string
	clientVersion string
}

// NewNDT7Collector creates a new NDT7 collector.
func NewNDT7Collector() *NDT7Collector {
	return &NDT7Collector{
		clientName:    "pingpong",
		clientVersion: "1.0.0",
	}
}

// Collect runs a full download + upload test (~20s total).
func (n *NDT7Collector) Collect(ctx context.Context) (NDT7Result, error) {
	slog.Info("running NDT7 speed test...")

	client := ndt7.NewClient(n.clientName, n.clientVersion)

	// Download test
	dlCh, err := client.StartDownload(ctx)
	if err != nil {
		return NDT7Result{}, fmt.Errorf("ndt7 start download: %w", err)
	}
	for range dlCh {
		// Drain the channel; the client stores the latest measurements internally.
	}

	// Upload test
	ulCh, err := client.StartUpload(ctx)
	if err != nil {
		return NDT7Result{}, fmt.Errorf("ndt7 start upload: %w", err)
	}
	for range ulCh {
		// Drain the channel; the client stores the latest measurements internally.
	}

	results := client.Results()
	result := parseNDT7Results(results[spec.TestDownload], results[spec.TestUpload], client.FQDN)

	slog.Info("NDT7 speed test complete",
		"download_mbps", fmt.Sprintf("%.1f", result.DownloadMbps),
		"upload_mbps", fmt.Sprintf("%.1f", result.UploadMbps),
		"min_rtt_ms", fmt.Sprintf("%.1f", result.MinRTTMs),
		"retrans_rate", fmt.Sprintf("%.4f", result.RetransRate),
		"server", result.ServerName,
	)
	return result, nil
}

// parseNDT7Results extracts throughput, latency, and retransmission data
// from the final NDT7 measurements. Unexported for unit testing.
func parseNDT7Results(dl, ul *ndt7.LatestMeasurements, fqdn string) NDT7Result {
	result := NDT7Result{ServerName: fqdn}

	// Download throughput from client-side AppInfo
	if dl != nil && dl.Client.AppInfo != nil {
		ai := dl.Client.AppInfo
		if ai.ElapsedTime > 0 {
			elapsed := float64(ai.ElapsedTime) / 1e6 // microseconds to seconds
			result.DownloadMbps = float64(ai.NumBytes) * 8 / elapsed / 1e6
		}
	}

	// MinRTT and retransmission rate from server-side TCPInfo (download)
	if dl != nil && dl.Server.TCPInfo != nil {
		ti := dl.Server.TCPInfo
		if ti.MinRTT > 0 {
			result.MinRTTMs = float64(ti.MinRTT) / 1000.0 // microseconds to ms
		}
		if ti.BytesSent > 0 {
			result.RetransRate = float64(ti.BytesRetrans) / float64(ti.BytesSent)
		}
	}

	// Upload throughput from server-side TCPInfo
	if ul != nil && ul.Server.TCPInfo != nil {
		ti := ul.Server.TCPInfo
		if ti.ElapsedTime > 0 {
			elapsed := float64(ti.ElapsedTime) / 1e6
			result.UploadMbps = float64(ti.BytesReceived) * 8 / elapsed / 1e6
		}
	}

	return result
}
