package collector

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// DefaultBufferbloatDownloadURL is the default CDN endpoint for load generation.
const DefaultBufferbloatDownloadURL = "https://speed.cloudflare.com/__down?bytes=100000000"

// BufferbloatResult holds the results of a latency-under-load test.
type BufferbloatResult struct {
	IdleLatencyMs     float64 // Baseline latency (median of idle pings)
	LoadedLatencyMs   float64 // Latency measured during download (median)
	LatencyIncreaseMs float64 // LoadedLatencyMs - IdleLatencyMs
	Grade             string  // "A+", "A", "B", "C", "D", "F"
	DownloadMbps      float64 // Throughput during the load phase (byproduct)
}

// BufferbloatCollector measures latency degradation under load.
type BufferbloatCollector struct {
	pingTarget  string
	downloadURL string
}

// NewBufferbloatCollector creates a new bufferbloat collector.
func NewBufferbloatCollector(pingTarget, downloadURL string) *BufferbloatCollector {
	return &BufferbloatCollector{
		pingTarget:  pingTarget,
		downloadURL: downloadURL,
	}
}

// Collect runs a latency-under-load test.
func (b *BufferbloatCollector) Collect(ctx context.Context) (BufferbloatResult, error) {
	slog.Info("running bufferbloat test...", "target", b.pingTarget)

	// Phase 1: Idle latency measurement (5 pings over 1 second)
	idleSamples, err := b.measureLatency(ctx, 5, 200*time.Millisecond)
	if err != nil {
		return BufferbloatResult{}, fmt.Errorf("bufferbloat idle measurement: %w", err)
	}
	idleLatency := ComputeMedian(idleSamples)

	// Phase 2: Start HTTP download to generate load, measure latency concurrently
	loadCtx, loadCancel := context.WithTimeout(ctx, 6*time.Second)
	defer loadCancel()

	var totalBytes atomic.Int64
	var downloadDone sync.WaitGroup
	downloadStart := time.Now()

	// Start download goroutine
	downloadDone.Add(1)
	go func() {
		defer downloadDone.Done()
		req, reqErr := http.NewRequestWithContext(loadCtx, "GET", b.downloadURL, nil)
		if reqErr != nil {
			return
		}
		resp, respErr := http.DefaultClient.Do(req)
		if respErr != nil {
			return
		}
		defer resp.Body.Close()
		buf := make([]byte, 64*1024)
		for {
			n, readErr := resp.Body.Read(buf)
			totalBytes.Add(int64(n))
			if readErr != nil {
				break
			}
		}
	}()

	// Brief pause to let download ramp up
	select {
	case <-ctx.Done():
		return BufferbloatResult{}, ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	// Measure latency under load (~25 pings over 5 seconds)
	loadedSamples, err := b.measureLatency(ctx, 25, 200*time.Millisecond)
	loadCancel() // Stop the download
	downloadDone.Wait()

	downloadElapsed := time.Since(downloadStart)

	if err != nil || len(loadedSamples) == 0 {
		if err != nil {
			return BufferbloatResult{}, fmt.Errorf("bufferbloat loaded measurement: %w", err)
		}
		return BufferbloatResult{}, fmt.Errorf("bufferbloat: no loaded latency samples")
	}

	loadedLatency := ComputeMedian(loadedSamples)
	latencyIncrease := loadedLatency - idleLatency
	if latencyIncrease < 0 {
		latencyIncrease = 0
	}

	throughput := ComputeThroughput(totalBytes.Load(), downloadElapsed)

	grade := GradeBufferbloat(latencyIncrease)

	result := BufferbloatResult{
		IdleLatencyMs:     idleLatency,
		LoadedLatencyMs:   loadedLatency,
		LatencyIncreaseMs: latencyIncrease,
		Grade:             grade,
		DownloadMbps:      throughput,
	}

	slog.Info("bufferbloat test complete",
		"idle_ms", fmt.Sprintf("%.1f", result.IdleLatencyMs),
		"loaded_ms", fmt.Sprintf("%.1f", result.LoadedLatencyMs),
		"increase_ms", fmt.Sprintf("%.1f", result.LatencyIncreaseMs),
		"grade", result.Grade,
		"download_mbps", fmt.Sprintf("%.1f", result.DownloadMbps),
	)

	return result, nil
}

// measureLatency sends ICMP pings and returns RTT samples in milliseconds.
func (b *BufferbloatCollector) measureLatency(ctx context.Context, count int, interval time.Duration) ([]float64, error) {
	pinger, err := probing.NewPinger(b.pingTarget)
	if err != nil {
		return nil, fmt.Errorf("create pinger: %w", err)
	}
	pinger.Count = count
	pinger.Interval = interval
	pinger.Timeout = time.Duration(count)*interval + 2*time.Second
	pinger.SetPrivileged(true)

	var mu sync.Mutex
	var samples []float64

	pinger.OnRecv = func(pkt *probing.Packet) {
		mu.Lock()
		samples = append(samples, float64(pkt.Rtt.Microseconds())/1000.0)
		mu.Unlock()
	}

	if err := pinger.RunWithContext(ctx); err != nil {
		return nil, fmt.Errorf("run pinger: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	return samples, nil
}

// ── Pure functions (exported for testing) ────────────────────

// GradeBufferbloat assigns a letter grade based on latency increase.
func GradeBufferbloat(latencyIncreaseMs float64) string {
	switch {
	case latencyIncreaseMs < 5:
		return "A+"
	case latencyIncreaseMs < 30:
		return "A"
	case latencyIncreaseMs < 60:
		return "B"
	case latencyIncreaseMs < 200:
		return "C"
	case latencyIncreaseMs < 400:
		return "D"
	default:
		return "F"
	}
}

// GradeToNumeric converts a letter grade to a numeric value for Prometheus.
func GradeToNumeric(grade string) float64 {
	switch grade {
	case "A+":
		return 6
	case "A":
		return 5
	case "B":
		return 4
	case "C":
		return 3
	case "D":
		return 2
	case "F":
		return 1
	default:
		return 0
	}
}

// ComputeMedian returns the median of a float64 slice.
func ComputeMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
