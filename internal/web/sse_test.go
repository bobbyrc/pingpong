package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestBroadcaster_SSE(t *testing.T) {
	// 1. Set up a registry with a single gauge.
	reg := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_metric",
		Help: "A test gauge",
	})
	reg.MustRegister(gauge)
	gauge.Set(42.0)

	// 2. Create a broadcaster with a short interval.
	b := NewBroadcaster(reg, 50*time.Millisecond)

	// 3. Start the broadcaster in the background.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go b.Run(ctx)

	// 4. Stand up a real HTTP test server so streaming/flushing works.
	srv := httptest.NewServer(b)
	defer srv.Close()

	// 5. Connect to the SSE endpoint.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE headers.
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// 6. Read SSE data lines. We expect at least one (the immediate snapshot).
	scanner := bufio.NewScanner(resp.Body)
	var found bool
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		var snap MetricSnapshot
		if err := json.Unmarshal([]byte(payload), &snap); err != nil {
			t.Fatalf("unmarshal snapshot: %v", err)
		}

		// Verify the snapshot contains our test metric.
		values, ok := snap.Metrics["test_metric"]
		if !ok {
			t.Fatalf("snapshot missing test_metric; got keys: %v", keysOf(snap.Metrics))
		}
		if len(values) != 1 {
			t.Fatalf("expected 1 value for test_metric, got %d", len(values))
		}
		if values[0].Value != 42.0 {
			t.Errorf("test_metric value = %v, want 42", values[0].Value)
		}
		if snap.Timestamp == "" {
			t.Error("snapshot timestamp is empty")
		}

		found = true
		break // One valid snapshot is enough.
	}

	if !found {
		t.Fatal("never received a valid SSE data line")
	}
}

func TestBroadcaster_GatherSnapshot(t *testing.T) {
	reg := prometheus.NewRegistry()

	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "labeled_gauge",
		Help: "A labeled gauge",
	}, []string{"env"})
	reg.MustRegister(gauge)
	gauge.WithLabelValues("prod").Set(99.9)

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "A test counter",
	})
	reg.MustRegister(counter)
	counter.Add(7)

	b := NewBroadcaster(reg, time.Second)
	snap, err := b.gatherSnapshot()
	if err != nil {
		t.Fatalf("gatherSnapshot: %v", err)
	}

	// Check gauge with labels.
	gVals, ok := snap.Metrics["labeled_gauge"]
	if !ok {
		t.Fatal("missing labeled_gauge")
	}
	if len(gVals) != 1 {
		t.Fatalf("expected 1 value, got %d", len(gVals))
	}
	if gVals[0].Value != 99.9 {
		t.Errorf("labeled_gauge = %v, want 99.9", gVals[0].Value)
	}
	if gVals[0].Labels["env"] != "prod" {
		t.Errorf("label env = %q, want prod", gVals[0].Labels["env"])
	}

	// Check counter.
	cVals, ok := snap.Metrics["test_counter"]
	if !ok {
		t.Fatal("missing test_counter")
	}
	if cVals[0].Value != 7 {
		t.Errorf("test_counter = %v, want 7", cVals[0].Value)
	}
}

func TestBroadcaster_SubscribeUnsubscribe(t *testing.T) {
	reg := prometheus.NewRegistry()
	b := NewBroadcaster(reg, time.Second)

	ch := b.subscribe()
	b.mu.Lock()
	if len(b.clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(b.clients))
	}
	b.mu.Unlock()

	b.unsubscribe(ch)
	b.mu.Lock()
	if len(b.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(b.clients))
	}
	b.mu.Unlock()
}

// keysOf returns the keys of a map for diagnostic output.
func keysOf(m map[string][]MetricValue) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
