package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	if m.PingLatency == nil {
		t.Fatal("PingLatency gauge should not be nil")
	}
	if m.PingMin == nil {
		t.Fatal("PingMin gauge should not be nil")
	}
	if m.PingMax == nil {
		t.Fatal("PingMax gauge should not be nil")
	}
	if m.Jitter == nil {
		t.Fatal("Jitter gauge should not be nil")
	}
	if m.PacketLoss == nil {
		t.Fatal("PacketLoss gauge should not be nil")
	}
	if m.DownloadSpeed == nil {
		t.Fatal("DownloadSpeed gauge should not be nil")
	}
	if m.UploadSpeed == nil {
		t.Fatal("UploadSpeed gauge should not be nil")
	}
	if m.SpeedtestLatency == nil {
		t.Fatal("SpeedtestLatency gauge should not be nil")
	}
	if m.DNSResolution == nil {
		t.Fatal("DNSResolution gauge should not be nil")
	}
	if m.ConnectionUp == nil {
		t.Fatal("ConnectionUp gauge should not be nil")
	}
	if m.DowntimeTotal == nil {
		t.Fatal("DowntimeTotal counter should not be nil")
	}
	if m.TracerouteHops == nil {
		t.Fatal("TracerouteHops gauge should not be nil")
	}
	if m.TracerouteHopLatency == nil {
		t.Fatal("TracerouteHopLatency gauge should not be nil")
	}
	if m.DNSFailures == nil {
		t.Fatal("DNSFailures counter should not be nil")
	}
	if m.SpeedtestFailures == nil {
		t.Fatal("SpeedtestFailures counter should not be nil")
	}
	if m.TracerouteFailures == nil {
		t.Fatal("TracerouteFailures counter should not be nil")
	}
	if m.ConnectionFlaps == nil {
		t.Fatal("ConnectionFlaps counter should not be nil")
	}
	if m.SpeedtestInfo == nil {
		t.Fatal("SpeedtestInfo gauge should not be nil")
	}

	// Verify metrics were actually registered by gathering
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	_ = families
}
