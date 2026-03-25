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
	if m.TracerouteFailures == nil {
		t.Fatal("TracerouteFailures counter should not be nil")
	}
	if m.ConnectionFlaps == nil {
		t.Fatal("ConnectionFlaps counter should not be nil")
	}
	// Verify metrics were actually registered by gathering
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	_ = families
}

func TestMetricNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := New(reg)

	// Set a value on every metric so Gather() returns them all.
	// Gauge vecs need WithLabelValues before they emit a family.
	m.PingLatency.WithLabelValues("8.8.8.8").Set(1)
	m.PingMin.WithLabelValues("8.8.8.8").Set(1)
	m.PingMax.WithLabelValues("8.8.8.8").Set(1)
	m.Jitter.WithLabelValues("8.8.8.8").Set(1)
	m.PacketLoss.WithLabelValues("8.8.8.8").Set(1)
	m.DownloadSpeed.Set(1)
	m.UploadSpeed.Set(1)
	m.ConnectionUp.Set(1)
	m.DowntimeTotal.Inc()
	m.ConnectionFlaps.Inc()
	m.DNSResolution.WithLabelValues("example.com", "8.8.8.8").Set(1)
	m.DNSFailures.WithLabelValues("example.com", "8.8.8.8").Inc()
	m.TracerouteHops.WithLabelValues("8.8.8.8").Set(1)
	m.TracerouteHopLatency.WithLabelValues("8.8.8.8", "1", "10.0.0.1").Set(1)
	m.TracerouteFailures.Inc()
	m.NDT7DownloadSpeed.Set(1)
	m.NDT7UploadSpeed.Set(1)
	m.NDT7MinRTT.Set(1)
	m.NDT7RetransRate.Set(0.01)
	m.NDT7Failures.Inc()
	m.NDT7Info.WithLabelValues("ndt-mlab1-lga0t.mlab.autojoin.measurement-lab.org").Set(1)
	m.BufferbloatLatencyIncrease.Set(10)
	m.BufferbloatGrade.Set(5)
	m.BufferbloatDownloadSpeed.Set(100)
	m.BufferbloatIdleLatency.Set(5)
	m.BufferbloatLoadedLatency.Set(15)
	m.BufferbloatFailures.Inc()
	m.MaxDownloadSpeed.Set(500)
	m.ThroughputStreams.Set(4)
	m.ThroughputFailures.Inc()
	m.BandwidthTestTriggers.WithLabelValues("baseline").Inc()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	got := make(map[string]bool, len(families))
	for _, f := range families {
		got[f.GetName()] = true
	}

	expected := []string{
		"pingpong_ping_latency_ms",
		"pingpong_ping_min_ms",
		"pingpong_ping_max_ms",
		"pingpong_jitter_ms",
		"pingpong_packet_loss_percent",
		"pingpong_download_speed_mbps",
		"pingpong_upload_speed_mbps",
		"pingpong_connection_up",
		"pingpong_downtime_seconds_total",
		"pingpong_connection_flaps_total",
		"pingpong_dns_resolution_ms",
		"pingpong_dns_failures_total",
		"pingpong_traceroute_hops",
		"pingpong_traceroute_hop_latency_ms",
		"pingpong_traceroute_failures_total",
		"pingpong_ndt7_download_speed_mbps",
		"pingpong_ndt7_upload_speed_mbps",
		"pingpong_ndt7_min_rtt_ms",
		"pingpong_ndt7_retransmission_rate",
		"pingpong_ndt7_failures_total",
		"pingpong_ndt7_info",
		"pingpong_bufferbloat_latency_increase_ms",
		"pingpong_bufferbloat_grade",
		"pingpong_bufferbloat_download_speed_mbps",
		"pingpong_bufferbloat_idle_latency_ms",
		"pingpong_bufferbloat_loaded_latency_ms",
		"pingpong_bufferbloat_failures_total",
		"pingpong_max_download_speed_mbps",
		"pingpong_throughput_streams",
		"pingpong_throughput_failures_total",
		"pingpong_bandwidth_test_triggers_total",
	}

	for _, name := range expected {
		if !got[name] {
			t.Errorf("expected metric %q not found in gathered families", name)
		}
	}
}
