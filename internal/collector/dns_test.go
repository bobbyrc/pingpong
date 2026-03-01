package collector

import (
	"context"
	"testing"
)

func TestDNSCollectorResolves(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DNS integration test in short mode")
	}

	c := NewDNSCollector("google.com", "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*1e9)
	defer cancel()

	result, err := c.Collect(ctx)
	if err != nil {
		t.Fatalf("DNS resolve failed: %v", err)
	}
	if result.Target != "google.com" {
		t.Fatalf("expected target google.com, got %s", result.Target)
	}
	if result.ResolutionMs <= 0 {
		t.Fatalf("expected positive resolution time, got %f", result.ResolutionMs)
	}
}
