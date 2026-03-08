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

// MetricSnapshot is the JSON structure broadcast to SSE clients.
type MetricSnapshot struct {
	Timestamp string                   `json:"timestamp"`
	Metrics   map[string][]MetricValue `json:"metrics"`
}

// MetricValue holds a single metric sample with optional labels.
type MetricValue struct {
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

// Broadcaster gathers Prometheus metrics at a fixed interval and pushes
// JSON-encoded snapshots to all connected SSE clients.
type Broadcaster struct {
	gatherer prometheus.Gatherer
	interval time.Duration

	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// NewBroadcaster creates a Broadcaster that reads from the given gatherer
// every interval.
func NewBroadcaster(gatherer prometheus.Gatherer, interval time.Duration) *Broadcaster {
	return &Broadcaster{
		gatherer: gatherer,
		interval: interval,
		clients:  make(map[chan []byte]struct{}),
	}
}

// Run starts the broadcast loop. It blocks until ctx is cancelled.
func (b *Broadcaster) Run(ctx context.Context) {
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
func (b *Broadcaster) broadcast() {
	data, err := b.snapshotJSON()
	if err != nil {
		slog.Error("failed to gather metrics snapshot", "error", err)
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

// subscribe adds a new client channel and returns it.
func (b *Broadcaster) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a client channel from the broadcast set.
func (b *Broadcaster) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	// Do not close the channel here — the sender (broadcast) may still
	// be selecting on it. Let GC reclaim it once both sides drop the ref.
}

// ServeHTTP implements http.Handler for the SSE endpoint.
func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

// snapshotJSON gathers metrics and returns the JSON-encoded snapshot.
func (b *Broadcaster) snapshotJSON() ([]byte, error) {
	snap, err := b.gatherSnapshot()
	if err != nil {
		return nil, err
	}
	return json.Marshal(snap)
}

// gatherSnapshot reads all metric families from the registry and converts
// GAUGE and COUNTER types into a MetricSnapshot.
func (b *Broadcaster) gatherSnapshot() (*MetricSnapshot, error) {
	families, err := b.gatherer.Gather()
	if err != nil {
		return nil, fmt.Errorf("gather: %w", err)
	}

	snap := &MetricSnapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Metrics:   make(map[string][]MetricValue),
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

			mv := MetricValue{Value: val}
			if pairs := m.GetLabel(); len(pairs) > 0 {
				mv.Labels = make(map[string]string, len(pairs))
				for _, lp := range pairs {
					mv.Labels[lp.GetName()] = lp.GetValue()
				}
			}

			snap.Metrics[name] = append(snap.Metrics[name], mv)
		}
	}

	return snap, nil
}
