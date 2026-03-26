package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// metricSnapshot is the JSON structure broadcast to SSE clients.
type metricSnapshot struct {
	Timestamp string                   `json:"timestamp"`
	Metrics   map[string][]metricValue `json:"metrics"`
}

// metricValue holds a single metric sample with optional labels.
type metricValue struct {
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// broadcaster gathers Prometheus metrics at a fixed interval and pushes
// JSON-encoded snapshots to all connected SSE clients.
type broadcaster struct {
	gatherer prometheus.Gatherer
	interval time.Duration
	history  *HistoryStore // may be nil

	mu      sync.Mutex
	clients map[chan []byte]struct{}

	pruneCount int                // broadcast ticks since last prune
	lastValues map[string]float64 // dedup: last recorded value per "metric:target"

	// hostnames maps ping target -> resolved hostname (for SSE clients).
	hostnames map[string]string
}

// newBroadcaster creates a broadcaster that reads from the given gatherer
// every interval.
func newBroadcaster(gatherer prometheus.Gatherer, interval time.Duration, history *HistoryStore) *broadcaster {
	return &broadcaster{
		gatherer:   gatherer,
		interval:   interval,
		history:    history,
		clients:    make(map[chan []byte]struct{}),
		lastValues: make(map[string]float64),
	}
}

// run starts the broadcast loop. It blocks until ctx is cancelled.
func (b *broadcaster) run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.broadcast()
		}
	}
}

// broadcast gathers a snapshot and sends it to every connected client.
func (b *broadcaster) broadcast() {
	snap, err := b.gatherSnapshot()
	if err != nil {
		slog.Error("failed to gather metrics snapshot", "error", err)
		return
	}

	b.recordHistory(snap)

	data, err := json.Marshal(snap)
	if err != nil {
		slog.Error("failed to marshal metrics snapshot", "error", err)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for ch := range b.clients {
		select {
		case ch <- data:
		default:
			// Client is slow; drop this message to avoid blocking.
		}
	}
}

// recordHistory persists sparkline-relevant metrics from the snapshot.
func (b *broadcaster) recordHistory(snap *metricSnapshot) {
	if b.history == nil {
		return
	}

	const keep = 60
	const pruneEvery = 12 // prune every 12th tick (~1 minute at 5s interval)

	b.pruneCount++
	shouldPrune := b.pruneCount >= pruneEvery
	if shouldPrune {
		b.pruneCount = 0
	}

	record := func(metric, target string, value float64) {
		key := metric + ":" + target
		if last, ok := b.lastValues[key]; ok && last == value {
			return
		}
		b.lastValues[key] = value
		if err := b.history.record(metric, target, value); err != nil {
			slog.Error("failed to record history", "metric", metric, "target", target, "error", err)
		}
	}

	// Ping latency per target (labelled metric)
	for _, mv := range snap.Metrics["pingpong_ping_latency_ms"] {
		target := ""
		if mv.Labels != nil {
			target = mv.Labels["target"]
		}
		record("ping_latency", target, mv.Value)
		if shouldPrune {
			if err := b.history.prune("ping_latency", target, keep); err != nil {
				slog.Error("failed to prune ping history", "target", target, "error", err)
			}
		}
	}

	// Scalar metrics: record and prune via table-driven loop.
	// Gate bandwidth-related series on a "has real data" signal to avoid
	// recording default gauge zeros before any test has run.
	hasNDT7 := len(snap.Metrics["pingpong_ndt7_info"]) > 0

	type historySpec struct {
		promName string
		seriesID string
		gate     bool
	}
	specs := []historySpec{
		{"pingpong_download_speed_mbps", "download_speed", hasNDT7},
		{"pingpong_upload_speed_mbps", "upload_speed", hasNDT7},
	}
	for _, spec := range specs {
		if spec.gate {
			for _, mv := range snap.Metrics[spec.promName] {
				record(spec.seriesID, "", mv.Value)
			}
		}
		if shouldPrune {
			if err := b.history.prune(spec.seriesID, "", keep); err != nil {
				slog.Error("failed to prune history", "series", spec.seriesID, "error", err)
			}
		}
	}
}

// subscribe adds a new client channel and returns it.
func (b *broadcaster) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a client channel from the broadcast set.
func (b *broadcaster) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	// Do not close the channel here — the sender (broadcast) may still
	// be selecting on it. Let GC reclaim it once both sides drop the ref.
}

// ServeHTTP implements http.Handler for the SSE endpoint.
func (b *broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	// Send an immediate snapshot so the client doesn't wait for the next tick.
	if data, err := b.snapshotJSON(); err == nil {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return
		}
		flusher.Flush()
	} else {
		slog.Error("failed to gather initial SSE snapshot", "error", err)
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// snapshotJSON gathers metrics and returns the JSON-encoded snapshot.
func (b *broadcaster) snapshotJSON() ([]byte, error) {
	snap, err := b.gatherSnapshot()
	if err != nil {
		return nil, err
	}
	return json.Marshal(snap)
}

// gatherSnapshot reads all metric families from the registry and converts
// GAUGE and COUNTER types into a metricSnapshot.
func (b *broadcaster) gatherSnapshot() (*metricSnapshot, error) {
	families, err := b.gatherer.Gather()
	if err != nil {
		return nil, fmt.Errorf("gather: %w", err)
	}

	snap := &metricSnapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Metrics:   make(map[string][]metricValue),
	}

	for _, fam := range families {
		name := fam.GetName()
		typ := fam.GetType()

		for _, m := range fam.GetMetric() {
			var val float64
			switch typ {
			case dto.MetricType_GAUGE:
				val = m.GetGauge().GetValue()
			case dto.MetricType_COUNTER:
				val = m.GetCounter().GetValue()
			default:
				continue
			}

			mv := metricValue{Value: val}
			if pairs := m.GetLabel(); len(pairs) > 0 {
				mv.Labels = make(map[string]string, len(pairs))
				for _, lp := range pairs {
					mv.Labels[lp.GetName()] = lp.GetValue()
				}
			}

			// Inject resolved hostname for ping metrics
			if b.hostnames != nil && name == "pingpong_ping_latency_ms" {
				if mv.Labels != nil {
					if hostname, ok := b.hostnames[mv.Labels["target"]]; ok {
						mv.Labels["hostname"] = hostname
					}
				}
			}

			snap.Metrics[name] = append(snap.Metrics[name], mv)
		}
	}

	return snap, nil
}
