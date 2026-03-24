package collector

import (
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
