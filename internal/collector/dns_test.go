package collector

import (
	"context"
	"testing"
	"time"
)

func TestDNSCollectorMultiTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DNS integration test in short mode")
	}

	c := NewDNSCollector([]string{"google.com", "cloudflare.com"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, failures := c.Collect(ctx)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(failures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(failures))
	}
	for _, r := range results {
		if r.Server != "system" {
			t.Fatalf("expected server 'system', got %s", r.Server)
		}
		if r.ResolutionMs <= 0 {
			t.Fatalf("expected positive resolution time for %s, got %f", r.Target, r.ResolutionMs)
		}
	}
}

func TestDNSCollectorWithServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DNS integration test in short mode")
	}

	c := NewDNSCollector([]string{"google.com"}, []string{"1.1.1.1"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, failures := c.Collect(ctx)
	// Expect 2 results: (google.com, system) and (google.com, 1.1.1.1)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (system + custom server), got %d", len(results))
	}
	if len(failures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(failures))
	}

	servers := map[string]bool{}
	for _, r := range results {
		servers[r.Server] = true
	}
	if !servers["system"] {
		t.Fatal("expected a result with server 'system'")
	}
	if !servers["1.1.1.1"] {
		t.Fatal("expected a result with server '1.1.1.1'")
	}
}
