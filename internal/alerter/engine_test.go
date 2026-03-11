package alerter

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/bobbyrc/pingpong/internal/collector"
	"github.com/bobbyrc/pingpong/internal/config"
)

// dummyApprise returns a non-nil AppriseClient for tests.
// fireAlert skips enqueuing when apprise is nil, so tests that
// expect alerts to be enqueued need a non-nil client.
var dummyApprise = NewAppriseClient("http://localhost", "test://")

// failingApprise returns an AppriseClient backed by an httptest server that
// always responds 500. Use this instead of dummyApprise when the test actually
// calls ProcessQueue/Send so failure is deterministic with no real network I/O.
func failingApprise(t *testing.T) *AppriseClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return NewAppriseClient(srv.URL, "test://")
}

func newTestEngine(t *testing.T, apprise *AppriseClient, cfg *config.Config) (*Engine, *Queue) {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	q, err := NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	return NewEngine(q, apprise, cfg), q
}

func TestEngineEvaluatePacketLoss(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:           1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 15.0, AvgMs: 20},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for high packet loss, got %d", len(pending))
	}
	if pending[0].AlertType != "packet_loss" {
		t.Fatalf("expected alert type packet_loss, got %s", pending[0].AlertType)
	}
}

func TestEngineEvaluateNoAlert(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertPingThreshold:       100,
		AlertCooldown:            1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 5.0, AvgMs: 20},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts for normal values, got %d", len(pending))
	}
}

func TestEngineCooldown(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            5 * time.Minute,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0, AvgMs: 20},
	}

	engine.EvaluatePing(results)
	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert on first eval, got %d", len(pending))
	}

	engine.EvaluatePing(results)
	pending, _ = q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected still 1 alert (cooldown active), got %d", len(pending))
	}
}

func TestEngineEvaluateSpeed(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertSpeedThreshold: 50,
		AlertCooldown:       1 * time.Second,
	})

	result := collector.SpeedtestResult{
		DownloadMbps: 25.0,
		UploadMbps:   10.0,
	}

	engine.EvaluateSpeed(result)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for slow speed, got %d", len(pending))
	}
	if pending[0].AlertType != "speed" {
		t.Fatalf("expected alert type speed, got %s", pending[0].AlertType)
	}
}

func TestEngineDisabledThresholds(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 0,
		AlertPingThreshold:       0,
		AlertSpeedThreshold:      0,
		AlertJitterThreshold:     0,
		AlertCooldown:            1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 100, AvgMs: 999, JitterMs: 999},
	}
	engine.EvaluatePing(results)

	speed := collector.SpeedtestResult{DownloadMbps: 0.1}
	engine.EvaluateSpeed(speed)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts with disabled thresholds, got %d", len(pending))
	}
}

func TestEvaluatePing_LatencyThreshold(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPingThreshold: 100,
		AlertCooldown:      1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "8.8.8.8", AvgMs: 150.0, PacketLoss: 0},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for high latency, got %d", len(pending))
	}
	if pending[0].AlertType != "latency" {
		t.Fatalf("expected alert type latency, got %s", pending[0].AlertType)
	}
	if pending[0].CooldownKey != "latency:8.8.8.8" {
		t.Fatalf("expected cooldown key latency:8.8.8.8, got %s", pending[0].CooldownKey)
	}
}

func TestEvaluatePing_JitterThreshold(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertJitterThreshold: 20,
		AlertCooldown:        1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "8.8.8.8", JitterMs: 35.0, AvgMs: 10, PacketLoss: 0},
	}

	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for high jitter, got %d", len(pending))
	}
	if pending[0].AlertType != "jitter" {
		t.Fatalf("expected alert type jitter, got %s", pending[0].AlertType)
	}
	if pending[0].CooldownKey != "jitter:8.8.8.8" {
		t.Fatalf("expected cooldown key jitter:8.8.8.8, got %s", pending[0].CooldownKey)
	}
}

func TestEvaluateDowntime_BelowThreshold(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertDowntimeThreshold: 2 * time.Minute,
		AlertCooldown:          1 * time.Second,
	})

	// Down for only 30 seconds — below the 2-minute threshold.
	downSince := time.Now().Add(-30 * time.Second)
	engine.EvaluateDowntime(true, downSince)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts when downtime below threshold, got %d", len(pending))
	}
}

func TestEvaluateDowntime_AboveThreshold(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertDowntimeThreshold: 2 * time.Minute,
		AlertCooldown:          1 * time.Second,
	})

	// Down for 5 minutes — above the 2-minute threshold.
	downSince := time.Now().Add(-5 * time.Minute)
	engine.EvaluateDowntime(true, downSince)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert when downtime above threshold, got %d", len(pending))
	}
	if pending[0].AlertType != "downtime" {
		t.Fatalf("expected alert type downtime, got %s", pending[0].AlertType)
	}
}

func TestEvaluateDowntime_NotDown(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertDowntimeThreshold: 1 * time.Minute,
		AlertCooldown:          1 * time.Second,
	})

	// isDown=false should be a no-op regardless of downSince.
	engine.EvaluateDowntime(false, time.Now().Add(-10*time.Minute))

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts when not down, got %d", len(pending))
	}
}

func TestEvaluateDowntime_DisabledThreshold(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertDowntimeThreshold: 0, // disabled
		AlertCooldown:          1 * time.Second,
	})

	// Even though we've been down for a long time, threshold=0 means disabled.
	engine.EvaluateDowntime(true, time.Now().Add(-1*time.Hour))

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts when downtime threshold is disabled (0), got %d", len(pending))
	}
}

func TestSeedCooldowns(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	cfg := &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            5 * time.Minute,
	}

	// Phase 1: create an engine, fire an alert, mark it sent.
	db1, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	q1, err := NewQueue(db1)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	e1 := NewEngine(q1, dummyApprise, cfg)

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	}
	e1.EvaluatePing(results)

	pending, _ := q1.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if err := q1.MarkSent(pending[0].ID); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	db1.Close()

	// Phase 2: create a NEW engine with the same DB, call SeedCooldowns,
	// and verify the same alert type+target is suppressed.
	db2, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db2.Close() })
	q2, err := NewQueue(db2)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}
	e2 := NewEngine(q2, dummyApprise, cfg)
	e2.SeedCooldowns()

	// Try to fire the same alert — should be suppressed by cooldown.
	e2.EvaluatePing(results)

	pending, _ = q2.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending alerts after SeedCooldowns (cooldown should suppress), got %d", len(pending))
	}
}

func TestEvaluatePing_MultiTargetCooldownIsolation(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            5 * time.Minute,
	})

	// Fire alert for target A.
	resultsA := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	}
	engine.EvaluatePing(resultsA)

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 alert for target A, got %d", len(pending))
	}

	// Now fire for target B — should succeed despite A being in cooldown.
	resultsB := []collector.PingResult{
		{Target: "8.8.8.8", PacketLoss: 50.0},
	}
	engine.EvaluatePing(resultsB)

	pending, _ = q.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 alerts (one per target), got %d", len(pending))
	}

	// Verify they have different cooldown keys.
	keys := map[string]bool{}
	for _, a := range pending {
		keys[a.CooldownKey] = true
	}
	if !keys["packet_loss:1.1.1.1"] {
		t.Error("expected cooldown key packet_loss:1.1.1.1 not found")
	}
	if !keys["packet_loss:8.8.8.8"] {
		t.Error("expected cooldown key packet_loss:8.8.8.8 not found")
	}

	// Re-evaluate target A — should be suppressed (cooldown active).
	engine.EvaluatePing(resultsA)

	pending, _ = q.Pending()
	if len(pending) != 2 {
		t.Fatalf("expected still 2 alerts (target A in cooldown), got %d", len(pending))
	}
}

func TestProcessQueue_SkipsWhenConnectionDown(t *testing.T) {
	engine, q := newTestEngine(t, dummyApprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
	})

	// Enqueue an alert.
	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}

	// Mark connection as down.
	engine.connState.SetDown(true)

	// ProcessQueue should short-circuit — alert stays pending.
	engine.ProcessQueue()

	pending, _ = q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected alert to remain pending when connection is down, got %d", len(pending))
	}
	// Verify retry count was NOT incremented.
	if pending[0].RetryCount != 0 {
		t.Fatalf("expected retry count 0 (skipped), got %d", pending[0].RetryCount)
	}
}

func TestProcessQueue_ProceedsWhenConnectionUp(t *testing.T) {
	apprise := failingApprise(t)
	engine, q := newTestEngine(t, apprise, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
		AlertMaxRetries:          5,
	})

	// Enqueue an alert.
	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	// Connection is up (default). ProcessQueue will attempt send
	// (which fails since dummyApprise points nowhere real), but
	// the point is it DOES attempt — retry count increments.
	engine.ProcessQueue()

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1 (attempted), got %d", pending[0].RetryCount)
	}
}

func TestProcessQueue_NilConnStateAlwaysProceeds(t *testing.T) {
	// Build engine without ConnectionState (simulates old behavior / tests).
	dir := t.TempDir()
	db, err := OpenDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	q, err := NewQueue(db)
	if err != nil {
		t.Fatalf("NewQueue: %v", err)
	}

	engine := NewEngine(q, failingApprise(t), &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
		AlertMaxRetries:          5,
	})
	// Explicitly set connState to nil to verify nil-safety.
	engine.connState = nil

	engine.EvaluatePing([]collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0},
	})

	// Should proceed (attempt send) even with nil connState.
	engine.ProcessQueue()

	pending, _ := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending alert, got %d", len(pending))
	}
	if pending[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1 (attempted), got %d", pending[0].RetryCount)
	}
}

func TestEngineNoAppriseSkipsEnqueue(t *testing.T) {
	engine, q := newTestEngine(t, nil, &config.Config{
		AlertPacketLossThreshold: 10,
		AlertCooldown:            1 * time.Second,
	})

	results := []collector.PingResult{
		{Target: "1.1.1.1", PacketLoss: 50.0, AvgMs: 20},
	}
	engine.EvaluatePing(results)

	pending, _ := q.Pending()
	if len(pending) != 0 {
		t.Fatalf("expected 0 alerts when apprise is nil, got %d", len(pending))
	}
}
