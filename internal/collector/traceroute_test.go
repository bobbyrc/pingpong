package collector

import (
	"testing"
)

func TestParseTracerouteOutput(t *testing.T) {
	output := `traceroute to 1.1.1.1 (1.1.1.1), 30 hops max, 60 byte packets
 1  gateway (192.168.1.1)  1.234 ms  1.345 ms  1.456 ms
 2  10.0.0.1 (10.0.0.1)  5.678 ms  5.789 ms  5.890 ms
 3  * * *
 4  one.one.one.one (1.1.1.1)  12.345 ms  12.456 ms  12.567 ms`

	result := parseTracerouteOutput("1.1.1.1", output)

	if result.Target != "1.1.1.1" {
		t.Fatalf("expected target 1.1.1.1, got %s", result.Target)
	}
	if result.HopCount != 4 {
		t.Fatalf("expected 4 hops, got %d", result.HopCount)
	}
	if len(result.Hops) != 4 {
		t.Fatalf("expected 4 hop entries, got %d", len(result.Hops))
	}

	if result.Hops[0].Number != 1 {
		t.Fatalf("expected hop 1, got %d", result.Hops[0].Number)
	}
	if result.Hops[0].Address != "192.168.1.1" {
		t.Fatalf("expected address 192.168.1.1, got %s", result.Hops[0].Address)
	}
	if result.Hops[0].LatencyMs < 1.3 || result.Hops[0].LatencyMs > 1.4 {
		t.Fatalf("expected hop 1 latency ~1.345, got %f", result.Hops[0].LatencyMs)
	}

	if result.Hops[2].Address != "*" {
		t.Fatalf("expected timeout hop address *, got %s", result.Hops[2].Address)
	}
	if result.Hops[2].LatencyMs != 0 {
		t.Fatalf("expected timeout hop latency 0, got %f", result.Hops[2].LatencyMs)
	}
}

func TestParseTracerouteOutput_Empty(t *testing.T) {
	result := parseTracerouteOutput("", "")

	if result.HopCount != 0 {
		t.Fatalf("expected 0 hops, got %d", result.HopCount)
	}
	if len(result.Hops) != 0 {
		t.Fatalf("expected empty hops slice, got %d entries", len(result.Hops))
	}
}

func TestParseTracerouteOutput_HeaderOnly(t *testing.T) {
	output := "traceroute to example.com (93.184.216.34), 30 hops max, 60 byte packets\n"

	result := parseTracerouteOutput("example.com", output)

	if result.Target != "example.com" {
		t.Fatalf("expected target example.com, got %s", result.Target)
	}
	if result.HopCount != 0 {
		t.Fatalf("expected 0 hops, got %d", result.HopCount)
	}
	if len(result.Hops) != 0 {
		t.Fatalf("expected empty hops slice, got %d entries", len(result.Hops))
	}
}
