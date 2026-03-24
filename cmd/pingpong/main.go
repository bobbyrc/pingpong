package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/bobbyrc/pingpong/internal/alerter"
	"github.com/bobbyrc/pingpong/internal/collector"
	"github.com/bobbyrc/pingpong/internal/config"
	"github.com/bobbyrc/pingpong/internal/metrics"
	"github.com/bobbyrc/pingpong/internal/web"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	bandwidthModeScheduled = "scheduled"
	bandwidthModeEvent     = "event"
)

func recordNDT7Result(m *metrics.Metrics, engine *alerter.Engine, result collector.NDT7Result) {
	m.NDT7DownloadSpeed.Set(result.DownloadMbps)
	m.NDT7UploadSpeed.Set(result.UploadMbps)
	m.NDT7MinRTT.Set(result.MinRTTMs)
	m.NDT7RetransRate.Set(result.RetransRate)

	// Backward-compat aliases
	m.DownloadSpeed.Set(result.DownloadMbps)
	m.UploadSpeed.Set(result.UploadMbps)

	m.NDT7Info.Reset()
	if result.ServerName != "" {
		m.NDT7Info.WithLabelValues(result.ServerName).Set(1)
	}

	engine.EvaluateNDT7(result)
}

func recordBufferbloatResult(m *metrics.Metrics, engine *alerter.Engine, result collector.BufferbloatResult) {
	m.BufferbloatLatencyIncrease.Set(result.LatencyIncreaseMs)
	m.BufferbloatGrade.Set(collector.GradeToNumeric(result.Grade))
	m.BufferbloatDownloadSpeed.Set(result.DownloadMbps)
	m.BufferbloatIdleLatency.Set(result.IdleLatencyMs)
	m.BufferbloatLoadedLatency.Set(result.LoadedLatencyMs)

	engine.EvaluateBufferbloat(result)
}

func resolveBufferbloatTarget(cfg *config.Config) string {
	if cfg.BufferbloatTarget != "" {
		return cfg.BufferbloatTarget
	}
	if len(cfg.PingTargets) > 0 {
		return cfg.PingTargets[0]
	}
	return ""
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	slog.Info("pingpong starting",
		"listen_addr", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
		"ping_targets", cfg.PingTargets,
		"ping_count", cfg.PingCount,
		"ping_interval", cfg.PingInterval,
		"dns_targets", cfg.DNSTargets,
		"dns_servers", cfg.DNSServers,
		"dns_interval", cfg.DNSInterval,
		"ndt7_interval", cfg.SpeedtestInterval,
		"bandwidth_mode", cfg.BandwidthMode,
		"traceroute_target", cfg.TracerouteTarget,
		"traceroute_interval", cfg.TracerouteInterval,
	)
	slog.Info("alert thresholds",
		"packet_loss_pct", cfg.AlertPacketLossThreshold,
		"latency_ms", cfg.AlertPingThreshold,
		"jitter_ms", cfg.AlertJitterThreshold,
		"download_mbps", cfg.AlertSpeedThreshold,
		"downtime", cfg.AlertDowntimeThreshold,
		"cooldown", cfg.AlertCooldown,
	)

	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	pingCollector := collector.NewPingCollector(cfg.PingTargets, cfg.PingCount)
	ndt7Collector := collector.NewNDT7Collector()
	dnsCollector := collector.NewDNSCollector(cfg.DNSTargets, cfg.DNSServers)
	traceCollector := collector.NewTracerouteCollector(cfg.TracerouteTarget)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "path", cfg.DataDir, "error", err)
		os.Exit(1)
	}
	db, err := alerter.OpenDB(filepath.Join(cfg.DataDir, "alerts.db"))
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	queue, err := alerter.NewQueue(db)
	if err != nil {
		slog.Error("failed to create alert queue", "error", err)
		os.Exit(1)
	}

	history, err := web.NewHistoryStore(db)
	if err != nil {
		slog.Warn("failed to create history store — continuing without history", "error", err)
		history = nil
	}

	var appriseClient *alerter.AppriseClient
	if cfg.AppriseURLs != "" {
		appriseClient = alerter.NewAppriseClient(cfg.AppriseURL, cfg.AppriseURLs)
		slog.Info("apprise notifications enabled")
	} else {
		slog.Warn("apprise not configured — notifications disabled")
	}

	engine := alerter.NewEngine(queue, appriseClient, cfg)
	engine.SeedCooldowns()

	mux := http.NewServeMux()

	// Web UI
	webHandler, err := web.NewHandler(reg, queue, history, cfg.EnvFile)
	if err != nil {
		slog.Error("failed to create web handler", "error", err)
		os.Exit(1)
	}
	webHandler.RegisterRoutes(mux)

	// Resolve hostnames for ping targets
	hostnames := collector.ResolveHostnames(cfg.PingTargets)
	for target, hostname := range hostnames {
		slog.Info("resolved hostname", "target", target, "hostname", hostname)
	}
	webHandler.SetHostnames(hostnames)

	// Core routes
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	webHandler.Start(ctx)

	var wg sync.WaitGroup

	var downSince time.Time
	flushCh := make(chan struct{}, 1)
	connState := engine.ConnState()

	// Channel for orchestrator trigger events (event mode only)
	var orchTriggerCh chan collector.TriggerEvent
	var orchestrator *collector.BandwidthOrchestrator

	bandwidthMode := cfg.BandwidthMode

	// Resolve bufferbloat target once for both modes
	bbTarget := resolveBufferbloatTarget(cfg)

	// Set up orchestrator for event mode
	if bandwidthMode == bandwidthModeEvent {
		orchCfg := collector.DefaultOrchestratorConfig()
		orchCfg.BaselineInterval = cfg.BandwidthBaselineInterval
		orchCfg.MinNDT7Interval = cfg.BandwidthMinNDT7Interval
		orchCfg.MinBloatInterval = cfg.BandwidthMinBloatInterval
		orchCfg.TriggerCooldown = cfg.BandwidthTriggerCooldown

		var bbCollector *collector.BufferbloatCollector
		if cfg.BufferbloatDownloadURL != "" && bbTarget != "" {
			bbCollector = collector.NewBufferbloatCollector(bbTarget, cfg.BufferbloatDownloadURL)
		}

		orchestrator = collector.NewBandwidthOrchestrator(ndt7Collector, bbCollector, orchCfg)
		orchTriggerCh = make(chan collector.TriggerEvent, 4)

		// Result consumer goroutine
		resultCh := make(chan collector.BandwidthResult, 4)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case result := <-resultCh:
					m.BandwidthTestTriggers.WithLabelValues(string(result.Trigger.Reason)).Inc()
					if result.NDT7 != nil {
						recordNDT7Result(m, engine, *result.NDT7)
					}
					if result.Bufferbloat != nil {
						recordBufferbloatResult(m, engine, *result.Bufferbloat)
					}
				}
			}
		}()

		// Orchestrator main loop
		wg.Add(1)
		go func() {
			defer wg.Done()
			orchestrator.Run(ctx, resultCh)
		}()

		// Trigger handler goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case trigger := <-orchTriggerCh:
					orchestrator.HandleTrigger(ctx, trigger, resultCh)
				}
			}
		}()
	} else if bandwidthMode != bandwidthModeScheduled {
		slog.Info("bandwidth testing disabled", "mode", bandwidthMode)
	}

	// Ping loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.PingInterval)
		defer ticker.Stop()

		runPing := func() {
			results := pingCollector.Collect(ctx)
			for _, r := range results {
				m.PingLatency.WithLabelValues(r.Target).Set(r.AvgMs)
				m.PingMin.WithLabelValues(r.Target).Set(r.MinMs)
				m.PingMax.WithLabelValues(r.Target).Set(r.MaxMs)
				m.Jitter.WithLabelValues(r.Target).Set(r.JitterMs)
				m.PacketLoss.WithLabelValues(r.Target).Set(r.PacketLoss)
			}

			allDown := len(results) > 0
			for _, r := range results {
				if r.PacketLoss < 100 {
					allDown = false
					break
				}
			}
			if allDown && !connState.IsDown() {
				connState.SetDown(true)
				downSince = time.Now()
				m.ConnectionUp.Set(0)
				m.ConnectionFlaps.Inc()
				slog.Warn("connection detected as DOWN")
			} else if !allDown && connState.IsDown() {
				downtime := time.Since(downSince)
				m.DowntimeTotal.Add(downtime.Seconds())
				connState.SetDown(false)
				m.ConnectionUp.Set(1)
				m.ConnectionFlaps.Inc()
				slog.Info("connection restored", "downtime", downtime.Round(time.Second))
				select {
				case flushCh <- struct{}{}:
				default:
				}
				if orchestrator != nil {
					orchestrator.ReportConnectionRecovery(orchTriggerCh)
				}
			} else if !allDown {
				m.ConnectionUp.Set(1)
			}

			isDown := connState.IsDown()
			ds := downSince

			engine.EvaluatePing(results)
			if isDown {
				engine.EvaluateDowntime(true, ds)
			}

			if orchestrator != nil {
				orchestrator.ReportPing(results, orchTriggerCh)
			}
		}

		runPing()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runPing()
			}
		}
	}()

	// NDT7 speed test loop (scheduled mode only)
	if bandwidthMode == bandwidthModeScheduled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(cfg.SpeedtestInterval)
			defer ticker.Stop()

			runNDT7 := func() {
				result, err := ndt7Collector.Collect(ctx)
				if err != nil {
					slog.Error("ndt7 test failed", "error", err)
					m.NDT7Failures.Inc()
					return
				}
				recordNDT7Result(m, engine, result)
			}

			runNDT7()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runNDT7()
				}
			}
		}()
	}

	// DNS loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.DNSInterval)
		defer ticker.Stop()

		runDNS := func() {
			results, failures := dnsCollector.Collect(ctx)
			for _, r := range results {
				m.DNSResolution.WithLabelValues(r.Target, r.Server).Set(r.ResolutionMs)
			}
			for _, f := range failures {
				m.DNSFailures.WithLabelValues(f.Target, f.Server).Inc()
			}

			if orchestrator != nil {
				orchestrator.ReportDNS(results, orchTriggerCh)
			}
		}

		runDNS()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDNS()
			}
		}
	}()

	// Traceroute loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.TracerouteInterval)
		defer ticker.Stop()

		runTrace := func() {
			result, err := traceCollector.Collect(ctx)
			if err != nil {
				slog.Error("traceroute failed", "error", err)
				m.TracerouteFailures.Inc()
				return
			}
			m.TracerouteHops.WithLabelValues(result.Target).Set(float64(result.HopCount))
			m.TracerouteHopLatency.Reset()
			for _, hop := range result.Hops {
				if hop.Address != "*" {
					m.TracerouteHopLatency.WithLabelValues(
						result.Target,
						fmt.Sprintf("%d", hop.Number),
						hop.Address,
					).Set(hop.LatencyMs)
				}
			}
		}

		runTrace()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runTrace()
			}
		}
	}()

	// Bufferbloat loop (scheduled mode only)
	if bandwidthMode == bandwidthModeScheduled && cfg.BufferbloatDownloadURL != "" && bbTarget != "" {
		bbCollector := collector.NewBufferbloatCollector(bbTarget, cfg.BufferbloatDownloadURL)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 60-second startup delay
			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
			}

			ticker := time.NewTicker(cfg.BufferbloatInterval)
			defer ticker.Stop()

			runBB := func() {
				result, err := bbCollector.Collect(ctx)
				if err != nil {
					slog.Error("bufferbloat test failed", "error", err)
					m.BufferbloatFailures.Inc()
					return
				}
				recordBufferbloatResult(m, engine, result)
			}

			runBB()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runBB()
				}
			}
		}()
		slog.Info("bufferbloat monitoring enabled", "target", bbTarget, "interval", cfg.BufferbloatInterval)
	}

	// Multi-stream throughput loop
	if cfg.ThroughputDownloadURL != "" {
		tpCollector := collector.NewThroughputCollector(cfg.ThroughputDownloadURL, cfg.ThroughputStreams, cfg.ThroughputDuration)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 2-minute startup delay
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Minute):
			}

			ticker := time.NewTicker(cfg.ThroughputInterval)
			defer ticker.Stop()

			runTP := func() {
				result, err := tpCollector.Collect(ctx)
				if err != nil {
					slog.Error("throughput test failed", "error", err)
					m.ThroughputFailures.Inc()
					return
				}
				m.MaxDownloadSpeed.Set(result.DownloadMbps)
				m.ThroughputStreams.Set(float64(result.Streams))

				// Also update backward-compat download speed if higher than NDT7
				slog.Info("throughput test complete",
					"download_mbps", result.DownloadMbps,
					"streams", result.Streams,
					"duration", result.DurationSecs,
					"bytes", result.BytesTotal,
				)
			}

			runTP()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runTP()
				}
			}
		}()
		slog.Info("throughput monitoring enabled", "interval", cfg.ThroughputInterval, "streams", cfg.ThroughputStreams)
	}

	// Alert retry loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(cfg.AlertRetryInterval)
		defer ticker.Stop()

		engine.ProcessQueue()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				engine.ProcessQueue()
			case <-flushCh:
				engine.ProcessQueue()
			}
		}
	}()

	// HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("HTTP server starting", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	<-sigCh
	slog.Info("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)

	wg.Wait()
	slog.Info("pingpong stopped")
}
