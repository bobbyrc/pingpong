package collector

import (
	"testing"
	"time"
)

func newTestOrchestrator(cfg OrchestratorConfig) (*BandwidthOrchestrator, chan TriggerEvent) {
	ndt7 := NewNDT7Collector()
	orch := NewBandwidthOrchestrator(ndt7, nil, cfg)
	triggerCh := make(chan TriggerEvent, 10)
	return orch, triggerCh
}

func TestOrchestrator_LatencySpike(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 3
	cfg.TriggerCooldown = 0 // disable cooldown for testing
	orch, triggerCh := newTestOrchestrator(cfg)

	// Warm up with normal latency
	for i := 0; i < 3; i++ {
		orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 2}}, triggerCh)
	}

	// spike: 20ms baseline * 2.0 multiplier = 40ms threshold
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 50, JitterMs: 2}}, triggerCh)

	select {
	case evt := <-triggerCh:
		if evt.Reason != TriggerLatencySpike {
			t.Fatalf("expected latency_spike trigger, got %s", evt.Reason)
		}
	default:
		t.Fatal("expected a latency spike trigger, got none")
	}
}

func TestOrchestrator_JitterSpike(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 3
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	// Warm up
	for i := 0; i < 3; i++ {
		orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 5}}, triggerCh)
	}

	// spike: 5ms baseline * 3.0 multiplier = 15ms threshold
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 20}}, triggerCh)

	select {
	case evt := <-triggerCh:
		if evt.Reason != TriggerJitterSpike {
			t.Fatalf("expected jitter_spike trigger, got %s", evt.Reason)
		}
	default:
		t.Fatal("expected a jitter spike trigger, got none")
	}
}

func TestOrchestrator_PacketLoss(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, PacketLoss: 5.0}}, triggerCh)

	select {
	case evt := <-triggerCh:
		if evt.Reason != TriggerPacketLoss {
			t.Fatalf("expected packet_loss trigger, got %s", evt.Reason)
		}
	default:
		t.Fatal("expected a packet loss trigger, got none")
	}
}

func TestOrchestrator_DNSSlow(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 3
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	// Warm up DNS
	for i := 0; i < 3; i++ {
		orch.ReportDNS([]DNSResult{{Target: "google.com", Server: "1.1.1.1", ResolutionMs: 10}}, triggerCh)
	}

	// spike: 10ms baseline * 2.0 = 20ms threshold
	orch.ReportDNS([]DNSResult{{Target: "google.com", Server: "1.1.1.1", ResolutionMs: 25}}, triggerCh)

	select {
	case evt := <-triggerCh:
		if evt.Reason != TriggerDNSSlow {
			t.Fatalf("expected dns_slow trigger, got %s", evt.Reason)
		}
	default:
		t.Fatal("expected a DNS slow trigger, got none")
	}
}

func TestOrchestrator_ConnectionRecovery(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	orch.ReportConnectionRecovery(triggerCh)

	select {
	case evt := <-triggerCh:
		if evt.Reason != TriggerConnectionRecovery {
			t.Fatalf("expected connection_recovery trigger, got %s", evt.Reason)
		}
	default:
		t.Fatal("expected a connection recovery trigger, got none")
	}
}

func TestOrchestrator_TriggerCooldown(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 1
	cfg.TriggerCooldown = 30 * time.Minute
	orch, triggerCh := newTestOrchestrator(cfg)

	fakeTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	orch.now = func() time.Time { return fakeTime }

	// Warm up with normal latency
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 2}}, triggerCh)

	// First spike — should trigger (baseline ~20, threshold 40, spike 100)
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 100, JitterMs: 2}}, triggerCh)
	select {
	case <-triggerCh:
		// expected
	default:
		t.Fatal("expected first trigger to fire")
	}

	// Second spike 5 minutes later — should be suppressed by cooldown
	fakeTime = fakeTime.Add(5 * time.Minute)
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 100, JitterMs: 2}}, triggerCh)
	select {
	case evt := <-triggerCh:
		t.Fatalf("expected trigger to be suppressed by cooldown, got %s", evt.Reason)
	default:
		// expected — cooldown active
	}

	// Third spike 31 minutes later — should fire (baseline moved up from spikes,
	// so use an even higher value to guarantee it exceeds 2x baseline)
	fakeTime = fakeTime.Add(31 * time.Minute)
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 500, JitterMs: 2}}, triggerCh)
	select {
	case <-triggerCh:
		// expected
	default:
		t.Fatal("expected trigger to fire after cooldown expired")
	}
}

func TestOrchestrator_WarmupPreventsSpuriousTriggers(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 5
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	// Send only 3 samples (< 5 warmup) with a "spike"
	for i := 0; i < 3; i++ {
		orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 2}}, triggerCh)
	}

	// This would be a spike, but baselines aren't ready yet
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 100, JitterMs: 2}}, triggerCh)

	select {
	case evt := <-triggerCh:
		// Packet loss triggers don't require warmup, filter those out
		if evt.Reason == TriggerLatencySpike || evt.Reason == TriggerJitterSpike {
			t.Fatalf("unexpected trigger during warmup: %s", evt.Reason)
		}
	default:
		// expected — baselines not ready
	}
}

func TestOrchestrator_NoTriggerBelowThreshold(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WarmupSamples = 3
	cfg.TriggerCooldown = 0
	orch, triggerCh := newTestOrchestrator(cfg)

	// Warm up
	for i := 0; i < 3; i++ {
		orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 20, JitterMs: 5, PacketLoss: 0}}, triggerCh)
	}

	// Normal values — shouldn't trigger
	orch.ReportPing([]PingResult{{Target: "1.1.1.1", AvgMs: 25, JitterMs: 8, PacketLoss: 0}}, triggerCh)

	select {
	case evt := <-triggerCh:
		t.Fatalf("unexpected trigger for normal values: %s", evt.Reason)
	default:
		// expected
	}
}
