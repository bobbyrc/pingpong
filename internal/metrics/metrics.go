package metrics

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	PingLatency          *prometheus.GaugeVec
	PingMin              *prometheus.GaugeVec
	PingMax              *prometheus.GaugeVec
	Jitter               *prometheus.GaugeVec
	PacketLoss           *prometheus.GaugeVec
	DownloadSpeed        prometheus.Gauge
	UploadSpeed          prometheus.Gauge
	SpeedtestLatency     prometheus.Gauge
	DNSResolution        *prometheus.GaugeVec
	ConnectionUp         prometheus.Gauge
	DowntimeTotal        prometheus.Counter
	TracerouteHops       *prometheus.GaugeVec
	TracerouteHopLatency *prometheus.GaugeVec
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		PingLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_latency_ms",
			Help: "Average ping latency in milliseconds",
		}, []string{"target"}),
		PingMin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_min_ms",
			Help: "Minimum ping latency in milliseconds",
		}, []string{"target"}),
		PingMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ping_max_ms",
			Help: "Maximum ping latency in milliseconds",
		}, []string{"target"}),
		Jitter: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_jitter_ms",
			Help: "Ping jitter (standard deviation) in milliseconds",
		}, []string{"target"}),
		PacketLoss: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_packet_loss_percent",
			Help: "Packet loss percentage",
		}, []string{"target"}),
		DownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_download_speed_mbps",
			Help: "Download speed in Mbps",
		}),
		UploadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_upload_speed_mbps",
			Help: "Upload speed in Mbps",
		}),
		SpeedtestLatency: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_speedtest_latency_ms",
			Help: "Latency reported by speed test in milliseconds",
		}),
		DNSResolution: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_dns_resolution_ms",
			Help: "DNS resolution time in milliseconds",
		}, []string{"target"}),
		ConnectionUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_connection_up",
			Help: "Whether the internet connection is up (1) or down (0)",
		}),
		DowntimeTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_downtime_seconds_total",
			Help: "Total downtime in seconds",
		}),
		TracerouteHops: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_traceroute_hops",
			Help: "Number of hops in traceroute",
		}, []string{"target"}),
		TracerouteHopLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_traceroute_hop_latency_ms",
			Help: "Latency per traceroute hop in milliseconds",
		}, []string{"target", "hop", "address"}),
	}

	reg.MustRegister(
		m.PingLatency, m.PingMin, m.PingMax,
		m.Jitter, m.PacketLoss,
		m.DownloadSpeed, m.UploadSpeed, m.SpeedtestLatency,
		m.DNSResolution,
		m.ConnectionUp, m.DowntimeTotal,
		m.TracerouteHops, m.TracerouteHopLatency,
	)

	return m
}
