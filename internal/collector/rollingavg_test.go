package collector

import (
	"math"
	"testing"
)

func TestRollingAvg_Basic(t *testing.T) {
	r := NewRollingAvg(0.1, 5)

	if r.Ready() {
		t.Fatal("should not be ready with 0 samples")
	}

	// Add 5 identical samples
	for i := 0; i < 5; i++ {
		r.Update(100.0)
	}

	if !r.Ready() {
		t.Fatal("should be ready after 5 samples")
	}

	if math.Abs(r.Value()-100.0) > 0.001 {
		t.Fatalf("expected value ~100.0, got %.3f", r.Value())
	}
}

func TestRollingAvg_EMAConvergence(t *testing.T) {
	r := NewRollingAvg(0.1, 1)

	// Start at 0
	r.Update(0)

	// Push toward 100
	for i := 0; i < 100; i++ {
		r.Update(100.0)
	}

	// After many samples, should converge close to 100
	if math.Abs(r.Value()-100.0) > 1.0 {
		t.Fatalf("expected value close to 100.0 after convergence, got %.3f", r.Value())
	}
}

func TestRollingAvg_SingleSample(t *testing.T) {
	r := NewRollingAvg(0.1, 1)
	r.Update(42.0)

	if !r.Ready() {
		t.Fatal("should be ready after 1 sample with minReady=1")
	}
	if r.Value() != 42.0 {
		t.Fatalf("expected 42.0, got %.3f", r.Value())
	}
}

func TestRollingAvg_ReadyThreshold(t *testing.T) {
	r := NewRollingAvg(0.1, 3)

	r.Update(10)
	if r.Ready() {
		t.Fatal("should not be ready after 1 sample")
	}

	r.Update(20)
	if r.Ready() {
		t.Fatal("should not be ready after 2 samples")
	}

	r.Update(30)
	if !r.Ready() {
		t.Fatal("should be ready after 3 samples")
	}

	if r.Count() != 3 {
		t.Fatalf("expected count 3, got %d", r.Count())
	}
}

func TestRollingAvg_InvalidParams(t *testing.T) {
	// alpha <= 0 defaults to 0.1
	r := NewRollingAvg(0, 0)
	r.Update(50)
	if r.Value() != 50 {
		t.Fatalf("expected 50, got %.3f", r.Value())
	}
	// minReady < 1 defaults to 1
	if !r.Ready() {
		t.Fatal("should be ready with minReady defaulting to 1")
	}
}
