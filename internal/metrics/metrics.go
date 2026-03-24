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
	SpeedtestJitter      prometheus.Gauge
	DNSResolution        *prometheus.GaugeVec
	ConnectionUp         prometheus.Gauge
	DowntimeTotal        prometheus.Counter
	TracerouteHops       *prometheus.GaugeVec
	TracerouteHopLatency *prometheus.GaugeVec
	DNSFailures          *prometheus.CounterVec
	SpeedtestFailures    prometheus.Counter
	TracerouteFailures   prometheus.Counter
	ConnectionFlaps      prometheus.Counter
	SpeedtestInfo        *prometheus.GaugeVec

	// NDT7 metrics
	NDT7DownloadSpeed prometheus.Gauge
	NDT7UploadSpeed   prometheus.Gauge
	NDT7MinRTT        prometheus.Gauge
	NDT7RetransRate   prometheus.Gauge
	NDT7Failures      prometheus.Counter
	NDT7Info          *prometheus.GaugeVec

	// Bufferbloat metrics
	BufferbloatLatencyIncrease prometheus.Gauge
	BufferbloatGrade           prometheus.Gauge
	BufferbloatDownloadSpeed   prometheus.Gauge
	BufferbloatIdleLatency     prometheus.Gauge
	BufferbloatLoadedLatency   prometheus.Gauge
	BufferbloatFailures        prometheus.Counter

	// Multi-stream throughput metrics
	MaxDownloadSpeed   prometheus.Gauge
	ThroughputStreams  prometheus.Gauge
	ThroughputFailures prometheus.Counter

	// Orchestrator metrics
	BandwidthTestTriggers *prometheus.CounterVec
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
		SpeedtestJitter: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_speedtest_jitter_ms",
			Help: "Jitter reported by speed test in milliseconds",
		}),
		DNSResolution: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_dns_resolution_ms",
			Help: "DNS resolution time in milliseconds",
		}, []string{"target", "server"}),
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
		DNSFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pingpong_dns_failures_total",
			Help: "Total DNS lookup failures",
		}, []string{"target", "server"}),
		SpeedtestFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_speedtest_failures_total",
			Help: "Total speedtest execution failures",
		}),
		TracerouteFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_traceroute_failures_total",
			Help: "Total traceroute execution failures",
		}),
		ConnectionFlaps: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_connection_flaps_total",
			Help: "Total connection state transitions (up/down flaps)",
		}),
		SpeedtestInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_speedtest_info",
			Help: "Speedtest server metadata",
		}, []string{"server_name", "server_location", "isp"}),

		NDT7DownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_ndt7_download_speed_mbps",
			Help: "NDT7 download speed in Mbps (single stream)",
		}),
		NDT7UploadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_ndt7_upload_speed_mbps",
			Help: "NDT7 upload speed in Mbps (single stream)",
		}),
		NDT7MinRTT: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_ndt7_min_rtt_ms",
			Help: "Minimum RTT observed during NDT7 test in milliseconds",
		}),
		NDT7RetransRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_ndt7_retransmission_rate",
			Help: "TCP retransmission rate during NDT7 test (0.0-1.0)",
		}),
		NDT7Failures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_ndt7_failures_total",
			Help: "Total NDT7 test failures",
		}),
		NDT7Info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pingpong_ndt7_info",
			Help: "NDT7 server metadata",
		}, []string{"server_name"}),

		BufferbloatLatencyIncrease: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_bufferbloat_latency_increase_ms",
			Help: "Latency increase under load in milliseconds",
		}),
		BufferbloatGrade: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_bufferbloat_grade",
			Help: "Bufferbloat grade as numeric value (A+=6, A=5, B=4, C=3, D=2, F=1)",
		}),
		BufferbloatDownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_bufferbloat_download_speed_mbps",
			Help: "Download speed during bufferbloat test in Mbps (byproduct)",
		}),
		BufferbloatIdleLatency: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_bufferbloat_idle_latency_ms",
			Help: "Idle latency before bufferbloat test in milliseconds",
		}),
		BufferbloatLoadedLatency: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_bufferbloat_loaded_latency_ms",
			Help: "Loaded latency during bufferbloat test in milliseconds",
		}),
		BufferbloatFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_bufferbloat_failures_total",
			Help: "Total bufferbloat test failures",
		}),

		MaxDownloadSpeed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_max_download_speed_mbps",
			Help: "Maximum download speed from multi-stream test in Mbps",
		}),
		ThroughputStreams: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pingpong_throughput_streams",
			Help: "Number of parallel streams used in throughput test",
		}),
		ThroughputFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pingpong_throughput_failures_total",
			Help: "Total multi-stream throughput test failures",
		}),

		BandwidthTestTriggers: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pingpong_bandwidth_test_triggers_total",
			Help: "Total bandwidth test triggers by reason",
		}, []string{"reason"}),
	}

	reg.MustRegister(
		m.PingLatency, m.PingMin, m.PingMax,
		m.Jitter, m.PacketLoss,
		m.DownloadSpeed, m.UploadSpeed, m.SpeedtestLatency, m.SpeedtestJitter,
		m.DNSResolution, m.DNSFailures,
		m.ConnectionUp, m.DowntimeTotal, m.ConnectionFlaps,
		m.TracerouteHops, m.TracerouteHopLatency, m.TracerouteFailures,
		m.SpeedtestFailures, m.SpeedtestInfo,
		m.NDT7DownloadSpeed, m.NDT7UploadSpeed, m.NDT7MinRTT, m.NDT7RetransRate,
		m.NDT7Failures, m.NDT7Info,
		m.BufferbloatLatencyIncrease, m.BufferbloatGrade, m.BufferbloatDownloadSpeed,
		m.BufferbloatIdleLatency, m.BufferbloatLoadedLatency, m.BufferbloatFailures,
		m.MaxDownloadSpeed, m.ThroughputStreams, m.ThroughputFailures,
		m.BandwidthTestTriggers,
	)

	return m
}
