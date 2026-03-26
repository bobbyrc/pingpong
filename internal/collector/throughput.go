package collector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	defaultThroughputDownloadURL = "https://speed.cloudflare.com/__down?bytes=250000000"
	defaultThroughputStreams     = 4
	defaultThroughputDuration    = 10 * time.Second
)

// ThroughputResult contains the result of a multi-stream download throughput test.
type ThroughputResult struct {
	DownloadMbps float64
	Streams      int
	DurationSecs float64
	BytesTotal   int64
}

// ThroughputCollector performs multi-stream parallel HTTP download tests.
type ThroughputCollector struct {
	downloadURL string
	streams     int
	duration    time.Duration
}

// NewThroughputCollector creates a ThroughputCollector.
func NewThroughputCollector(downloadURL string, streams int, duration time.Duration) *ThroughputCollector {
	if downloadURL == "" {
		downloadURL = defaultThroughputDownloadURL
	}
	if streams <= 0 {
		streams = defaultThroughputStreams
	}
	if streams > 16 {
		streams = 16
	}
	if duration <= 0 {
		duration = defaultThroughputDuration
	}
	return &ThroughputCollector{
		downloadURL: downloadURL,
		streams:     streams,
		duration:    duration,
	}
}

// Collect runs the multi-stream throughput test by launching parallel HTTP GETs
// and counting aggregate bytes within the configured duration.
func (t *ThroughputCollector) Collect(ctx context.Context) (ThroughputResult, error) {
	testCtx, cancel := context.WithTimeout(ctx, t.duration+5*time.Second)
	defer cancel()

	var totalBytes atomic.Int64
	done := make(chan struct{}, t.streams)

	start := time.Now()

	// Launch parallel download streams
	for i := 0; i < t.streams; i++ {
		go func(stream int) {
			defer func() { done <- struct{}{} }()
			bytes, err := downloadStream(testCtx, t.downloadURL, t.duration)
			if err != nil {
				slog.Debug("throughput stream error", "stream", stream, "error", err)
				return
			}
			totalBytes.Add(bytes)
		}(i)
	}

	// Wait for all streams to finish
	for i := 0; i < t.streams; i++ {
		<-done
	}

	elapsed := time.Since(start)
	// Cap at configured duration to avoid diluting throughput with teardown overhead
	if elapsed > t.duration {
		elapsed = t.duration
	}
	total := totalBytes.Load()

	if total == 0 {
		return ThroughputResult{}, fmt.Errorf("all %d download streams failed or returned no data", t.streams)
	}

	result := ThroughputResult{
		DownloadMbps: computeThroughput(total, elapsed),
		Streams:      t.streams,
		DurationSecs: elapsed.Seconds(),
		BytesTotal:   total,
	}

	return result, nil
}

// downloadStream performs a single HTTP GET and reads bytes until the duration
// elapses, the context is cancelled, or the server closes the connection.
func downloadStream(ctx context.Context, url string, duration time.Duration) (int64, error) {
	dlCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	buf := make([]byte, 64*1024)
	var total int64

	for {
		n, err := resp.Body.Read(buf)
		total += int64(n)
		if err != nil {
			if err == io.EOF || dlCtx.Err() != nil {
				return total, nil
			}
			return total, err
		}
	}
}

// computeThroughput converts bytes and elapsed time to Mbps.
func computeThroughput(bytes int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	bits := float64(bytes) * 8
	seconds := elapsed.Seconds()
	return bits / seconds / 1_000_000
}
