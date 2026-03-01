package alerter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bcraig/pingpong/internal/collector"
	"github.com/bcraig/pingpong/internal/config"
)

func TestEngineEvaluatePacketLoss(t *testing.T) {
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
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
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
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
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
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
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
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
	dir := t.TempDir()
	q, _ := NewQueue(filepath.Join(dir, "test.db"))
	defer q.Close()

	engine := NewEngine(q, nil, &config.Config{
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
