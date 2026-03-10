package collector

import (
	"testing"
	"time"
)

func TestCalculatePingResult(t *testing.T) {
	rtts := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		15 * time.Millisecond,
		25 * time.Millisecond,
		30 * time.Millisecond,
	}

	result := calculatePingResult("1.1.1.1", rtts, 5, 0)

	if result.Target != "1.1.1.1" {
		t.Fatalf("expected target 1.1.1.1, got %s", result.Target)
	}
	if result.AvgMs != 20.0 {
		t.Fatalf("expected avg 20.0, got %f", result.AvgMs)
	}
	if result.MinMs != 10.0 {
		t.Fatalf("expected min 10.0, got %f", result.MinMs)
	}
	if result.MaxMs != 30.0 {
		t.Fatalf("expected max 30.0, got %f", result.MaxMs)
	}
	if result.PacketLoss != 0.0 {
		t.Fatalf("expected 0%% packet loss, got %f", result.PacketLoss)
	}
	// Jitter should be stddev of [10,20,15,25,30] ≈ 7.07
	if result.JitterMs < 7.0 || result.JitterMs > 7.2 {
		t.Fatalf("expected jitter ~7.07, got %f", result.JitterMs)
	}
}

func TestCalculatePingResultWithLoss(t *testing.T) {
	rtts := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
	}

	result := calculatePingResult("8.8.8.8", rtts, 5, 3)

	if result.PacketLoss != 60.0 {
		t.Fatalf("expected 60%% packet loss, got %f", result.PacketLoss)
	}
}

func TestCalculatePingResultAllLost(t *testing.T) {
	result := calculatePingResult("8.8.8.8", nil, 5, 5)

	if result.PacketLoss != 100.0 {
		t.Fatalf("expected 100%% packet loss, got %f", result.PacketLoss)
	}
	if result.AvgMs != 0 {
		t.Fatalf("expected avg 0 with no responses, got %f", result.AvgMs)
	}
}

func TestResolveHostnames_HostnameTarget(t *testing.T) {
	result := ResolveHostnames([]string{"example.com"})
	if result["example.com"] != "example.com" {
		t.Errorf("expected hostname target to map to itself, got %q", result["example.com"])
	}
}

func TestResolveHostnames_NonIPTarget(t *testing.T) {
	result := ResolveHostnames([]string{"not-an-ip"})
	if result["not-an-ip"] != "not-an-ip" {
		t.Errorf("expected non-IP to map to itself, got %q", result["not-an-ip"])
	}
}
