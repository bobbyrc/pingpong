package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestComputeThroughput(t *testing.T) {
	tests := []struct {
		name    string
		bytes   int64
		elapsed time.Duration
		want    float64
	}{
		{"1MB in 1s = 8 Mbps", 1_000_000, 1 * time.Second, 8.0},
		{"10MB in 1s = 80 Mbps", 10_000_000, 1 * time.Second, 80.0},
		{"125MB in 10s = 100 Mbps", 125_000_000, 10 * time.Second, 100.0},
		{"zero bytes", 0, 1 * time.Second, 0},
		{"zero duration", 1_000_000, 0, 0},
		{"negative duration", 1_000_000, -1 * time.Second, 0},
	}

	for _, tt := range tests {
		got := ComputeThroughput(tt.bytes, tt.elapsed)
		if got != tt.want {
			t.Errorf("ComputeThroughput(%s) = %.2f, want %.2f", tt.name, got, tt.want)
		}
	}
}

func TestComputeThroughput_LargeValues(t *testing.T) {
	// 1 Gbps = 125MB/s, so 1.25GB in 10s
	bytes := int64(1_250_000_000)
	elapsed := 10 * time.Second
	got := ComputeThroughput(bytes, elapsed)
	want := 1000.0
	if got != want {
		t.Errorf("ComputeThroughput(1Gbps) = %.2f, want %.2f", got, want)
	}
}

func TestThroughputCollector_Collect(t *testing.T) {
	// Serve 1MB of data per request
	data := make([]byte, 1_000_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(data)
	}))
	defer srv.Close()

	tc := NewThroughputCollector(srv.URL, 2, 1*time.Second)
	result, err := tc.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if result.Streams != 2 {
		t.Errorf("Streams = %d, want 2", result.Streams)
	}
	if result.BytesTotal == 0 {
		t.Error("BytesTotal = 0, expected data to be downloaded")
	}
	if result.DownloadMbps <= 0 {
		t.Errorf("DownloadMbps = %.2f, expected > 0", result.DownloadMbps)
	}
	if result.DurationSecs <= 0 {
		t.Errorf("DurationSecs = %.2f, expected > 0", result.DurationSecs)
	}
}

func TestThroughputCollector_Collect_ContextCancel(t *testing.T) {
	// Serve data slowly so context cancellation is the exit path
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		buf := make([]byte, 1024)
		for {
			if _, err := w.Write(buf); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	tc := NewThroughputCollector(srv.URL, 1, 2*time.Second)
	result, err := tc.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Should have downloaded some bytes before context cancelled
	if result.BytesTotal == 0 {
		t.Error("BytesTotal = 0, expected some data before cancellation")
	}
}
